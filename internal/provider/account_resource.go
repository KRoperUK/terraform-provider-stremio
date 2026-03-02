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
	ID       types.String `tfsdk:"id"`
	Email    types.String `tfsdk:"email"`
	Password types.String `tfsdk:"password"`
	AuthKey  types.String `tfsdk:"auth_key"`
	UserID   types.String `tfsdk:"user_id"`
}

func (r *accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account"
}

func (r *accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates or imports a Stremio account using email/password.",
		MarkdownDescription: "Manages a Stremio account using email and password credentials.\n\nUse this resource to create a new account, or import an existing account with `email:password`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform resource ID. Uses the account email.",
				MarkdownDescription: "Terraform resource ID, set to the account email.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "Stremio account email.",
				MarkdownDescription: "Email address for the Stremio account.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Stremio account password.",
				MarkdownDescription: "Password for the Stremio account.",
			},
			"auth_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Authentication auth key returned by Stremio.",
				MarkdownDescription: "Computed auth key returned by Stremio after successful authentication.",
			},
			"user_id": schema.StringAttribute{
				Computed:    true,
				Description: "Stremio user identifier.",
				MarkdownDescription: "Computed unique Stremio user ID.",
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
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
		ID:       types.StringValue(email),
		Email:    types.StringValue(email),
		Password: types.StringValue(password),
		AuthKey:  types.StringValue(r.client.authKey),
		UserID:   types.StringValue(r.client.userID),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
