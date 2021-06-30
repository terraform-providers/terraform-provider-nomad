package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestDataSourceACLToken_Basic(t *testing.T) {
	resourceName := "data.nomad_acl_token.test"

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceACLTokenConfig,
				Check: resource.ComposeTestCheckFunc(
					testDataSourceACLTokenExists(resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "secret_id"),
					resource.TestCheckResourceAttrSet(resourceName, "accessor_id"),
					resource.TestCheckResourceAttrSet(resourceName, "create_time"),
					resource.TestCheckResourceAttr(resourceName, "name", "Terraform Test Token"),
					resource.TestCheckResourceAttr(resourceName, "type", "client"),
					resource.TestCheckResourceAttr(resourceName, "policies.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "global", "false"),
				),
			},
		},
	})
}

func testDataSourceACLTokenExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("root module has no resource called %s", name)
		}
		return nil
	}
}

const testDataSourceACLTokenConfig = `
resource "nomad_acl_token" "test" {
  name = "Terraform Test Token"
  type = "client"
  policies = ["dev", "qa"]
  global = false
}

data "nomad_acl_token" "test" {
		accessor_id = "${nomad_acl_token.test.accessor_id}"
}
`
