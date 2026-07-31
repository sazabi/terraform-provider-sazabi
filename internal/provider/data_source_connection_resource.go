package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sazabi/terraform-provider-sazabi/internal/client"
)

// dataSourceConnectionResource implements sazabi_data_source_connection.
//
// The API supports create, read, and delete only — there is no update
// endpoint for connection metadata, so every attribute requires replacement
// (credential rotation is disconnect-and-recreate, as the design records in
// Known Limitations). Connection metadata is write-only server-side: it is
// sent on create, encrypted at rest, and never returned by any GET, so the
// provider carries it in state without drift-checking it. metadata values
// are Terraform strings (the design's map(string) resolution of the dynamic
// schema open question); per-type field validation happens server-side at
// create time.
type dataSourceConnectionResource struct {
	providerData *ProviderData
}

type dataSourceConnectionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	DataSourceType types.String `tfsdk:"data_source_type"`
	Metadata       types.Map    `tfsdk:"metadata"`
	DisplayName    types.String `tfsdk:"display_name"`
	PublicKey      types.String `tfsdk:"public_key"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

var (
	_ resource.Resource                = (*dataSourceConnectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*dataSourceConnectionResource)(nil)
	_ resource.ResourceWithImportState = (*dataSourceConnectionResource)(nil)
)

// NewDataSourceConnectionResource returns the sazabi_data_source_connection resource factory.
func NewDataSourceConnectionResource() resource.Resource {
	return &dataSourceConnectionResource{}
}

func (r *dataSourceConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_source_connection"
}

func (r *dataSourceConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sazabi data source connection. The API has no update endpoint, so every change requires " +
			"replacement — credential rotation is disconnect-and-recreate. Connection metadata is write-only: " +
			"it is validated and encrypted at creation and never returned by the API, so it cannot be drift-checked.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Connection ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Project the connection belongs to. Defaults to the API key's project scope.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"data_source_type": schema.StringAttribute{
				Required:    true,
				Description: "Data source type identifier (e.g. vercel, datadog). Valid values come from GET /data-sources/types; invalid types fail at create time.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"metadata": schema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Sensitive:   true,
				Description: "Type-specific credentials and settings; the field set varies per data_source_type (see GET /data-sources/types). Marked sensitive as a whole because it typically carries tokens.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Optional:    true,
				Description: "Human-readable connection name. Changing it requires replacement — the API has no update.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"public_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Public API key auto-generated for the connection. Returned only at creation; empty for imported connections.",
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

func (r *dataSourceConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dataSourceConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dataSourceConnectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	metadata := map[string]string{}
	resp.Diagnostics.Append(plan.Metadata.ElementsAs(ctx, &metadata, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.providerData.Client.CreateDataSourceConnection(ctx, client.CreateDataSourceConnectionInput{
		ProjectID:      plan.ProjectID.ValueString(),
		DataSourceType: plan.DataSourceType.ValueString(),
		Metadata:       metadata,
		DisplayName:    plan.DisplayName.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create data source connection", err.Error())
		return
	}

	connection, err := r.providerData.Client.GetDataSourceConnection(ctx, created.ConnectionID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read created data source connection", err.Error())
		return
	}

	model := plan
	model.ID = types.StringValue(connection.ID)
	model.ProjectID = types.StringValue(connection.ProjectID)
	model.DataSourceType = types.StringValue(connection.DataSourceType)
	model.DisplayName = types.StringPointerValue(connection.DisplayName)
	model.PublicKey = types.StringValue(created.PublicKey)
	model.CreatedAt = types.StringValue(connection.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *dataSourceConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dataSourceConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connection, err := r.providerData.Client.GetDataSourceConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read data source connection", err.Error())
		return
	}

	// metadata and public_key are never returned by the API; carry the
	// stored values forward.
	state.ProjectID = types.StringValue(connection.ProjectID)
	state.DataSourceType = types.StringValue(connection.DataSourceType)
	state.DisplayName = types.StringPointerValue(connection.DisplayName)
	state.CreatedAt = types.StringValue(connection.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dataSourceConnectionResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Unreachable: every attribute is marked RequiresReplace because the
	// public API has no connection update endpoint.
	resp.Diagnostics.AddError(
		"Data source connections cannot be updated",
		"The Sazabi public API has no connection update endpoint; all changes require replacement. "+
			"This plan reaching Update is a bug in the provider.",
	)
}

func (r *dataSourceConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dataSourceConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.providerData.Client.DisconnectDataSourceConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to disconnect data source connection", err.Error())
		return
	}
	if result.ConnectionTeardownError != nil {
		resp.Diagnostics.AddWarning(
			"Data source connection disconnected with a teardown error",
			fmt.Sprintf(
				"The connection was soft-deleted in Sazabi, but remote cleanup reported: %s. "+
					"Vendor-side resources may need manual cleanup.",
				*result.ConnectionTeardownError,
			),
		)
	}
}

func (r *dataSourceConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// metadata and public_key are unrecoverable for imported connections;
	// record them as empty so plans converge (changing metadata afterwards
	// forces replacement anyway).
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("metadata"), map[string]string{})...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("public_key"), "")...)
}
