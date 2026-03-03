package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &watchHistoryDataSource{}

func NewWatchHistoryDataSource() datasource.DataSource {
	return &watchHistoryDataSource{}
}

type watchHistoryDataSource struct {
	client *client
}

type watchHistoryDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Limit   types.Int64  `tfsdk:"limit"`
	Entries types.List   `tfsdk:"entries"`
}

type watchHistoryEntryModel struct {
	ID           types.String  `tfsdk:"entry_id"`
	Name         types.String  `tfsdk:"name"`
	Type         types.String  `tfsdk:"type"`
	VideoID      types.String  `tfsdk:"video_id"`
	LastWatched  types.String  `tfsdk:"last_watched"`
	TimeOffset   types.Int64   `tfsdk:"time_offset"`
	Duration     types.Int64   `tfsdk:"duration"`
	TimeWatched  types.Int64   `tfsdk:"time_watched"`
	TimesWatched types.Int64   `tfsdk:"times_watched"`
	Progress     types.Float64 `tfsdk:"progress"`
	RawJSON      types.String  `tfsdk:"raw_json"`
}

var watchHistoryEntryObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"entry_id":      types.StringType,
		"name":          types.StringType,
		"type":          types.StringType,
		"video_id":      types.StringType,
		"last_watched":  types.StringType,
		"time_offset":   types.Int64Type,
		"duration":      types.Int64Type,
		"time_watched":  types.Int64Type,
		"times_watched": types.Int64Type,
		"progress":      types.Float64Type,
		"raw_json":      types.StringType,
	},
}

func (d *watchHistoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_watch_history"
}

func (d *watchHistoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Returns watch history entries for the authenticated account.",
		MarkdownDescription: "Reads watch history entries for the authenticated account from library item state in Stremio datastore.\n\n## Example Usage\n\n```hcl\ndata \"stremio_watch_history\" \"recent\" {\n  limit = 25\n}\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Static identifier for this data source.",
				MarkdownDescription: "Static identifier for this data source instance.",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				Description:         "Optional maximum number of entries to return.",
				MarkdownDescription: "Optional maximum number of watch history entries to return.",
			},
			"entries": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Watch history entries.",
				MarkdownDescription: "Watch history entries returned by Stremio.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entry_id": schema.StringAttribute{
							Computed:            true,
							Description:         "History entry identifier when available.",
							MarkdownDescription: "History entry identifier when available.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							Description:         "Display name/title when available.",
							MarkdownDescription: "Display name/title when available.",
						},
						"type": schema.StringAttribute{
							Computed:            true,
							Description:         "Content type when available (for example `movie` or `series`).",
							MarkdownDescription: "Content type when available (for example `movie` or `series`).",
						},
						"video_id": schema.StringAttribute{
							Computed:            true,
							Description:         "Video identifier from entry state when available.",
							MarkdownDescription: "Video identifier from entry state when available.",
						},
						"last_watched": schema.StringAttribute{
							Computed:            true,
							Description:         "Timestamp of last watch activity when available.",
							MarkdownDescription: "Timestamp of last watch activity when available.",
						},
						"time_offset": schema.Int64Attribute{
							Computed:            true,
							Description:         "Playback position offset in seconds when available.",
							MarkdownDescription: "Playback position offset in seconds when available.",
						},
						"duration": schema.Int64Attribute{
							Computed:            true,
							Description:         "Content duration in seconds when available.",
							MarkdownDescription: "Content duration in seconds when available.",
						},
						"time_watched": schema.Int64Attribute{
							Computed:            true,
							Description:         "Total watched time in seconds when available.",
							MarkdownDescription: "Total watched time in seconds when available.",
						},
						"times_watched": schema.Int64Attribute{
							Computed:            true,
							Description:         "Number of times watched when available.",
							MarkdownDescription: "Number of times watched when available.",
						},
						"progress": schema.Float64Attribute{
							Computed:            true,
							Description:         "Watch progress percent when available.",
							MarkdownDescription: "Watch progress percent when available.",
						},
						"raw_json": schema.StringAttribute{
							Computed:            true,
							Description:         "Raw JSON object for this history entry.",
							MarkdownDescription: "Raw JSON object for this history entry.",
						},
					},
				},
			},
		},
	}
}

func (d *watchHistoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *watchHistoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var config watchHistoryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := int64(0)
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		limit = config.Limit.ValueInt64()
	}

	entries, err := d.client.WatchHistory(ctx, limit)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read watch history", err.Error())
		return
	}

	mappedEntries := make([]watchHistoryEntryModel, 0, len(entries))
	for index, entry := range entries {
		mappedEntries = append(mappedEntries, mapWatchHistoryEntry(entry, index))
	}

	entriesValue, diags := types.ListValueFrom(ctx, watchHistoryEntryObjectType, mappedEntries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := watchHistoryDataSourceModel{
		ID:      types.StringValue("watch-history"),
		Limit:   config.Limit,
		Entries: entriesValue,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapWatchHistoryEntry(entry map[string]any, index int) watchHistoryEntryModel {
	state, _ := entry["state"].(map[string]any)
	entryID := firstString(entry, []string{"_id", "id"})
	if entryID == "" {
		entryID = fmt.Sprintf("entry-%d", index)
	}

	name := firstString(entry, []string{"name", "title"})
	entryType := firstString(entry, []string{"type"})
	videoID := firstString(state, []string{"video_id"})
	lastWatched := firstString(state, []string{"lastWatched"})
	timeOffset, hasTimeOffset := firstInt64(state, []string{"timeOffset"})
	duration, hasDuration := firstInt64(state, []string{"duration"})
	timeWatched, hasTimeWatched := firstInt64(state, []string{"timeWatched"})
	timesWatched, hasTimesWatched := firstInt64(state, []string{"timesWatched"})
	progress, hasProgress := firstFloat64(entry, []string{"progress"})
	if !hasProgress && hasTimeOffset && hasDuration && duration > 0 {
		progress = float64(timeOffset) / float64(duration) * 100
		hasProgress = true
	}

	rawBytes, _ := json.Marshal(entry)
	result := watchHistoryEntryModel{
		ID:          stringValueOrNull(entryID),
		Name:        stringValueOrNull(name),
		Type:        stringValueOrNull(entryType),
		VideoID:     stringValueOrNull(videoID),
		LastWatched: stringValueOrNull(lastWatched),
		RawJSON:     types.StringValue(string(rawBytes)),
	}

	if hasTimeOffset {
		result.TimeOffset = types.Int64Value(timeOffset)
	} else {
		result.TimeOffset = types.Int64Null()
	}
	if hasDuration {
		result.Duration = types.Int64Value(duration)
	} else {
		result.Duration = types.Int64Null()
	}
	if hasTimeWatched {
		result.TimeWatched = types.Int64Value(timeWatched)
	} else {
		result.TimeWatched = types.Int64Null()
	}
	if hasTimesWatched {
		result.TimesWatched = types.Int64Value(timesWatched)
	} else {
		result.TimesWatched = types.Int64Null()
	}
	if hasProgress {
		result.Progress = types.Float64Value(progress)
	} else {
		result.Progress = types.Float64Null()
	}

	return result
}

func stringValueOrNull(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func firstString(source map[string]any, keys []string) string {
	for _, key := range keys {
		if value, ok := source[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func firstInt64(source map[string]any, keys []string) (int64, bool) {
	for _, key := range keys {
		value, exists := source[key]
		if !exists {
			continue
		}
		switch converted := value.(type) {
		case int:
			return int64(converted), true
		case int32:
			return int64(converted), true
		case int64:
			return converted, true
		case float32:
			return int64(converted), true
		case float64:
			return int64(converted), true
		}
	}
	return 0, false
}

func firstFloat64(source map[string]any, keys []string) (float64, bool) {
	for _, key := range keys {
		value, exists := source[key]
		if !exists {
			continue
		}
		switch converted := value.(type) {
		case float32:
			return float64(converted), true
		case float64:
			return converted, true
		case int:
			return float64(converted), true
		case int32:
			return float64(converted), true
		case int64:
			return float64(converted), true
		}
	}
	return 0, false
}
