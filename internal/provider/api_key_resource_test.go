package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAPIKeyResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewAPIKeyResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	if !schemaResp.Schema.Attributes["value"].IsSensitive() {
		t.Error("value must be marked sensitive")
	}
	for _, attr := range []string{"id", "name", "project_id", "expires_at", "value", "created_at"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}

// TestAccAPIKeyResource covers the full CRUD cycle: create (with the
// plaintext value returned once), rename in place, import (value
// unrecoverable), and delete.
func TestAccAPIKeyResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	testresource.Test(t, testresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sazabi_api_key" "test" {
  name = %q
}
`, name),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttrSet("sazabi_api_key.test", "id"),
					testresource.TestCheckResourceAttr("sazabi_api_key.test", "name", name),
					testresource.TestCheckResourceAttrSet("sazabi_api_key.test", "value"),
					testresource.TestCheckResourceAttrSet("sazabi_api_key.test", "created_at"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "sazabi_api_key" "test" {
  name = %q
}
`, name+"-renamed"),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("sazabi_api_key.test", "name", name+"-renamed"),
				),
			},
			{
				ResourceName:      "sazabi_api_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The plaintext value is returned only at creation and can
				// never be re-fetched, so the imported state differs here.
				ImportStateVerifyIgnore: []string{"value"},
			},
		},
	})
}
