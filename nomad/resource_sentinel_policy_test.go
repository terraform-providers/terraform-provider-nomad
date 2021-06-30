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

func TestResourceSentinelPolicy_import(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	description := "A terraform acctest policy"
	policy := `main = rule { true }`
	scope := "submit-job"
	enforcementLevel := "advisory"
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceSentinelPolicy_config(name, description, policy, scope, enforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(name, description, policy, scope, enforcementLevel),
			},
			{
				ResourceName:      "nomad_sentinel_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},

		CheckDestroy: testResourceSentinelPolicy_checkDestroy(name),
	})
}

func TestResourceSentinelPolicy_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	description := "A terraform acctest policy"
	policy := `main = rule { true }`
	scope := "submit-job"
	enforcementLevel := "advisory"
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceSentinelPolicy_config(name, description, policy, scope, enforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(name, description, policy, scope, enforcementLevel),
			},
		},

		CheckDestroy: testResourceSentinelPolicy_checkDestroy(name),
	})
}

func TestResourceSentinelPolicy_nameChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	newName := acctest.RandomWithPrefix("tf-nomad-test")
	description := "A terraform acctest policy"
	policy := `main = rule { true }`
	scope := "submit-job"
	enforcementLevel := "advisory"
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceSentinelPolicy_config(name, description, policy, scope, enforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(name, description, policy, scope, enforcementLevel),
			},

			// Change our name
			{
				Config: testResourceSentinelPolicy_config(newName, description, policy, scope, enforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(newName, description, policy, scope, enforcementLevel),
			},
		},
		CheckDestroy: testResourceSentinelPolicy_checkDestroy(name),
	})
}

func TestResourceSentinelPolicy_update(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	description := "A terraform acctest policy"
	newDescription := "An updated terraform acctest policy"
	policy := `main = rule { true }`
	newPolicy := `
# Test policy only allows exec based tasks
main = rule { all_drivers_exec }

# all_drivers_exec checks that all the drivers in use are exec
all_drivers_exec = rule {
    all job.task_groups as tg {
        all tg.tasks as task {
            task.driver is "exec"
        }
    }
}
`
	scope := "submit-job"
	enforcementLevel := "advisory"
	newEnforcementLevel := "hard-mandatory"
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceSentinelPolicy_config(name, description, policy, scope, enforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(name, description, policy, scope, enforcementLevel),
			},
			{
				Config: testResourceSentinelPolicy_config(name, newDescription, newPolicy, scope, newEnforcementLevel),
				Check:  testResourceSentinelPolicy_checkAttrs(name, newDescription, newPolicy, scope, newEnforcementLevel),
			},
		},
		CheckDestroy: testResourceSentinelPolicy_checkDestroy(name),
	})
}

func testResourceSentinelPolicy_config(name, description, sentinelPolicy, scope, enforcementLevel string) string {
	return fmt.Sprintf(`
resource "nomad_sentinel_policy" "test" {
  name = "%s"
  description = "%s"
  policy = <<EOT
%s
EOT
  enforcement_level = "%s"
  scope = "%s"
}
`, name, description, sentinelPolicy, enforcementLevel, scope)
}

func testResourceSentinelPolicy_checkAttrs(name, description, sentinelPolicy, scope, enforcementLevel string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		sentinelPolicy = strings.TrimSpace(sentinelPolicy)
		resourceState := s.Modules[0].Resources["nomad_sentinel_policy.test"]
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

		if strings.TrimSpace(instanceState.Attributes["policy"]) != sentinelPolicy {
			return fmt.Errorf("expected policy to be %q, is %q in state", sentinelPolicy, strings.TrimSpace(instanceState.Attributes["policy"]))
		}

		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.SentinelPolicies().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back policy %q: %s", name, err)
		}

		if policy.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, policy.Name)
		}
		if policy.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, policy.Description)
		}
		if strings.TrimSpace(policy.Policy) != sentinelPolicy {
			return fmt.Errorf("expected policy to be %q, is %q in API", sentinelPolicy, strings.TrimSpace(policy.Policy))
		}
		if policy.Scope != scope {
			return fmt.Errorf("expected scope to be %q, is %q in API", scope, policy.Scope)
		}
		if policy.EnforcementLevel != enforcementLevel {
			return fmt.Errorf("expected enforcement level to be %q, is %q in API", enforcementLevel, policy.EnforcementLevel)
		}

		return nil
	}
}

func testResourceSentinelPolicy_checkExists(name string) resource.TestCheckFunc {
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

func testResourceSentinelPolicy_checkDestroy(name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		policy, _, err := client.ACLPolicies().Info(name, nil)
		if err != nil && strings.Contains(err.Error(), "404") || policy == nil {
			return nil
		}
		return fmt.Errorf("Policy %q has not been deleted.", name)
	}
}

func testResourceSentinelPolicy_delete(t *testing.T, name string) func() {
	return func() {
		client := testProvider.Meta().(ProviderConfig).Client
		_, err := client.ACLPolicies().Delete(name, nil)
		if err != nil {
			t.Fatalf("error deleting ACL policy: %s", err)
		}
	}
}
