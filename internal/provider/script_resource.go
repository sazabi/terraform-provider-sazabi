package provider

import (
	"context"
	"fmt"
	"regexp"

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

// scriptNameRegex mirrors ProjectScriptNameSchema's pattern in
// packages/public-api-contracts/src/scripts.ts.
var scriptNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

const scriptNameRegexMessage = "name must start with a letter or digit and contain only letters, digits, underscores, and hyphens (max 64 characters)"

// scriptResource implements sazabi_script, a full-CRUD resource backing the
// durable bash scripts stored per project (scripts.create/.get/.update/
// .delete). Scripts are keyed by name within a project, so the API addresses
// get/update/delete by name; renaming therefore forces replacement. Delete
// is a soft delete.
type scriptResource struct {
	providerData *ProviderData
}

type scriptResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Content     types.String `tfsdk:"content"`
	Description types.String `tfsdk:"description"`
	ContentHash types.String `tfsdk:"content_hash"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

var (
	_ resource.Resource                = (*scriptResource)(nil)
	_ resource.ResourceWithConfigure   = (*scriptResource)(nil)
	_ resource.ResourceWithImportState = (*scriptResource)(nil)
)

// NewScriptResource returns the sazabi_script resource factory.
func NewScriptResource() resource.Resource {
	return &scriptResource{}
}

func (r *scriptResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_script"
}

func (r *scriptResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A durable bash script stored in a Sazabi project, materialized as " +
			"/home/sazabi/scripts/<name>.sh in the sandbox and run by scheduled automations " +
			"(see sazabi_automation). Scripts are keyed by name within a project, so renaming " +
			"forces replacement. Destroy soft-deletes the script.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Script ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Project that owns the script. Defaults to the API key's project scope.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Script name, unique within the project among non-deleted scripts. Must start with a " +
					"letter or digit and contain only letters, digits, underscores, and hyphens (max 64 characters). " +
					"Renaming requires replacement.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(scriptNameRegex, scriptNameRegexMessage),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"content": schema.StringAttribute{
				Required:    true,
				Description: "Bash script body (at most 1 MiB). Commonly sourced with the file() function.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional human-readable description (max 500 characters).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 500),
				},
			},
			"content_hash": schema.StringAttribute{
				Computed:    true,
				Description: "sha256 hex digest of the script content, computed server-side.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC 3339 timestamp when the script was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC 3339 timestamp when the script was last updated.",
			},
		},
	}
}

func (r *scriptResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *scriptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan scriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	script, err := r.providerData.Client.CreateScript(ctx, client.CreateScriptInput{
		ProjectID:   plan.ProjectID.ValueString(),
		Name:        plan.Name.ValueString(),
		Content:     plan.Content.ValueString(),
		Description: plan.Description.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create script", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, scriptModelFromAPI(script))...)
}

func (r *scriptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state scriptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	script, err := r.providerData.Client.GetScript(ctx, state.Name.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read script", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, scriptModelFromAPI(script))...)
}

func (r *scriptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan scriptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// name is RequiresReplace, so only content/description can reach here.
	// Terraform manages the full body, so always send content.
	script, err := r.providerData.Client.UpdateScript(ctx, plan.Name.ValueString(), plan.ProjectID.ValueString(), client.UpdateScriptInput{
		Content:     plan.Content.ValueStringPointer(),
		Description: plan.Description.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update script", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, scriptModelFromAPI(script))...)
}

func (r *scriptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state scriptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.providerData.Client.DeleteScript(ctx, state.Name.ValueString(), state.ProjectID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete script", err.Error())
	}
}

// ImportState imports a script by name (optionally "name" or "projectId/name").
func (r *scriptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name, projectID := parseScriptImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	if projectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	}
}

// parseScriptImportID splits an import ID of the form "name" or
// "projectId/name" into its parts. A leading "projectId/" is optional.
func parseScriptImportID(id string) (name, projectID string) {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '/' {
			return id[i+1:], id[:i]
		}
	}
	return id, ""
}

func scriptModelFromAPI(script *client.Script) scriptResourceModel {
	return scriptResourceModel{
		ID:          types.StringValue(script.ID),
		ProjectID:   types.StringValue(script.ProjectID),
		Name:        types.StringValue(script.Name),
		Content:     types.StringValue(script.Content),
		Description: types.StringPointerValue(script.Description),
		ContentHash: types.StringValue(script.ContentHash),
		CreatedAt:   types.StringValue(script.CreatedAt),
		UpdatedAt:   types.StringValue(script.UpdatedAt),
	}
}
