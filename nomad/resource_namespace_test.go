package nomad

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceNamespace_import(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},
			{
				ResourceName:      "nomad_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},

		CheckDestroy: testResourceNamespace_checkDestroy(name),
	})
}

func TestResourceNamespace_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},
		},

		CheckDestroy: testResourceNamespace_checkDestroy(name),
	})
}

func TestResourceNamespace_refresh(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},

			// This should successfully cause the policy to be recreated,
			// testing the Exists function.
			{
				PreConfig: testResourceNamespace_delete(t, name),
				Config:    testResourceNamespace_initialConfig(name),
			},
		},
	})
}

func TestResourceNamespace_nameChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	newName := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},

			// Change our name
			{
				Config: testResourceNamespace_updateConfig(newName),
				Check:  testResourceNamespace_updateCheck(newName),
			},
		},
	})
}

func TestResourceNamespace_update(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},
			{
				Config: testResourceNamespace_updateConfig(name),
				Check:  testResourceNamespace_updateCheck(name),
			},
		},
	})
}

func TestResourceNamespace_deleteDefault(t *testing.T) {
	name := api.DefaultNamespace
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceNamespace_initialConfig(name),
				Check:  testResourceNamespace_initialCheck(name),
			},
		},

		CheckDestroy: testResourceNamespace_checkResetDefault(),
	})
}

func testResourceNamespace_initialConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_namespace" "test" {
  name = "%s"
  description = "A Terraform acctest namespace"
}
`, name)
}

func testResourceNamespace_initialCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			description = "A Terraform acctest namespace"
		)
		resourceState := s.Modules[0].Resources["nomad_namespace.test"]
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

		client := testProvider.Meta().(ProviderConfig).Client
		namespace, _, err := client.Namespaces().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back namespace %q: %s", name, err)
		}

		if namespace.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, namespace.Name)
		}
		if namespace.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, namespace.Description)
		}

		return nil
	}
}

func testResourceNamespace_checkExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		namespace, _, err := client.Namespaces().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back namespace %q: %s", name, err)
		}
		if namespace == nil {
			return fmt.Errorf("no namespace returned for %q", name)
		}

		return nil
	}
}

func testResourceNamespace_checkDestroy(name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		namespace, _, err := client.Namespaces().Info(name, nil)
		if err != nil && strings.Contains(err.Error(), "404") || namespace == nil {
			return nil
		}
		return fmt.Errorf("namespace %q has not been deleted.", name)
	}
}

func testResourceNamespace_checkResetDefault() resource.TestCheckFunc {
	return func(*terraform.State) error {
		defaultNamespace := api.Namespace{
			Name:        api.DefaultNamespace,
			Description: "Default shared namespace",
			Quota:       "",
		}
		client := testProvider.Meta().(ProviderConfig).Client
		namespace, _, err := client.Namespaces().Info(defaultNamespace.Name, nil)
		if err != nil {
			return fmt.Errorf("failed to find default namespace %q.", defaultNamespace.Name)
		}
		if namespace.Description != defaultNamespace.Description || namespace.Quota != defaultNamespace.Quota {
			return fmt.Errorf("default namespace %q not reset.", defaultNamespace.Name)
		}

		return nil
	}
}

func testResourceNamespace_delete(t *testing.T, name string) func() {
	return func() {
		client := testProvider.Meta().(ProviderConfig).Client
		_, err := client.Namespaces().Delete(name, nil)
		if err != nil {
			t.Fatalf("error deleting namespace %q: %s", name, err)
		}
	}
}

func testResourceNamespace_updateConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_namespace" "test" {
  name = "%s"
  description = "An updated Terraform acctest namespace"
}
`, name)
}

func testResourceNamespace_updateCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const (
			description = "An updated Terraform acctest namespace"
		)
		resourceState := s.Modules[0].Resources["nomad_namespace.test"]
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

		client := testProvider.Meta().(ProviderConfig).Client
		namespace, _, err := client.Namespaces().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back namespace %q: %s", name, err)
		}

		if namespace.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, namespace.Name)
		}
		if namespace.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, namespace.Description)
		}

		return nil
	}
}
