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

// projectResource implements sazabi_project.
//
// The public API supports create and read only (projects.create,
// projects.get) — there is no update or delete endpoint. Every attribute
// therefore requires replacement, and Delete only removes the project from
// Terraform state with a loud warning; the underlying project keeps running.
// This models the API's real CRUD surface instead of stubbing missing
// operations (the partial-CRUD-honesty decision in the design).
type projectResource struct {
	providerData *ProviderData
}

type projectResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Region         types.String `tfsdk:"region"`
}

var (
	_ resource.Resource                = (*projectResource)(nil)
	_ resource.ResourceWithConfigure   = (*projectResource)(nil)
	_ resource.ResourceWithImportState = (*projectResource)(nil)
)

// NewProjectResource returns the sazabi_project resource factory.
func NewProjectResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sazabi project. The public API supports create and read only: " +
			"projects cannot be renamed or deleted via Terraform, so every change requires replacement, " +
			"and destroy only removes the project from state (the project itself keeps running).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Project ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Organization the project belongs to. Defaults to the provider-level organization_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Project name (≤100 characters; letters, numbers, spaces, hyphens, underscores). Changing it requires replacement — the API has no rename.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "AWS region where the project's data lives. Defaults to us-west-2 server-side.",
				Validators: []validator.String{
					stringvalidator.OneOf(client.ProjectRegions...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID := plan.OrganizationID.ValueString()
	if organizationID == "" {
		organizationID = r.providerData.OrganizationID
	}

	project, err := r.providerData.Client.CreateProject(ctx, client.CreateProjectInput{
		OrganizationID: organizationID,
		Name:           plan.Name.ValueString(),
		Region:         plan.Region.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectModelFromAPI(project))...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.providerData.Client.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectModelFromAPI(project))...)
}

func (r *projectResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Unreachable: every mutable attribute is marked RequiresReplace because
	// the public API has no project update endpoint.
	resp.Diagnostics.AddError(
		"Projects cannot be updated",
		"The Sazabi public API has no project update endpoint; all changes require replacement. "+
			"This plan reaching Update is a bug in the provider.",
	)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Project not deleted from Sazabi",
		fmt.Sprintf(
			"The Sazabi public API has no project delete endpoint. Project %q (%s) was removed from Terraform state only and keeps running in Sazabi. "+
				"Delete it from the dashboard if you want it gone.",
			state.Name.ValueString(), state.ID.ValueString(),
		),
	)
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func projectModelFromAPI(project *client.Project) projectResourceModel {
	return projectResourceModel{
		ID:             types.StringValue(project.ID),
		OrganizationID: types.StringValue(project.OrganizationID),
		Name:           types.StringValue(project.Name),
		Region:         types.StringValue(project.Region),
	}
}
