// Package provider implements the sazabi Terraform provider.
//
// Every resource and data source maps 1:1 to Sazabi public API operations
// defined in packages/public-api-contracts (sazabi/monorepo) — the provider
// never models capability the API does not have. See the design doc:
// docs/design/infrastructure/terraform-provider-v2/design.md.
package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sazabi/terraform-provider-sazabi/internal/client"
)

// Environment variable fallbacks, matching the CLI's auth precedence:
// explicit provider block first, then environment, then a clear failure.
const (
	envAPIKey         = "SAZABI_API_KEY"
	envOrganizationID = "SAZABI_ORGANIZATION_ID"
	envBaseURL        = "SAZABI_API_BASE_URL"
)

// SazabiProvider is the provider implementation.
type SazabiProvider struct {
	version string
}

// ProviderData is passed to every resource and data source via Configure.
type ProviderData struct {
	Client *client.Client
	// OrganizationID is the provider-level default organization, used when a
	// resource does not set its own organization_id. May be empty: org-wide
	// secret keys imply their organization server-side.
	OrganizationID string
}

// SazabiProviderModel maps the provider block schema.
type SazabiProviderModel struct {
	APIKey         types.String `tfsdk:"api_key"`
	OrganizationID types.String `tfsdk:"organization_id"`
	BaseURL        types.String `tfsdk:"base_url"`
}

var _ provider.Provider = (*SazabiProvider)(nil)

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SazabiProvider{version: version}
	}
}

func (p *SazabiProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "sazabi"
	resp.Version = p.version
}

func (p *SazabiProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Declare Sazabi platform configuration as code, backed by the Sazabi public API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Sazabi secret API key (sazabi_secret_...). Falls back to the " + envAPIKey + " environment variable.",
			},
			"organization_id": schema.StringAttribute{
				Optional:    true,
				Description: "Default organization for resources that do not set their own. Falls back to the " + envOrganizationID + " environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "API base URL override for staging or region targeting, without the /v1 path (default " + client.DefaultBaseURL + "). Falls back to the " + envBaseURL + " environment variable.",
			},
		},
	}
}

func (p *SazabiProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config SazabiProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.APIKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Sazabi API key",
			"The provider cannot connect to the Sazabi API with an unknown api_key value. "+
				"Set it statically, or apply the resource producing it first.",
		)
		return
	}

	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv(envAPIKey)
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Sazabi API key",
			"Set api_key in the provider block or export "+envAPIKey+". "+
				"Create a secret key in the Sazabi dashboard under Settings → API Keys.",
		)
		return
	}

	organizationID := config.OrganizationID.ValueString()
	if organizationID == "" {
		organizationID = os.Getenv(envOrganizationID)
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv(envBaseURL)
	}

	apiClient, err := client.New(client.Config{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		UserAgent: fmt.Sprintf("terraform-provider-sazabi/%s", p.version),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Sazabi API client", err.Error())
		return
	}

	data := &ProviderData{Client: apiClient, OrganizationID: organizationID}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *SazabiProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewComponentResource,
		NewScriptResource,
		NewAPIKeyResource,
		NewDataSourceConnectionResource,
		NewDataSourceStreamResource,
		NewAutomationResource,
		NewPublicKeyLogForwardingResource,
	}
}

func (p *SazabiProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewIntegrationConnectionDataSource,
		NewMcpConnectorDataSource,
	}
}
