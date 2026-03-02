package provider

import (
	"context"
	"fmt"

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
	ID     types.String `tfsdk:"id"`
	Addons types.List   `tfsdk:"addons"`
}

func (d *installedAddonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_installed_addons"
}

func (d *installedAddonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns installed Stremio addons for the authenticated account.",
		MarkdownDescription: "Reads installed Stremio addons for the authenticated account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Static identifier for this data source.",
				MarkdownDescription: "Static identifier for this data source instance.",
			},
			"addons": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Installed add-on descriptors.",
				MarkdownDescription: "List of installed addon descriptors.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
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

func (d *installedAddonsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
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
		addonObjects = append(addonObjects, addonModel{
			TransportURL: types.StringValue(addon.TransportURL),
			Name:         types.StringValue(addon.Name),
		})
	}

	addonsValue, diags := types.ListValueFrom(ctx, addonModelObjectType, addonObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := installedAddonsDataSourceModel{
		ID:     types.StringValue("installed-addons"),
		Addons: addonsValue,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
