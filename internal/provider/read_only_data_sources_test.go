package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestIntegrationConnectionDataSourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &datasource.SchemaResponse{}
	NewIntegrationConnectionDataSource().Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"connection_id", "organization_id", "provider_id", "status", "is_active", "needs_attention", "health_status"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}

func TestMcpConnectorDataSourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &datasource.SchemaResponse{}
	NewMcpConnectorDataSource().Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"connection_id", "project_id", "connection_key", "provider_id", "install_status", "transport", "server_url", "read_only", "enabled_tool_count"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}
