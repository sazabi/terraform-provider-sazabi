package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories instantiates the provider for acceptance
// tests. Acceptance tests require TF_ACC=1, SAZABI_API_KEY, and
// SAZABI_ORGANIZATION_ID pointing at a sandbox organization.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"sazabi": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	p := New("test")()

	schemaResp := &provider.SchemaResponse{}
	p.Schema(ctx, provider.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}

	for _, attr := range []string{"api_key", "organization_id", "base_url"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected provider schema attribute %q", attr)
		}
	}
	if !schemaResp.Schema.Attributes["api_key"].IsSensitive() {
		t.Error("api_key must be marked sensitive")
	}
}

func TestProviderMetadata(t *testing.T) {
	ctx := context.Background()
	p := New("1.2.3")()

	metadataResp := &provider.MetadataResponse{}
	p.Metadata(ctx, provider.MetadataRequest{}, metadataResp)
	if metadataResp.TypeName != "sazabi" {
		t.Errorf("expected type name sazabi, got %q", metadataResp.TypeName)
	}
	if metadataResp.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %q", metadataResp.Version)
	}
}
