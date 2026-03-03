package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &accountResource{}
var _ resource.ResourceWithImportState = &accountResource{}

func NewAccountResource() resource.Resource {
	return &accountResource{}
}

type accountResource struct {
	client *client
}

type accountResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Email         types.String `tfsdk:"email"`
	Password      types.String `tfsdk:"password"`
	AuthKey       types.String `tfsdk:"auth_key"`
	UserID        types.String `tfsdk:"user_id"`
	TransportURLs types.Set    `tfsdk:"transport_urls"`
}

func (r *accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Creates or imports a Stremio account using email/password.",
		MarkdownDescription: "Manages a Stremio account using email and password credentials.\n\nUse this resource to create a new account, or import an existing account with `email:password`.\n\n## Example Usage\n\n```hcl\nresource \"stremio_account\" \"user\" {\n  email    = var.stremio_email\n  password = var.stremio_password\n}\n```\n\n## Import\n\n```bash\nterraform import stremio_account.user 'user@example.com:super-secret-password'\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Terraform resource ID. Uses the account email.",
				MarkdownDescription: "Terraform resource ID, set to the account email.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Required:            true,
				Description:         "Stremio account email.",
				MarkdownDescription: "Email address for the Stremio account.",
			},
			"password": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				Description:         "Stremio account password.",
				MarkdownDescription: "Password for the Stremio account.",
			},
			"auth_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				Description:         "Authentication auth key returned by Stremio.",
				MarkdownDescription: "Computed auth key returned by Stremio after successful authentication.",
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				Description:         "Stremio user identifier.",
				MarkdownDescription: "Computed unique Stremio user ID.",
			},
			"transport_urls": schema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				Description:         "Desired addon transport URLs. When set, Terraform manages the addon collection to match this set.",
				MarkdownDescription: "Optional set of desired addon manifest `transportUrl` values. When set, Terraform will add/remove addons to match this set. When not set, the addon collection is not managed by this resource.",
			},
		},
	}
}

func (r *accountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var plan accountResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Register(ctx, plan.Email.ValueString(), plan.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Stremio account",
			err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(plan.Email.ValueString())
	plan.AuthKey = types.StringValue(r.client.authKey)
	plan.UserID = types.StringValue(r.client.userID)

	if !plan.TransportURLs.IsNull() && !plan.TransportURLs.IsUnknown() {
		transportURLs := make([]string, 0)
		resp.Diagnostics.Append(plan.TransportURLs.ElementsAs(ctx, &transportURLs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := r.client.SetInstalledAddons(ctx, transportURLs); err != nil {
			resp.Diagnostics.AddError("Unable to set addon collection", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *accountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var state accountResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Login(ctx, state.Email.ValueString(), state.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Stremio account",
			err.Error(),
		)
		return
	}

	state.AuthKey = types.StringValue(r.client.authKey)
	state.UserID = types.StringValue(r.client.userID)

	if !state.TransportURLs.IsNull() {
		addons, err := r.client.InstalledAddons(ctx)
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
		state.TransportURLs = setValue
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var plan accountResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Login(ctx, plan.Email.ValueString(), plan.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update Stremio account",
			err.Error(),
		)
		return
	}

	plan.ID = types.StringValue(plan.Email.ValueString())
	plan.AuthKey = types.StringValue(r.client.authKey)
	plan.UserID = types.StringValue(r.client.userID)

	if !plan.TransportURLs.IsNull() && !plan.TransportURLs.IsUnknown() {
		transportURLs := make([]string, 0)
		resp.Diagnostics.Append(plan.TransportURLs.ElementsAs(ctx, &transportURLs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := r.client.SetInstalledAddons(ctx, transportURLs); err != nil {
			resp.Diagnostics.AddError("Unable to update addon collection", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	var state accountResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Login(ctx, state.Email.ValueString(), state.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to authenticate before deleting account", err.Error())
		return
	}

	err = r.client.DeleteUser(ctx, state.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete Stremio account", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *accountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "The provider client is not available.")
		return
	}

	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Use the format email:password when importing stremio_account.",
		)
		return
	}

	email := parts[0]
	password := parts[1]

	err := r.client.Login(ctx, email, password)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import Stremio account",
			err.Error(),
		)
		return
	}

	state := accountResourceModel{
		ID:            types.StringValue(email),
		Email:         types.StringValue(email),
		Password:      types.StringValue(password),
		AuthKey:       types.StringValue(r.client.authKey),
		UserID:        types.StringValue(r.client.userID),
		TransportURLs: types.SetNull(types.StringType),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
