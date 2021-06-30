package nomad

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceACLPolicy_import(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLPolicy_initialConfig(name),
				Check:  testResourceACLPolicy_initialCheck(name),
			},
			{
				ResourceName:      "nomad_acl_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},

		CheckDestroy: testResourceACLPolicy_checkDestroy(name),
	})
}

func TestResourceACLPolicy_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLPolicy_initialConfig(name),
				Check:  testResourceACLPolicy_initialCheck(name),
			},
		},

		CheckDestroy: testResourceACLPolicy_checkDestroy(name),
	})
}

func TestResourceACLPolicy_refresh(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLPolicy_initialConfig(name),
				Check:  testResourceACLPolicy_initialCheck(name),
			},

			// This should successfully cause the policy to be recreated,
			// testing the Exists function.
			{
				PreConfig: testResourceACLPolicy_delete(t, name),
				Config:    testResourceACLPolicy_initialConfig(name),
			},
		},
	})
}

func TestResourceACLPolicy_nameChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	newName := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLPolicy_initialConfig(name),
				Check:  testResourceACLPolicy_initialCheck(name),
			},

			// Change our name
			{
				Config: testResourceACLPolicy_updateConfig(newName),
				Check:  testResourceACLPolicy_updateCheck(newName),
			},
		},
	})
}

func TestResourceACLPolicy_update(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLPolicy_initialConfig(name),
				Check:  testResourceACLPolicy_initialCheck(name),
			},
			{
				Config: testResourceACLPolicy_updateConfig(name),
				Check:  testResourceACLPolicy_updateCheck(name),
			},
		},
	})
}

func testResourceACLPolicy_initialConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_acl_policy" "test" {
  name = "%s"
  description = "A Terraform acctest ACL policy"
  rules_hcl = <<EOT
namespace "default" {
  policy = "read"
  capabilities = ["submit-job"]
}
EOT
}
`, name)
}

func testResourceACLPolicy_initialCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			description = "A Terraform acctest ACL policy"
			rules_hcl   = `namespace "default" {
  policy = "read"
  capabilities = ["submit-job"]
}
`
		)
		resourceState := s.Modules[0].Resources["nomad_acl_policy.test"]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}

		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}

		if instanceState.ID != name {
			return fmt.Errorf("expected ID to be %q, got %q", name, instanceState.ID)
		}

		if instanceState.Attributes["name"] != name {
			return fmt.Errorf("expected name to be %q, is %q in state", name, instanceState.Attributes["name"])
		}

		if instanceState.Attributes["description"] != description {
			return fmt.Errorf("expected description to be %q, is %q in state", description, instanceState.Attributes["description"])
		}

		if instanceState.Attributes["rules_hcl"] != rules_hcl {
			return fmt.Errorf("expected rules_hcl to be %q, is %q in state", rules_hcl, instanceState.Attributes["rules_hcl"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.ACLPolicies().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back policy %q: %s", name, err)
		}

		if policy.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, policy.Name)
		}
		if policy.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, policy.Description)
		}
		if policy.Rules != rules_hcl {
			return fmt.Errorf("expected rules_hcl to be %q, is %q in API", rules_hcl, policy.Rules)
		}

		return nil
	}
}

func testResourceACLPolicy_checkExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.ACLPolicies().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back policy: %s", err)
		}
		if policy == nil {
			return fmt.Errorf("no policy returned for %q", name)
		}

		return nil
	}
}

func testResourceACLPolicy_checkDestroy(name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.ACLPolicies().Info(name, nil)
		if err != nil && strings.Contains(err.Error(), "404") || policy == nil {
			return nil
		}
		return fmt.Errorf("Policy %q has not been deleted.", name)
	}
}

func testResourceACLPolicy_delete(t *testing.T, name string) func() {
	return func() {
		client := testProvider.Meta().(ProviderConfig).Client
		_, err := client.ACLPolicies().Delete(name, nil)
		if err != nil {
			t.Fatalf("error deleting ACL policy: %s", err)
		}
	}
}

func testResourceACLPolicy_updateConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_acl_policy" "test" {
  name = "%s"
  description = "An updated Terraform acctest ACL policy"
  rules_hcl = <<EOT
namespace "default" {
  policy = "read"
  capabilities = ["submit-job", "read-job"]
}
EOT
}
`, name)
}

func testResourceACLPolicy_updateCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			description = "An updated Terraform acctest ACL policy"
			rules_hcl   = `namespace "default" {
  policy = "read"
  capabilities = ["submit-job", "read-job"]
}
`
		)
		resourceState := s.Modules[0].Resources["nomad_acl_policy.test"]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}

		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}

		if instanceState.ID != name {
			return fmt.Errorf("expected ID to be %q, got %q", name, instanceState.ID)
		}

		if instanceState.Attributes["name"] != name {
			return fmt.Errorf("expected name to be %q, is %q in state", name, instanceState.Attributes["name"])
		}

		if instanceState.Attributes["description"] != description {
			return fmt.Errorf("expected description to be %q, is %q in state", description, instanceState.Attributes["description"])
		}

		if instanceState.Attributes["rules_hcl"] != rules_hcl {
			return fmt.Errorf("expected rules_hcl to be %q, is %q in state", rules_hcl, instanceState.Attributes["rules_hcl"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.ACLPolicies().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back policy %q: %s", name, err)
		}

		if policy.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, policy.Name)
		}
		if policy.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, policy.Description)
		}
		if policy.Rules != rules_hcl {
			return fmt.Errorf("expected rules_hcl to be %q, is %q in API", rules_hcl, policy.Rules)
		}

		return nil
	}
}
