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

// statusComponentResource implements sazabi_status_component.
//
// The API's write surface is register (an upsert by name within a project)
// and deregister (a soft delete). Create adopts an existing active component
// with the same name instead of erroring — register semantics. Update
// re-registers with the same name, which refreshes the description. The
// description cannot be *cleared* through register (the contract rejects
// empty strings), so removing it from config requires replacement.
type statusComponentResource struct {
	providerData *ProviderData
}

type statusComponentResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

var (
	_ resource.Resource                = (*statusComponentResource)(nil)
	_ resource.ResourceWithConfigure   = (*statusComponentResource)(nil)
	_ resource.ResourceWithImportState = (*statusComponentResource)(nil)
)

// NewStatusComponentResource returns the sazabi_status_component resource factory.
func NewStatusComponentResource() resource.Resource {
	return &statusComponentResource{}
}

func (r *statusComponentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_status_component"
}

func (r *statusComponentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sazabi status page component. Registering a name that already exists in the project adopts " +
			"the existing component (the API's register operation is an upsert by name). Destroy soft-deletes via deregister. " +
			"Volatile runtime fields (current status, first/last seen) are intentionally not tracked as state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Status component ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Project the component belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Component name, unique among active components within the project. Renaming requires replacement.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Component description. Can be updated in place, but not cleared — the API rejects empty descriptions, so removing it requires replacement.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						descriptionCleared,
						"Removing the description requires replacing the component; the API cannot clear it in place.",
						"Removing the description requires replacing the component; the API cannot clear it in place.",
					),
				},
			},
		},
	}
}

// descriptionCleared triggers replacement only when a previously-set
// description is removed from configuration.
func descriptionCleared(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = !req.StateValue.IsNull() && req.ConfigValue.IsNull()
}

func (r *statusComponentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *statusComponentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan statusComponentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	component, err := r.providerData.Client.RegisterStatusComponent(ctx, client.RegisterStatusComponentInput{
		ProjectID:   plan.ProjectID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to register status component", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, statusComponentModelFromAPI(component))...)
}

func (r *statusComponentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state statusComponentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	component, err := r.providerData.Client.GetStatusComponent(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read status component", err.Error())
		return
	}
	if component.DeletedAt != nil {
		// Deregistered out-of-band: treat the soft-deleted component as gone.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, statusComponentModelFromAPI(component))...)
}

func (r *statusComponentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan statusComponentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-register with the same name: the API refreshes the existing active
	// component and applies the new description.
	component, err := r.providerData.Client.RegisterStatusComponent(ctx, client.RegisterStatusComponentInput{
		ProjectID:   plan.ProjectID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update status component", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, statusComponentModelFromAPI(component))...)
}

func (r *statusComponentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state statusComponentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.providerData.Client.DeregisterStatusComponent(ctx, state.ID.ValueString(), "Destroyed via Terraform"); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to deregister status component", err.Error())
	}
}

func (r *statusComponentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func statusComponentModelFromAPI(component *client.StatusComponent) statusComponentResourceModel {
	return statusComponentResourceModel{
		ID:          types.StringValue(component.ID),
		ProjectID:   types.StringValue(component.ProjectID),
		Name:        types.StringValue(component.Name),
		Description: types.StringPointerValue(component.Description),
	}
}
