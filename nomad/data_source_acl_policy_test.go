package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceNomadAclPolicy_Basic(t *testing.T) {
	policyName := acctest.RandomWithPrefix("test-policy")
	resourceName := "data.nomad_acl_policy.test"
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testProviders,
		CheckDestroy: testResourceACLPolicy_checkDestroy(policyName),
		Steps: []resource.TestStep{
			{
				Config: testAccNomadAclPolicyConfig(policyName),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourceNomadAclPolicyExists(resourceName),
					resource.TestCheckResourceAttr(
						resourceName, "name", policyName),
					resource.TestCheckResourceAttr(
						resourceName, "description", "Test ACL Policy"),
					resource.TestCheckResourceAttrSet(resourceName, "rules"),
				),
			},
		},
	})
}

func testAccNomadAclPolicyConfig(name string) string {
	return `
resource "nomad_acl_policy" "test" {
	name        = "` + name + `"
	description = "Test ACL Policy"
    rules_hcl   = <<EOT
namespace "default" {
  policy = "write"
}
EOT
}

data "nomad_acl_policy" "test" {
	name = "${nomad_acl_policy.test.id}"
}
`
}

func testAccDataSourceNomadAclPolicyExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("ACL Policy not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No ACL Policy Name is set")
		}

		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client

		name := rs.Primary.ID

		// Try to get the Policy
		policy, _, err := client.ACLPolicies().Info(name, &api.QueryOptions{})
		if err != nil {
			return err
		}

		if policy.Name != name {
			return fmt.Errorf("ACL Policy not found")
		}

		return nil
	}
}
