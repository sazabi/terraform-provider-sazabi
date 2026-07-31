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

// automationResource implements sazabi_automation, a toggle-only resource
// per the design's partial-CRUD-honesty decision: the public API can only
// enable and disable automations (automations.enable/.disable) — creation,
// schedule edits, and deletion happen out-of-band. The resource adopts an
// existing automation by its required automation_id; destroy removes it from
// state without touching the automation.
type automationResource struct {
	providerData *ProviderData
}

type automationResourceModel struct {
	ID           types.String `tfsdk:"id"`
	AutomationID types.String `tfsdk:"automation_id"`
	ProjectID    types.String `tfsdk:"project_id"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Name         types.String `tfsdk:"name"`
}

var (
	_ resource.Resource                = (*automationResource)(nil)
	_ resource.ResourceWithConfigure   = (*automationResource)(nil)
	_ resource.ResourceWithImportState = (*automationResource)(nil)
)

// NewAutomationResource returns the sazabi_automation resource factory.
func NewAutomationResource() resource.Resource {
	return &automationResource{}
}

func (r *automationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation"
}

func (r *automationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the enabled/disabled state of an existing Sazabi automation. The public API cannot " +
			"create, edit, or delete automations — create one in the dashboard first, then adopt it here by ID. " +
			"Destroy removes the automation from state without changing its enabled state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same as automation_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"automation_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the automation to manage, created out-of-band (dashboard or agent).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Project that owns the automation. Defaults to the API key's project scope.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the automation runs on its schedule.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Automation name (read-only; edited out-of-band).",
			},
		},
	}
}

func (r *automationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyEnabled reconciles the automation's enabled state with the plan and
// returns the resulting model. Used by both Create (adopt) and Update.
func (r *automationResource) applyEnabled(ctx context.Context, plan automationResourceModel) (*automationResourceModel, error) {
	automationID := plan.AutomationID.ValueString()
	projectID := plan.ProjectID.ValueString()

	automation, err := r.providerData.Client.GetAutomation(ctx, automationID, projectID)
	if err != nil {
		return nil, err
	}
	if automation.Enabled != plan.Enabled.ValueBool() {
		if !automation.CanToggle {
			return nil, fmt.Errorf("automation %s cannot be toggled by this credential (canToggle=false)", automationID)
		}
		automation, err = r.providerData.Client.SetAutomationEnabled(ctx, automationID, projectID, plan.Enabled.ValueBool())
		if err != nil {
			return nil, err
		}
	}

	model := automationModelFromAPI(automation)
	return &model, nil
}

func (r *automationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.applyEnabled(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to adopt automation", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *automationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	automation, err := r.providerData.Client.GetAutomation(ctx, state.AutomationID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read automation", err.Error())
		return
	}

	model := automationModelFromAPI(automation)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *automationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.applyEnabled(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to toggle automation", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *automationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Automation not deleted from Sazabi",
		fmt.Sprintf(
			"The Sazabi public API cannot delete automations. Automation %q (%s) was removed from Terraform state only; "+
				"its current enabled state is unchanged. Delete it from the dashboard if you want it gone.",
			state.Name.ValueString(), state.AutomationID.ValueString(),
		),
	)
}

func (r *automationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("automation_id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func automationModelFromAPI(automation *client.Automation) automationResourceModel {
	return automationResourceModel{
		ID:           types.StringValue(automation.ID),
		AutomationID: types.StringValue(automation.ID),
		ProjectID:    types.StringValue(automation.ProjectID),
		Enabled:      types.BoolValue(automation.Enabled),
		Name:         types.StringValue(automation.Name),
	}
}
