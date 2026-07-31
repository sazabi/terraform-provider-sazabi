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

	"github.com/sazabi/terraform-provider-sazabi/internal/client"
)

// publicKeyLogForwardingResource implements sazabi_public_key_log_forwarding.
//
// The backing operation, publicKeys.ensureLogForwarding, is an upsert keyed
// by project: it adopts the project's existing active forwarding key when
// one exists, mints one otherwise, and returns the plaintext value on every
// call (the key is stored encrypted server-side). There is no generic
// public-key create and no hard delete — destroy soft-disables the key via
// publicKeys.deactivate. The resource has no user-mutable attributes beyond
// its identity, so Update is unreachable.
type publicKeyLogForwardingResource struct {
	providerData *ProviderData
}

type publicKeyLogForwardingResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Value     types.String `tfsdk:"value"`
	CreatedAt types.String `tfsdk:"created_at"`
}

var (
	_ resource.Resource                = (*publicKeyLogForwardingResource)(nil)
	_ resource.ResourceWithConfigure   = (*publicKeyLogForwardingResource)(nil)
	_ resource.ResourceWithImportState = (*publicKeyLogForwardingResource)(nil)
)

// NewPublicKeyLogForwardingResource returns the sazabi_public_key_log_forwarding resource factory.
func NewPublicKeyLogForwardingResource() resource.Resource {
	return &publicKeyLogForwardingResource{}
}

func (r *publicKeyLogForwardingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_public_key_log_forwarding"
}

func (r *publicKeyLogForwardingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The reusable log-forwarding public key for a project. The backing API operation is an " +
			"upsert: it adopts the project's existing active forwarding key or mints one, and returns the plaintext " +
			"value on every call. Destroy soft-disables the key (deactivate); there is no hard delete.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Public key ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Project the key forwards logs for. Defaults to the API key's project scope.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Key name (always the shared log-forwarding key name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"value": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Plaintext public key value (sazabi_public_...). Recoverable on every apply via the ensure operation.",
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

func (r *publicKeyLogForwardingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *publicKeyLogForwardingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan publicKeyLogForwardingResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.providerData.Client.EnsureLogForwardingPublicKey(ctx, plan.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to ensure log forwarding public key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, publicKeyLogForwardingResourceModel{
		ID:        types.StringValue(key.ID),
		ProjectID: types.StringValue(key.ProjectID),
		Name:      types.StringValue(key.Name),
		Value:     types.StringValue(key.Value),
		CreatedAt: types.StringValue(key.CreatedAt),
	})...)
}

func (r *publicKeyLogForwardingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state publicKeyLogForwardingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.providerData.Client.GetPublicKey(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read log forwarding public key", err.Error())
		return
	}
	if key.DeactivatedAt != nil {
		// Deactivated out-of-band: the ensure operation would mint a
		// replacement, so treat this key as gone.
		resp.State.RemoveResource(ctx)
		return
	}

	// GET never returns the plaintext value; carry it forward from state.
	state.Name = types.StringValue(key.Name)
	state.ProjectID = types.StringValue(key.ProjectID)
	state.CreatedAt = types.StringValue(key.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *publicKeyLogForwardingResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Unreachable: the only configurable attribute (project_id) is marked
	// RequiresReplace and everything else is computed.
	resp.Diagnostics.AddError(
		"Log forwarding keys cannot be updated",
		"The log forwarding public key has no user-mutable attributes. This plan reaching Update is a bug in the provider.",
	)
}

func (r *publicKeyLogForwardingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state publicKeyLogForwardingResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.providerData.Client.DeactivatePublicKey(ctx, state.ID.ValueString(), state.ProjectID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to deactivate log forwarding public key", err.Error())
	}
}

func (r *publicKeyLogForwardingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("value"), "")...)
}
