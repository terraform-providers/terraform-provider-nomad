package nomad

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceACLToken_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLToken_initialConfig(),
				Check:  testResourceACLToken_initialCheck(),
			},
			{
				ResourceName:      "nomad_acl_token.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},

		CheckDestroy: testResourceACLToken_checkDestroy,
	})
}

func TestResourceACLToken_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLToken_initialConfig(),
				Check:  testResourceACLToken_initialCheck(),
			},
		},

		CheckDestroy: testResourceACLToken_checkDestroy,
	})
}

func TestResourceACLToken_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceACLToken_initialConfig(),
				Check:  testResourceACLToken_initialCheck(),
			},
			{
				Config: testResourceACLToken_updateConfig(),
				Check:  testResourceACLToken_updateCheck(),
			},
		},

		CheckDestroy: testResourceACLToken_checkDestroy,
	})
}

func testResourceACLToken_initialConfig() string {
	return `
resource "nomad_acl_token" "test" {
  name = "Terraform Test Token"
  type = "client"
  policies = ["dev", "qa"]
  global = false
}
`
}

func testResourceACLToken_initialCheck() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			name     = "Terraform Test Token"
			typ      = "client"
			policies = "2"
			global   = "false"
		)
		resourceState := s.Modules[0].Resources["nomad_acl_token.test"]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}

		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}

		if len(instanceState.ID) < 1 {
			return fmt.Errorf("expected ID to be set, got %q", instanceState.ID)
		}

		if len(instanceState.Attributes["secret_id"]) < 1 {
			return fmt.Errorf("expected secret_id to be set, got %q", instanceState.Attributes["secret_id"])
		}

		if instanceState.Attributes["name"] != name {
			return fmt.Errorf("expected name to be %q, is %q in state", name, instanceState.Attributes["name"])
		}

		if instanceState.Attributes["type"] != typ {
			return fmt.Errorf("expected type to be %q, is %q in state", typ, instanceState.Attributes["type"])
		}

		if instanceState.Attributes["global"] != global {
			return fmt.Errorf("expected global to be %q, is %q in state", global, instanceState.Attributes["global"])
		}

		if instanceState.Attributes["create_time"] == "" {
			return fmt.Errorf("expected create_time to be set, got %q", instanceState.Attributes["create_time"])
		}
		// because policies is a set, it's a pain to try and check the values here
		if instanceState.Attributes["policies.#"] != policies {
			return fmt.Errorf("expected policies.# to be %q, is %q in state", policies, instanceState.Attributes["policies.#"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		token, _, err := client.ACLTokens().Info(instanceState.ID, nil)
		if err != nil {
			return fmt.Errorf("error reading back token %q: %s", instanceState.ID, err)
		}

		if token.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, token.Name)
		}
		if token.Type != typ {
			return fmt.Errorf("expected type to be %q, is %q in API", typ, token.Type)
		}
		if token.Global != false {
			return fmt.Errorf("expected global to be %v, is %v in API", false, token.Global)
		}
		if len(token.Policies) != 2 {
			return fmt.Errorf("expected %d policies, got %v from the API", 2, token.Policies)
		}

		return nil
	}
}

func testResourceACLToken_checkDestroy(s *terraform.State) error {
	for _, s := range s.Modules[0].Resources {
		if s.Type != "nomad_acl_token" {
			continue
		}
		if s.Primary == nil {
			continue
		}
		client := testProvider.Meta().(ProviderConfig).Client
		token, _, err := client.ACLTokens().Info(s.Primary.ID, nil)
		if err != nil && strings.Contains(err.Error(), "404") || token == nil {
			continue
		}
		return fmt.Errorf("Token %q has not been deleted.", token.AccessorID)
	}

	return nil
}

func testResourceACLToken_updateConfig() string {
	return `
resource "nomad_acl_token" "test" {
  name = "Updated Terraform Test Token"
  type = "client"
  policies = ["dev", "qa", "prod"]
  global = false
}
`
}

func testResourceACLToken_updateCheck() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			name     = "Updated Terraform Test Token"
			typ      = "client"
			policies = "3"
			global   = "false"
		)
		resourceState := s.Modules[0].Resources["nomad_acl_token.test"]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}

		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}

		if len(instanceState.ID) < 1 {
			return fmt.Errorf("expected ID to be set, got %q", instanceState.ID)
		}

		if len(instanceState.Attributes["secret_id"]) < 1 {
			return fmt.Errorf("expected secret_id to be set, got %q", instanceState.Attributes["secret_id"])
		}

		if instanceState.Attributes["name"] != name {
			return fmt.Errorf("expected name to be %q, is %q in state", name, instanceState.Attributes["name"])
		}

		if instanceState.Attributes["type"] != typ {
			return fmt.Errorf("expected type to be %q, is %q in state", typ, instanceState.Attributes["type"])
		}

		if instanceState.Attributes["global"] != global {
			return fmt.Errorf("expected global to be %q, is %q in state", global, instanceState.Attributes["global"])
		}
		// because policies is a set, it's a pain to try and check the values here
		if instanceState.Attributes["policies.#"] != policies {
			return fmt.Errorf("expected policies.# to be %q, is %q in state", policies, instanceState.Attributes["policies.#"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		token, _, err := client.ACLTokens().Info(instanceState.ID, nil)
		if err != nil {
			return fmt.Errorf("error reading back token %q: %s", instanceState.ID, err)
		}

		if token.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, token.Name)
		}
		if token.Type != typ {
			return fmt.Errorf("expected type to be %q, is %q in API", typ, token.Type)
		}
		if token.Global != false {
			return fmt.Errorf("expected global to be %v, is %v in API", false, token.Global)
		}
		if len(token.Policies) != 3 {
			return fmt.Errorf("expected %d policies, got %v from the API", 3, token.Policies)
		}

		return nil
	}
}
