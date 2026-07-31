package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestAutomationResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewAutomationResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"id", "automation_id", "project_id", "enabled", "name"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}

func TestPublicKeyLogForwardingResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewPublicKeyLogForwardingResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	if !schemaResp.Schema.Attributes["value"].IsSensitive() {
		t.Error("value must be marked sensitive")
	}
	for _, attr := range []string{"id", "project_id", "name", "value", "created_at"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}
