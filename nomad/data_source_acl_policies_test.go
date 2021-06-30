package nomad

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceNomadAclPolicies_Basic(t *testing.T) {
	dataSourceName := "data.nomad_acl_policies.test"
	numPolicies := 2

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				PreConfig: testAccCreateNomadAclPolicies(t, numPolicies),
				Config:    testAccNomadAclPoliciesConfig("non-existent"),
			},
			{
				Config: testAccNomadAclPoliciesConfig("tf-acc-test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "2"),
					resource.TestMatchResourceAttr(dataSourceName, "policies.0.name", regexp.MustCompile("tf-acc-test")),
					resource.TestMatchResourceAttr(dataSourceName, "policies.0.description", regexp.MustCompile("Terraform ACL Policy tf-acc-test")),
					resource.TestMatchResourceAttr(dataSourceName, "policies.1.name", regexp.MustCompile("tf-acc-test")),
					resource.TestMatchResourceAttr(dataSourceName, "policies.1.description", regexp.MustCompile("Terraform ACL Policy tf-acc-test")),
				),
			},
		},
	})
	// ACL Policy Resource Clean-up
	err := sweepACLPolicies()
	if err != nil {
		t.Error(err)
	}
}

func testAccNomadAclPoliciesConfig(prefix string) string {
	return fmt.Sprintf(`
data "nomad_acl_policies" "test" {
	prefix = "%s"
}
`, prefix)
}

func testAccCreateNomadAclPolicies(t *testing.T, n int) func() {
	return func() {
		client := testProvider.Meta().(ProviderConfig).Client
		for i := 0; i < n; i++ {
			rName := acctest.RandomWithPrefix("tf-acc-test")
			policy := api.ACLPolicy{
				Name:        rName,
				Description: fmt.Sprintf("Terraform ACL Policy %s", rName),
				Rules: `
				namespace "default" {
				  policy = "write"
				}
				`,
			}
			_, err := client.ACLPolicies().Upsert(&policy, nil)
			if err != nil {
				t.Fatalf("error inserting ACLPolicy %q: %s", policy.Name, err.Error())
			}
		}
	}
}

func sweepACLPolicies() error {
	client := testProvider.Meta().(ProviderConfig).Client
	policies, _, err := client.ACLPolicies().List(&api.QueryOptions{})
	if err != nil {
		return err
	}
	for _, policy := range policies {
		_, err := client.ACLPolicies().Delete(policy.Name, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
