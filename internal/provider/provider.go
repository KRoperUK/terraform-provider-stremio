package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &stremioProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &stremioProvider{version: version}
	}
}

type stremioProvider struct {
	version string
}

type stremioProviderModel struct {
	BaseURL  types.String `tfsdk:"base_url"`
	Email    types.String `tfsdk:"email"`
	Password types.String `tfsdk:"password"`
}

func (p *stremioProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "stremio"
	resp.Version = p.version
}

func (p *stremioProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Terraform provider for managing Stremio account-level operations.",
		MarkdownDescription: "Terraform provider for Stremio account operations, including authentication, account management, and addon collection management.\n\n## Example Usage\n\n```hcl\nprovider \"stremio\" {\n  base_url = \"https://api.strem.io\"\n  email    = var.stremio_email\n  password = var.stremio_password\n}\n```",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:            true,
				Description:         "Stremio API base URL.",
				MarkdownDescription: "Base URL for the Stremio API. Defaults to `https://api.strem.io`.",
			},
			"email": schema.StringAttribute{
				Optional:            true,
				Description:         "Email used to authenticate against Stremio.",
				MarkdownDescription: "Account email used for provider-level authentication.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Password used to authenticate against Stremio.",
				MarkdownDescription: "Account password used for provider-level authentication.",
			},
		},
	}
}

func (p *stremioProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data stremioProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := "https://api.strem.io"
	if !data.BaseURL.IsNull() && !data.BaseURL.IsUnknown() {
		baseURL = data.BaseURL.ValueString()
	}

	client, err := newClient(baseURL)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("base_url"),
			"Invalid base_url",
			fmt.Sprintf("Unable to create Stremio client: %s", err),
		)
		return
	}

	if !data.Email.IsNull() && !data.Email.IsUnknown() && !data.Password.IsNull() && !data.Password.IsUnknown() {
		err = client.Login(ctx, data.Email.ValueString(), data.Password.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to authenticate provider",
				fmt.Sprintf("Error logging in with provider credentials: %s", err),
			)
			return
		}
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *stremioProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAccountResource,
		NewAddonCollectionResource,
	}
}

func (p *stremioProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewInstalledAddonsDataSource,
		NewWatchHistoryDataSource,
		NewContinueWatchingDataSource,
	}
}
