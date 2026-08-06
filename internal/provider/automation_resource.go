package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sazabi/terraform-provider-sazabi/internal/client"
)

// automationResource implements sazabi_automation as a full-CRUD resource.
//
// It maps to the public API's automations.create/.get/.update plus
// .enable/.disable. The automation runs a durable project script (see
// sazabi_script) on a cron schedule. Two API constraints shape the resource:
//
//   - automations.update cannot change which script an automation runs, so
//     script/script_id force replacement.
//   - The public API has no delete-automation operation (delete is admin-only),
//     so destroy disables the automation and removes it from state without
//     deleting it server-side. See Delete for details.
//
// This is a breaking change from the previous toggle-only resource, which
// adopted an existing automation by a required automation_id. The automation
// is now created and owned by Terraform; automation_id no longer exists (use
// the computed id, or terraform import, to adopt an existing automation).
type automationResource struct {
	providerData *ProviderData
}

type automationResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Script         types.String `tfsdk:"script"`
	ScriptID       types.String `tfsdk:"script_id"`
	ScriptName     types.String `tfsdk:"script_name"`
	CronExpression types.String `tfsdk:"cron_expression"`
	Timezone       types.String `tfsdk:"timezone"`
	TimeoutSeconds types.Int64  `tfsdk:"timeout_seconds"`
	Enabled        types.Bool   `tfsdk:"enabled"`
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
		Description: "A Sazabi automation: a durable project script (see sazabi_script) run on a cron schedule. " +
			"Name, description, and schedule update in place; the script it runs is immutable and forces " +
			"replacement. The public API cannot delete automations, so destroy disables the automation and " +
			"removes it from Terraform state without deleting it server-side.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Automation ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Automation name (1–200 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 200),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional human-readable description (max 500 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 500),
				},
			},
			"script": schema.StringAttribute{
				Optional: true,
				Description: "Name of the project script (sazabi_script) this automation runs. Provide exactly one of " +
					"script or script_id. Changing the script forces replacement.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(path.MatchRoot("script_id")),
					stringvalidator.ExactlyOneOf(path.MatchRoot("script"), path.MatchRoot("script_id")),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"script_id": schema.StringAttribute{
				Optional: true,
				Description: "ID of the project script (sazabi_script) this automation runs. Provide exactly one of " +
					"script or script_id. Changing the script forces replacement.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"script_name": schema.StringAttribute{
				Computed:    true,
				Description: "Resolved name of the project script this automation runs.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cron_expression": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cron schedule for the automation. Defaults to every minute (\"* * * * *\") server-side.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timezone": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IANA timezone for cron_expression. Defaults to UTC server-side.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Execution timeout in seconds (1–3600). Defaults to 60 server-side.",
				Validators: []validator.Int64{
					int64validator.Between(1, 3600),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the automation runs on its schedule. Defaults to true.",
				Default:     booldefault.StaticBool(true),
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

func (r *automationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.CreateAutomationInput{
		ProjectID:      plan.ProjectID.ValueString(),
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueStringPointer(),
		ScriptID:       plan.ScriptID.ValueString(),
		Script:         plan.Script.ValueString(),
		CronExpression: plan.CronExpression.ValueString(),
		Timezone:       plan.Timezone.ValueString(),
		TimeoutSeconds: plan.TimeoutSeconds.ValueInt64Pointer(),
		Enabled:        plan.Enabled.ValueBoolPointer(),
	}

	automation, err := r.providerData.Client.CreateAutomation(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create automation", err.Error())
		return
	}

	// The create input takes script/script_id, but the API response reports
	// scriptId/scriptName. Preserve the configured script identity in state
	// so the config-supplied attribute (script vs script_id) does not drift.
	resp.Diagnostics.Append(resp.State.Set(ctx, automationModelFromAPI(automation, plan))...)
}

func (r *automationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	automation, err := r.providerData.Client.GetAutomation(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read automation", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, automationModelFromAPI(automation, state))...)
}

func (r *automationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state automationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	automationID := state.ID.ValueString()
	projectID := plan.ProjectID.ValueString()

	// automations.update covers name/description/schedule. Script identity is
	// RequiresReplace, and enabled is toggled through enable/disable below.
	automation, err := r.providerData.Client.UpdateAutomation(ctx, automationID, projectID, client.UpdateAutomationInput{
		Name:           plan.Name.ValueString(),
		Description:    plan.Description.ValueStringPointer(),
		CronExpression: plan.CronExpression.ValueString(),
		Timezone:       plan.Timezone.ValueString(),
		TimeoutSeconds: plan.TimeoutSeconds.ValueInt64Pointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update automation", err.Error())
		return
	}

	if automation.Enabled != plan.Enabled.ValueBool() {
		automation, err = r.providerData.Client.SetAutomationEnabled(ctx, automationID, projectID, plan.Enabled.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError("Failed to set automation enabled state", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, automationModelFromAPI(automation, plan))...)
}

// Delete disables the automation and drops it from state. The public API has
// no delete-automation operation, so the automation continues to exist
// server-side (paused). Removing it entirely requires the dashboard or the
// sazabi CLI.
func (r *automationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state automationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	automationID := state.ID.ValueString()
	projectID := state.ProjectID.ValueString()

	_, err := r.providerData.Client.SetAutomationEnabled(ctx, automationID, projectID, false)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to disable automation on destroy", err.Error())
		return
	}

	resp.Diagnostics.AddWarning(
		"Automation disabled, not deleted",
		fmt.Sprintf(
			"The Sazabi public API cannot delete automations. Automation %q (%s) was disabled and removed from "+
				"Terraform state, but still exists (paused) in Sazabi. Delete it from the dashboard or with the "+
				"sazabi CLI if you want it gone.",
			state.Name.ValueString(), automationID,
		),
	)
}

// ImportState imports an automation by its ID (optionally "projectId/id").
func (r *automationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, projectID := parseScriptImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

// automationModelFromAPI builds resource state from the API response,
// preserving the config-supplied script identity (script vs script_id) from
// prior so the chosen attribute does not drift between plan and state.
func automationModelFromAPI(automation *client.Automation, prior automationResourceModel) automationResourceModel {
	model := automationResourceModel{
		ID:             types.StringValue(automation.ID),
		ProjectID:      types.StringValue(automation.ProjectID),
		Name:           types.StringValue(automation.Name),
		Description:    types.StringPointerValue(automation.Description),
		Script:         prior.Script,
		ScriptID:       types.StringPointerValue(automation.ScriptID),
		ScriptName:     types.StringPointerValue(automation.ScriptName),
		CronExpression: types.StringPointerValue(automation.CronExpression),
		Timezone:       types.StringValue(automation.Timezone),
		TimeoutSeconds: types.Int64PointerValue(automation.TimeoutSeconds),
		Enabled:        types.BoolValue(automation.Enabled),
	}

	// When the user configured the script by name (script), keep script_id
	// null in state and vice versa, so exactly the configured attribute is
	// tracked. On import both are populated from the API.
	if !prior.Script.IsNull() && !prior.Script.IsUnknown() {
		model.ScriptID = types.StringNull()
	}

	return model
}
