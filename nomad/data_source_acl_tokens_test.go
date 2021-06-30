package nomad

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataSourceACLTokens_Basic(t *testing.T) {
	resourceName := "data.nomad_acl_tokens.test"

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceACLTokensConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "prefix"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.#", "1"),
					resource.TestCheckResourceAttrSet(resourceName, "acl_tokens.0.accessor_id"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.name", "Terraform Test Token"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.type", "client"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.policies.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.policies.0", "qa"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.policies.1", "dev"),
					resource.TestCheckResourceAttr(resourceName, "acl_tokens.0.global", "false"),
					resource.TestCheckResourceAttrSet(resourceName, "acl_tokens.0.create_time"),
				),
			},
		},
	})
}

const testDataSourceACLTokensConfig = `
resource "nomad_acl_token" "test" {
	name = "Terraform Test Token"
	type = "client"
	policies = ["dev", "qa"]
}

data "nomad_acl_tokens" "test" {
	prefix = split("-", nomad_acl_token.test.accessor_id)[0]
}
`
