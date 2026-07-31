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

// dataSourceStreamResource implements sazabi_data_source_stream.
//
// The API supports create, read, and delete — no update — so all attributes
// require replacement. Provisioning is asynchronous; the volatile status and
// errorMessage fields are intentionally not tracked as state. The configured
// config map is authoritative in state and not drift-checked against the
// server copy, which the backend may normalize or extend during provisioning.
type dataSourceStreamResource struct {
	providerData *ProviderData
}

type dataSourceStreamResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ConnectionID types.String `tfsdk:"connection_id"`
	DisplayName  types.String `tfsdk:"display_name"`
	Config       types.Map    `tfsdk:"config"`
	PublicKey    types.String `tfsdk:"public_key"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

var (
	_ resource.Resource                = (*dataSourceStreamResource)(nil)
	_ resource.ResourceWithConfigure   = (*dataSourceStreamResource)(nil)
	_ resource.ResourceWithImportState = (*dataSourceStreamResource)(nil)
)

// NewDataSourceStreamResource returns the sazabi_data_source_stream resource factory.
func NewDataSourceStreamResource() resource.Resource {
	return &dataSourceStreamResource{}
}

func (r *dataSourceStreamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_source_stream"
}

func (r *dataSourceStreamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A stream under a Sazabi data source connection. The API has no update endpoint, so every " +
			"change requires replacement. Provisioning is asynchronous; poll status via the API or dashboard.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Stream ID (UUID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connection_id": schema.StringAttribute{
				Required:    true,
				Description: "Data source connection the stream belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Required:    true,
				Description: "Resource name, e.g. a service or app name. Changing it requires replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"config": schema.MapAttribute{
				ElementType: types.StringType,
				Required:    true,
				Description: "Type-specific stream configuration. The configured value is authoritative; the server copy may be normalized during provisioning and is not drift-checked.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"public_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Per-stream public API key, minted only by sources that send to a Sazabi endpoint. Returned only at creation; empty otherwise.",
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

func (r *dataSourceStreamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dataSourceStreamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dataSourceStreamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config := map[string]string{}
	resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &config, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.providerData.Client.CreateDataSourceStream(ctx, plan.ConnectionID.ValueString(), client.CreateDataSourceStreamInput{
		DisplayName: plan.DisplayName.ValueString(),
		Config:      config,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create data source stream", err.Error())
		return
	}

	stream, err := r.providerData.Client.GetDataSourceStream(ctx, created.StreamID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read created data source stream", err.Error())
		return
	}

	model := plan
	model.ID = types.StringValue(stream.ID)
	model.DisplayName = types.StringValue(stream.DisplayName)
	model.PublicKey = types.StringValue(created.PublicKey)
	model.CreatedAt = types.StringValue(stream.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *dataSourceStreamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dataSourceStreamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stream, err := r.providerData.Client.GetDataSourceStream(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read data source stream", err.Error())
		return
	}

	// config and public_key stay authoritative from state; the server may
	// normalize config during provisioning and never re-returns the key.
	if stream.ConnectionID != nil {
		state.ConnectionID = types.StringValue(*stream.ConnectionID)
	}
	state.DisplayName = types.StringValue(stream.DisplayName)
	state.CreatedAt = types.StringValue(stream.CreatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *dataSourceStreamResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Unreachable: every attribute is marked RequiresReplace because the
	// public API has no stream update endpoint.
	resp.Diagnostics.AddError(
		"Data source streams cannot be updated",
		"The Sazabi public API has no stream update endpoint; all changes require replacement. "+
			"This plan reaching Update is a bug in the provider.",
	)
}

func (r *dataSourceStreamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dataSourceStreamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.providerData.Client.DeleteDataSourceStream(ctx, state.ID.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete data source stream", err.Error())
	}
}

func (r *dataSourceStreamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// config and public_key are unrecoverable in full fidelity on import:
	// record config empty (changing it forces replacement) and the key empty.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("config"), map[string]string{})...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("public_key"), "")...)
}
