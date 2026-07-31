package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sazabi/terraform-provider-sazabi/internal/client"
)

// apiKeyResource implements sazabi_api_key over the secret-keys endpoints,
// the one full-CRUD resource in Phase 1. The plaintext key value is returned
// only by create; it lives in state as a sensitive attribute and can never
// be drift-checked or re-fetched afterwards (imports leave it empty).
type apiKeyResource struct {
	providerData *ProviderData
}

type apiKeyResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	ProjectID types.String `tfsdk:"project_id"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	Value     types.String `tfsdk:"value"`
	CreatedAt types.String `tfsdk:"created_at"`
}

var (
	_ resource.Resource                = (*apiKeyResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiKeyResource)(nil)
	_ resource.ResourceWithImportState = (*apiKeyResource)(nil)
)

// NewAPIKeyResource returns the sazabi_api_key resource factory.
func NewAPIKeyResource() resource.Resource {
	return &apiKeyResource{}
}

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sazabi secret API key. The plaintext value is returned once at creation and stored in " +
			"Terraform state (sensitive); the API never returns it again, so imported keys have an empty value.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Key ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable key name (≤100 characters; letters, numbers, spaces, hyphens, underscores).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "Project to scope the key to. Omit for an organization-wide key. Changing it requires replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Optional:    true,
				Description: "Expiration timestamp (RFC 3339). Remove to clear the expiration.",
			},
			"value": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Plaintext key value (sazabi_secret_...). Returned only at creation; empty for imported keys.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (RFC 3339).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *ProviderData, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return
	}
	r.providerData = providerData
}

func (r *apiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.providerData.Client.CreateSecretKey(ctx, client.CreateSecretKeyInput{
		ProjectID: plan.ProjectID.ValueString(),
		Name:      plan.Name.ValueString(),
		ExpiresAt: plan.ExpiresAt.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create API key", err.Error())
		return
	}

	model := apiKeyModelFromAPI(key)
	model.Value = types.StringValue(key.Value)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.providerData.Client.GetSecretKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read API key", err.Error())
		return
	}

	model := apiKeyModelFromAPI(key)
	// The API never returns the plaintext value after creation; carry the
	// stored value forward (empty for imported keys).
	model.Value = state.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state apiKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.UpdateSecretKeyInput{Name: plan.Name.ValueString()}
	if !plan.ExpiresAt.Equal(state.ExpiresAt) {
		expiresAt := plan.ExpiresAt.ValueStringPointer()
		input.ExpiresAt = &expiresAt
	}

	key, err := r.providerData.Client.UpdateSecretKey(ctx, state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update API key", err.Error())
		return
	}

	model := apiKeyModelFromAPI(key)
	model.Value = state.Value
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.providerData.Client.DeleteSecretKey(ctx, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete API key", err.Error())
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// The plaintext value is unrecoverable for imported keys; record it as
	// empty rather than unknown so plans converge.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("value"), "")...)
}

func apiKeyModelFromAPI(key *client.SecretKey) apiKeyResourceModel {
	return apiKeyResourceModel{
		ID:        types.StringValue(key.ID),
		Name:      types.StringValue(key.Name),
		ProjectID: types.StringPointerValue(key.ProjectID),
		ExpiresAt: types.StringPointerValue(key.ExpiresAt),
		CreatedAt: types.StringValue(key.CreatedAt),
	}
}
