package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &addonCollectionResource{}
var _ resource.ResourceWithImportState = &addonCollectionResource{}

func NewAddonCollectionResource() resource.Resource {
	return &addonCollectionResource{}
}

type addonCollectionResource struct {
	client *client
}

type addonCollectionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Email         types.String `tfsdk:"email"`
	Password      types.String `tfsdk:"password"`
	TransportURLs types.Set    `tfsdk:"transport_urls"`
}

func (r *addonCollectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_collection"
}

func (r *addonCollectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages the Stremio addon collection for the authenticated account.",
		MarkdownDescription: "Manages the full Stremio addon collection for the authenticated account.\n\nTerraform treats this resource as authoritative and will add/remove addons to match `transport_urls`.\n\n## Example Usage\n\n```hcl\nresource \"stremio_addon_collection\" \"main\" {\n  transport_urls = [\n    \"https://v3-cinemeta.strem.io/manifest.json\",\n    \"https://opensubtitles-v3.strem.io/manifest.json\",\n  ]\n}\n```\n\n## Import\n\n```bash\nterraform import stremio_addon_collection.main addon-collection\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Static resource ID.",
				MarkdownDescription: "Static resource identifier (`addon-collection`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Optional:            true,
				Description:         "Optional account email for this resource. Use with password to manage multiple accounts in one configuration.",
				MarkdownDescription: "Optional account email for this resource. If set (with `password`), the resource authenticates with these credentials instead of provider-level credentials.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				Description:         "Optional account password for this resource.",
				MarkdownDescription: "Optional account password for this resource. Required when `email` is set.",
			},
			"transport_urls": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				Description:         "Desired addon transport URLs. Terraform adds/removes to match this set.",
				MarkdownDescription: "Set of desired addon manifest `transportUrl` values. Add a URL to install it, remove a URL to uninstall it.",
			},
		},
	}
}

func (r *addonCollectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = c
}

func (r *addonCollectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan addonCollectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	activeClient, err := r.authenticatedClient(ctx, plan.Email, plan.Password)
	if err != nil {
		resp.Diagnostics.AddError("Unable to authenticate addon collection resource", err.Error())
		return
	}

	transportURLs := make([]string, 0)
	resp.Diagnostics.Append(plan.TransportURLs.ElementsAs(ctx, &transportURLs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = activeClient.SetInstalledAddons(ctx, transportURLs)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update addon collection", err.Error())
		return
	}

	plan.ID = types.StringValue("addon-collection")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *addonCollectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state addonCollectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	activeClient, err := r.authenticatedClient(ctx, state.Email, state.Password)
	if err != nil {
		resp.Diagnostics.AddError("Unable to authenticate addon collection resource", err.Error())
		return
	}

	addons, err := activeClient.InstalledAddons(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read addon collection", err.Error())
		return
	}

	transportURLs := make([]string, 0, len(addons))
	for _, item := range addons {
		if item.TransportURL == "" {
			continue
		}
		transportURLs = append(transportURLs, item.TransportURL)
	}

	setValue, diags := types.SetValueFrom(ctx, types.StringType, transportURLs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue("addon-collection")
	state.TransportURLs = setValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *addonCollectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan addonCollectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	activeClient, err := r.authenticatedClient(ctx, plan.Email, plan.Password)
	if err != nil {
		resp.Diagnostics.AddError("Unable to authenticate addon collection resource", err.Error())
		return
	}

	transportURLs := make([]string, 0)
	resp.Diagnostics.Append(plan.TransportURLs.ElementsAs(ctx, &transportURLs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = activeClient.SetInstalledAddons(ctx, transportURLs)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update addon collection", err.Error())
		return
	}

	plan.ID = types.StringValue("addon-collection")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *addonCollectionResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state addonCollectionResourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	activeClient, err := r.authenticatedClient(ctx, state.Email, state.Password)
	if err == nil {
		_ = activeClient.SetInstalledAddons(ctx, []string{})
	}
	resp.State.RemoveResource(ctx)
}

func (r *addonCollectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *addonCollectionResource) authenticatedClient(ctx context.Context, email, password types.String) (*client, error) {
	if r.client == nil {
		return nil, fmt.Errorf("provider client is not available")
	}

	hasEmail := !email.IsNull() && !email.IsUnknown() && email.ValueString() != ""
	hasPassword := !password.IsNull() && !password.IsUnknown() && password.ValueString() != ""

	if hasEmail || hasPassword {
		if !hasEmail || !hasPassword {
			return nil, fmt.Errorf("both email and password must be provided together")
		}

		resourceClient, err := newClient(r.client.baseURL.String())
		if err != nil {
			return nil, err
		}

		err = resourceClient.Login(ctx, email.ValueString(), password.ValueString())
		if err != nil {
			return nil, err
		}

		return resourceClient, nil
	}

	if r.client.authKey == "" {
		return nil, fmt.Errorf("provider-level credentials are missing; set provider email/password or resource email/password")
	}

	return r.client, nil
}
