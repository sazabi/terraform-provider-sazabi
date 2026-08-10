package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestComponentResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewComponentResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"id", "project_id", "name", "description"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}

// TestAccComponentResource covers register, description update via
// re-register, deregister on destroy, and import. Requires TF_ACC plus a
// sandbox org (SAZABI_API_KEY, SAZABI_ORGANIZATION_ID) and an existing
// project (SAZABI_TEST_PROJECT_ID) since projects cannot be deleted.
func TestAccComponentResource(t *testing.T) {
	projectID := os.Getenv("SAZABI_TEST_PROJECT_ID")
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	config := func(description string) string {
		return fmt.Sprintf(`
resource "sazabi_component" "test" {
  project_id  = %q
  name        = %q
  description = %q
}
`, projectID, name, description)
	}

	testresource.Test(t, testresource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if projectID == "" {
				t.Fatal("SAZABI_TEST_PROJECT_ID must be set for component acceptance tests")
			}
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: config("Managed by acceptance tests"),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttrSet("sazabi_component.test", "id"),
					testresource.TestCheckResourceAttr("sazabi_component.test", "name", name),
					testresource.TestCheckResourceAttr("sazabi_component.test", "description", "Managed by acceptance tests"),
				),
			},
			{
				Config: config("Updated description"),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttr("sazabi_component.test", "description", "Updated description"),
				),
			},
			{
				ResourceName:      "sazabi_component.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
