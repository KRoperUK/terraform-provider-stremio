package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &continueWatchingDataSource{}

func NewContinueWatchingDataSource() datasource.DataSource {
	return &continueWatchingDataSource{}
}

type continueWatchingDataSource struct {
	client *client
}

type continueWatchingDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Limit   types.Int64  `tfsdk:"limit"`
	Entries types.List   `tfsdk:"entries"`
}

func (d *continueWatchingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_continue_watching"
}

func (d *continueWatchingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Returns continue watching entries for the authenticated account.",
		MarkdownDescription: "Reads continue watching entries for the authenticated account from library item state in Stremio datastore.\n\n## Example Usage\n\n```hcl\ndata \"stremio_continue_watching\" \"current\" {\n  limit = 25\n}\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Static identifier for this data source.",
				MarkdownDescription: "Static identifier for this data source instance.",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				Description:         "Optional maximum number of entries to return.",
				MarkdownDescription: "Optional maximum number of continue watching entries to return.",
			},
			"entries": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Continue watching entries.",
				MarkdownDescription: "Continue watching entries returned by Stremio.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entry_id": schema.StringAttribute{
							Computed:            true,
							Description:         "Entry identifier when available.",
							MarkdownDescription: "Entry identifier when available.",
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
							Description:         "Raw JSON object for this entry.",
							MarkdownDescription: "Raw JSON object for this entry.",
						},
					},
				},
			},
		},
	}
}

func (d *continueWatchingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *continueWatchingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var config continueWatchingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	limit := int64(0)
	if !config.Limit.IsNull() && !config.Limit.IsUnknown() {
		limit = config.Limit.ValueInt64()
	}

	entries, err := d.client.ContinueWatching(ctx, limit)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read continue watching entries", err.Error())
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

	state := continueWatchingDataSourceModel{
		ID:      types.StringValue("continue-watching"),
		Limit:   config.Limit,
		Entries: entriesValue,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
