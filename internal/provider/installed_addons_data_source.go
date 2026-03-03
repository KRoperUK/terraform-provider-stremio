package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &installedAddonsDataSource{}

func NewInstalledAddonsDataSource() datasource.DataSource {
	return &installedAddonsDataSource{}
}

type installedAddonsDataSource struct {
	client *client
}

type installedAddonsDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Type        types.String `tfsdk:"type"`
	CatalogType types.String `tfsdk:"catalog_type"`
	Resource    types.String `tfsdk:"resource"`
	NameRegex   types.String `tfsdk:"name_regex"`
	SortBy      types.String `tfsdk:"sort_by"`
	SortOrder   types.String `tfsdk:"sort_order"`
	Addons      types.List   `tfsdk:"addons"`
}

func (d *installedAddonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_installed_addons"
}

func (d *installedAddonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Returns installed Stremio addons for the authenticated account.",
		MarkdownDescription: "Reads installed Stremio addons for the authenticated account.\n\n## Example Usage\n\n```hcl\ndata \"stremio_installed_addons\" \"current\" {}\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Static identifier for this data source.",
				MarkdownDescription: "Static identifier for this data source instance.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional filter for addon-supported content type (for example `movie`, `series`, `tv`).",
				MarkdownDescription: "Optional filter for addon-supported content type (for example `movie`, `series`, `tv`).",
			},
			"catalog_type": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional filter for addon catalog type.",
				MarkdownDescription: "Optional filter for addon catalog type.",
			},
			"resource": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional filter for addon resource name (for example `catalog`, `meta`, `stream`).",
				MarkdownDescription: "Optional filter for addon resource name (for example `catalog`, `meta`, `stream`).",
			},
			"name_regex": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional regular expression filter applied to addon name.",
				MarkdownDescription: "Optional regular expression filter applied to addon name.",
			},
			"sort_by": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional sort field: `name`, `addon_id`, or `version`.",
				MarkdownDescription: "Optional sort field: `name`, `addon_id`, or `version`.",
			},
			"sort_order": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional sort order: `asc` or `desc`. Defaults to `asc` when `sort_by` is set.",
				MarkdownDescription: "Optional sort order: `asc` or `desc`. Defaults to `asc` when `sort_by` is set.",
			},
			"addons": schema.ListNestedAttribute{
				Computed:            true,
				Description:         "Installed add-on descriptors.",
				MarkdownDescription: "List of installed addon descriptors.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"addon_id": schema.StringAttribute{
							Computed:            true,
							Description:         "Addon identifier from manifest metadata when available.",
							MarkdownDescription: "Addon identifier from manifest metadata when available.",
						},
						"transport_url": schema.StringAttribute{
							Computed:            true,
							Description:         "Addon transport URL.",
							MarkdownDescription: "Addon `transportUrl` value.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							Description:         "Addon display name, if available.",
							MarkdownDescription: "Addon display name when provided by descriptor metadata.",
						},
						"version": schema.StringAttribute{
							Computed:            true,
							Description:         "Addon version from manifest metadata when available.",
							MarkdownDescription: "Addon version from manifest metadata when available.",
						},
						"types": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							Description:         "Content types this addon supports when available.",
							MarkdownDescription: "Content types this addon supports when available.",
						},
						"catalog_types": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							Description:         "Catalog types advertised by addon catalogs when available.",
							MarkdownDescription: "Catalog types advertised by addon catalogs when available.",
						},
						"resources": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							Description:         "Resource names advertised by the addon when available.",
							MarkdownDescription: "Resource names advertised by the addon when available.",
						},
					},
				},
			},
		},
	}
}

func (d *installedAddonsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *installedAddonsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var config installedAddonsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filterType := strings.ToLower(strings.TrimSpace(config.Type.ValueString()))
	filterCatalogType := strings.ToLower(strings.TrimSpace(config.CatalogType.ValueString()))
	filterResource := strings.ToLower(strings.TrimSpace(config.Resource.ValueString()))
	nameRegexPattern := strings.TrimSpace(config.NameRegex.ValueString())
	sortBy := strings.ToLower(strings.TrimSpace(config.SortBy.ValueString()))
	sortOrder := strings.ToLower(strings.TrimSpace(config.SortOrder.ValueString()))
	if sortOrder == "" {
		sortOrder = "asc"
	}
	if sortBy != "" && sortBy != "name" && sortBy != "addon_id" && sortBy != "version" {
		resp.Diagnostics.AddError("Invalid sort_by", "Valid values are: name, addon_id, version.")
		return
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		resp.Diagnostics.AddError("Invalid sort_order", "Valid values are: asc, desc.")
		return
	}

	var nameRegex *regexp.Regexp
	if nameRegexPattern != "" {
		compiledRegex, err := regexp.Compile(nameRegexPattern)
		if err != nil {
			resp.Diagnostics.AddError("Invalid name_regex", err.Error())
			return
		}
		nameRegex = compiledRegex
	}

	addons, err := d.client.InstalledAddons(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read installed addons",
			err.Error(),
		)
		return
	}

	addonObjects := make([]addonModel, 0, len(addons))
	for _, addon := range addons {
		if filterType != "" && !containsIgnoreCase(addon.Types, filterType) {
			continue
		}
		if filterCatalogType != "" && !containsIgnoreCase(addon.CatalogTypes, filterCatalogType) {
			continue
		}
		if filterResource != "" && !containsIgnoreCase(addon.Resources, filterResource) {
			continue
		}
		if nameRegex != nil && !nameRegex.MatchString(addon.Name) {
			continue
		}

		typesValue, diags := types.ListValueFrom(ctx, types.StringType, addon.Types)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		catalogTypesValue, diags := types.ListValueFrom(ctx, types.StringType, addon.CatalogTypes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		resourcesValue, diags := types.ListValueFrom(ctx, types.StringType, addon.Resources)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		addonObjects = append(addonObjects, addonModel{
			AddonID:      types.StringValue(addon.AddonID),
			TransportURL: types.StringValue(addon.TransportURL),
			Name:         types.StringValue(addon.Name),
			Version:      types.StringValue(addon.Version),
			Types:        typesValue,
			CatalogTypes: catalogTypesValue,
			Resources:    resourcesValue,
		})
	}

	if sortBy != "" {
		descending := sortOrder == "desc"
		sort.SliceStable(addonObjects, func(i, j int) bool {
			left := ""
			right := ""
			switch sortBy {
			case "name":
				left = strings.ToLower(addonObjects[i].Name.ValueString())
				right = strings.ToLower(addonObjects[j].Name.ValueString())
			case "addon_id":
				left = strings.ToLower(addonObjects[i].AddonID.ValueString())
				right = strings.ToLower(addonObjects[j].AddonID.ValueString())
			case "version":
				left = strings.ToLower(addonObjects[i].Version.ValueString())
				right = strings.ToLower(addonObjects[j].Version.ValueString())
			}
			if descending {
				return left > right
			}
			return left < right
		})
	}

	addonsValue, diags := types.ListValueFrom(ctx, addonModelObjectType, addonObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := installedAddonsDataSourceModel{
		ID:          types.StringValue("installed-addons"),
		Type:        config.Type,
		CatalogType: config.CatalogType,
		Resource:    config.Resource,
		NameRegex:   config.NameRegex,
		SortBy:      config.SortBy,
		SortOrder:   config.SortOrder,
		Addons:      addonsValue,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func containsIgnoreCase(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			return true
		}
	}
	return false
}
