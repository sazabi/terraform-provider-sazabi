package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mcpConnectorDataSource implements data.sazabi_mcp_connector. The MCP
// connector endpoints are explicitly documented read-only in the contract
// source; connectors are configured through the dashboard.
type mcpConnectorDataSource struct {
	providerData *ProviderData
}

type mcpConnectorDataSourceModel struct {
	ConnectionID     types.String `tfsdk:"connection_id"`
	ProjectID        types.String `tfsdk:"project_id"`
	ConnectionKey    types.String `tfsdk:"connection_key"`
	ProviderID       types.String `tfsdk:"provider_id"`
	DisplayName      types.String `tfsdk:"display_name"`
	Source           types.String `tfsdk:"source"`
	InstallStatus    types.String `tfsdk:"install_status"`
	AuthMode         types.String `tfsdk:"auth_mode"`
	Transport        types.String `tfsdk:"transport"`
	ServerURL        types.String `tfsdk:"server_url"`
	ReadOnly         types.Bool   `tfsdk:"read_only"`
	EnabledToolCount types.Int64  `tfsdk:"enabled_tool_count"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

var (
	_ datasource.DataSource              = (*mcpConnectorDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*mcpConnectorDataSource)(nil)
)

// NewMcpConnectorDataSource returns the sazabi_mcp_connector data source factory.
func NewMcpConnectorDataSource() datasource.DataSource {
	return &mcpConnectorDataSource{}
}

func (d *mcpConnectorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_connector"
}

func (d *mcpConnectorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads one MCP connector. MCP connectors are provisioned through the dashboard " +
			"(often via interactive OAuth), so this surface is read-only by design.",
		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				Required:    true,
				Description: "MCP connector connection ID (UUID).",
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "Project the connector belongs to. Defaults to the API key's project scope.",
			},
			"connection_key": schema.StringAttribute{
				Computed:    true,
				Description: "Stable key used to reference this connector in tool calls.",
			},
			"provider_id": schema.StringAttribute{
				Computed:    true,
				Description: "Provider identifier (e.g. linear).",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "Human-readable connector name.",
			},
			"source": schema.StringAttribute{
				Computed:    true,
				Description: "Whether the connector is a built-in preset or a custom server.",
			},
			"install_status": schema.StringAttribute{
				Computed:    true,
				Description: "Current connection lifecycle status.",
			},
			"auth_mode": schema.StringAttribute{
				Computed:    true,
				Description: "Authentication mode.",
			},
			"transport": schema.StringAttribute{
				Computed:    true,
				Description: "Transport protocol.",
			},
			"server_url": schema.StringAttribute{
				Computed:    true,
				Description: "MCP server URL.",
			},
			"read_only": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the connector is restricted to read-only tools.",
			},
			"enabled_tool_count": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of tools enabled for this connector.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp (RFC 3339).",
			},
		},
	}
}

func (d *mcpConnectorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *mcpConnectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config mcpConnectorDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connector, err := d.providerData.Client.GetMcpConnector(ctx, config.ConnectionID.ValueString(), config.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read MCP connector", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, mcpConnectorDataSourceModel{
		ConnectionID:     types.StringValue(connector.ConnectionID),
		ProjectID:        config.ProjectID,
		ConnectionKey:    types.StringValue(connector.ConnectionKey),
		ProviderID:       types.StringValue(connector.ProviderID),
		DisplayName:      types.StringValue(connector.DisplayName),
		Source:           types.StringValue(connector.Source),
		InstallStatus:    types.StringValue(connector.InstallStatus),
		AuthMode:         types.StringValue(connector.AuthMode),
		Transport:        types.StringValue(connector.Transport),
		ServerURL:        types.StringValue(connector.ServerURL),
		ReadOnly:         types.BoolValue(connector.ReadOnly),
		EnabledToolCount: types.Int64Value(int64(connector.EnabledToolCount)),
		CreatedAt:        types.StringValue(connector.CreatedAt),
	})...)
}
