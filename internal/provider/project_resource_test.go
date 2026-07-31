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

func TestProjectResourceSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	NewProjectResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	if err := schemaResp.Schema.ValidateImplementation(ctx); err != nil {
		t.Fatalf("schema implementation invalid: %v", err)
	}

	for _, attr := range []string{"id", "organization_id", "name", "region"} {
		if _, ok := schemaResp.Schema.Attributes[attr]; !ok {
			t.Errorf("expected attribute %q", attr)
		}
	}
}

// testAccPreCheck skips acceptance tests unless the sandbox-org environment
// is configured. TF_ACC gating is handled by terraform-plugin-testing itself.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"SAZABI_API_KEY", "SAZABI_ORGANIZATION_ID"} {
		if os.Getenv(env) == "" {
			t.Fatalf("%s must be set for acceptance tests (use a sandbox organization)", env)
		}
	}
}

// TestAccProjectResource exercises create, read, and import against a real
// sandbox organization. Projects cannot be deleted via the API, so destroy
// only removes state — expect the sandbox org to accumulate test projects.
func TestAccProjectResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))

	testresource.Test(t, testresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sazabi_project" "test" {
  name = %q
}
`, name),
				Check: testresource.ComposeAggregateTestCheckFunc(
					testresource.TestCheckResourceAttrSet("sazabi_project.test", "id"),
					testresource.TestCheckResourceAttrSet("sazabi_project.test", "organization_id"),
					testresource.TestCheckResourceAttr("sazabi_project.test", "name", name),
					testresource.TestCheckResourceAttr("sazabi_project.test", "region", "us-west-2"),
				),
			},
			{
				ResourceName:      "sazabi_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
