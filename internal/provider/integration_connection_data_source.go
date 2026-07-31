package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// integrationConnectionDataSource implements data.sazabi_integration_connection.
//
// Integrations are provisioned through interactive OAuth/app-installation
// flows with no Terraform-native analog, so the public API — and therefore
// the provider — exposes them read-only (the design's OAuth non-goal).
type integrationConnectionDataSource struct {
	providerData *ProviderData
}

type integrationConnectionDataSourceModel struct {
	ConnectionID   types.String `tfsdk:"connection_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Provider       types.String `tfsdk:"provider_id"`
	DisplayName    types.String `tfsdk:"display_name"`
	Status         types.String `tfsdk:"status"`
	IsActive       types.Bool   `tfsdk:"is_active"`
	NeedsAttention types.Bool   `tfsdk:"needs_attention"`
	HealthStatus   types.String `tfsdk:"health_status"`
	ConnectedBy    types.String `tfsdk:"connected_by"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

var (
	_ datasource.DataSource              = (*integrationConnectionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*integrationConnectionDataSource)(nil)
)

// NewIntegrationConnectionDataSource returns the sazabi_integration_connection data source factory.
func NewIntegrationConnectionDataSource() datasource.DataSource {
	return &integrationConnectionDataSource{}
}

func (d *integrationConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_connection"
}

func (d *integrationConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads one integration connection. Integrations connect via interactive OAuth or app " +
			"installation, which Terraform cannot drive, so this surface is read-only by design.",
		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				Required:    true,
				Description: "Integration connection ID (UUID).",
			},
			"organization_id": schema.StringAttribute{
				Optional:    true,
				Description: "Organization the connection belongs to. Defaults to the provider-level organization_id.",
			},
			"provider_id": schema.StringAttribute{
				Computed:    true,
				Description: "Integration provider identifier (e.g. slack, github).",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable connection name.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Connection status. An error status means connected-but-broken, not absent.",
			},
			"is_active": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this connection counts as connected (including error-status connections needing attention).",
			},
			"needs_attention": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the connection should surface a reconnect prompt.",
			},
			"health_status": schema.StringAttribute{
				Computed:    true,
				Description: "Result of the most recent health sweep.",
			},
			"connected_by": schema.StringAttribute{
				Computed:    true,
				Description: "Who connected the integration, if recorded.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (RFC 3339).",
			},
		},
	}
}

func (d *integrationConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.providerData = providerData
}

func (d *integrationConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config integrationConnectionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationID := config.OrganizationID.ValueString()
	if organizationID == "" {
		organizationID = d.providerData.OrganizationID
	}

	connection, err := d.providerData.Client.GetIntegrationConnection(ctx, config.ConnectionID.ValueString(), organizationID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read integration connection", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, integrationConnectionDataSourceModel{
		ConnectionID:   types.StringValue(connection.ID),
		OrganizationID: config.OrganizationID,
		Provider:       types.StringValue(connection.Provider),
		DisplayName:    types.StringPointerValue(connection.DisplayName),
		Status:         types.StringValue(connection.Status),
		IsActive:       types.BoolValue(connection.IsActive),
		NeedsAttention: types.BoolValue(connection.NeedsAttention),
		HealthStatus:   types.StringValue(connection.HealthStatus),
		ConnectedBy:    types.StringPointerValue(connection.ConnectedBy),
		CreatedAt:      types.StringValue(connection.CreatedAt),
	})...)
}
