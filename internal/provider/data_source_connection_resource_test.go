package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestDataSourceConnectionResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewDataSourceConnectionResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"id", "project_id", "data_source_type", "metadata", "display_name", "public_key", "created_at"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
	if !schemaResp.Schema.Attributes["metadata"].IsSensitive() {
		t.Error("metadata must be marked sensitive (it carries credentials)")
	}
	if !schemaResp.Schema.Attributes["public_key"].IsSensitive() {
		t.Error("public_key must be marked sensitive")
	}
}

func TestDataSourceStreamResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewDataSourceStreamResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"id", "connection_id", "display_name", "config", "public_key", "created_at"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
	if !schemaResp.Schema.Attributes["public_key"].IsSensitive() {
		t.Error("public_key must be marked sensitive")
	}
}
