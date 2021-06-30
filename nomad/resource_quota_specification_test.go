package nomad

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/hashicorp/nomad/api"
)

func TestResourceQuotaSpecification_import(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceQuotaSpecification_initialConfig(name),
				Check:  testResourceQuotaSpecification_initialCheck(name),
			},
			{
				ResourceName:      "nomad_quota_specification.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},

		CheckDestroy: testResourceQuotaSpecification_checkDestroy(name),
	})
}

func TestResourceQuotaSpecification_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceQuotaSpecification_initialConfig(name),
				Check:  testResourceQuotaSpecification_initialCheck(name),
			},
		},

		CheckDestroy: testResourceQuotaSpecification_checkDestroy(name),
	})
}

func TestResourceQuotaSpecification_refresh(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceQuotaSpecification_initialConfig(name),
				Check:  testResourceQuotaSpecification_initialCheck(name),
			},

			// This should successfully cause the quota spec to be recreated,
			// testing the Exists function.
			{
				PreConfig: testResourceQuotaSpecification_delete(t, name),
				Config:    testResourceQuotaSpecification_initialConfig(name),
			},
		},
	})
}

func TestResourceQuotaSpecification_nameChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	newName := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceQuotaSpecification_initialConfig(name),
				Check:  testResourceQuotaSpecification_initialCheck(name),
			},

			// Change our name
			{
				Config: testResourceQuotaSpecification_updateConfig(newName),
				Check:  testResourceQuotaSpecification_updateCheck(newName),
			},
		},
	})
}

func TestResourceQuotaSpecification_update(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-nomad-test")
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testResourceQuotaSpecification_initialConfig(name),
				Check:  testResourceQuotaSpecification_initialCheck(name),
			},
			{
				Config: testResourceQuotaSpecification_updateConfig(name),
				Check:  testResourceQuotaSpecification_updateCheck(name),
			},
		},
	})
}

func testResourceQuotaSpecification_initialConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_quota_specification" "test" {
  name = "%s"
  description = "A Terraform acctest quota specification"
  limits {
	  region = "global"
	  region_limit {
		  cpu = 2500
	  }
  }
}
`, name)
}

func testResourceQuotaSpecification_initialCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const description = "A Terraform acctest quota specification"
		cpu := 2500
		limits := []*api.QuotaLimit{
			{
				Region: "global",
				RegionLimit: &api.Resources{
					CPU: &cpu,
				},
			},
		}
		resourceState := s.Modules[0].Resources["nomad_quota_specification.test"]
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

		if instanceState.Attributes["limits.#"] != "1" {
			return fmt.Errorf("expected limits.# to be %q, is %q in state", "1", instanceState.Attributes["limits.#"])
		}

		var key, regKey string
		for k := range instanceState.Attributes {
			if !strings.HasPrefix(k, "limits.") {
				continue
			}
			parts := strings.Split(k, ".")
			if len(parts) < 4 {
				continue
			}
			if parts[1] == "#" || parts[3] == "#" {
				continue
			}
			key = parts[1]
			regKey = parts[3]
		}

		if instanceState.Attributes["limits."+key+".region"] != limits[0].Region {
			return fmt.Errorf("expected limits.%s.region to be %q, is %q in state", key, limits[0].Region, instanceState.Attributes["limits."+key+".region"])
		}

		if instanceState.Attributes["limits."+key+".region_limit."+regKey+".cpu"] != strconv.Itoa(*limits[0].RegionLimit.CPU) {
			return fmt.Errorf("expected limits.%s.region_limit.%s.cpu to be %q, is %q in state", key, regKey,
				strconv.Itoa(*limits[0].RegionLimit.CPU),
				instanceState.Attributes["limits."+key+".region_limit."+regKey+".cpu"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		spec, _, err := client.Quotas().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back quota specification %q: %s", name, err)
		}

		if spec.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, spec.Name)
		}
		if spec.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, spec.Description)
		}

		if len(spec.Limits) != len(limits) {
			return fmt.Errorf("expected %d limits, is %d in API", len(limits), len(spec.Limits))
		}
		if spec.Limits[0].Region != limits[0].Region {
			return fmt.Errorf("expected limits[0].Region to be %q, is %q in API", limits[0].Region, spec.Limits[0].Region)
		}
		if *spec.Limits[0].RegionLimit.CPU != *limits[0].RegionLimit.CPU {
			return fmt.Errorf("expected limits[0].RegionLimit.CPU to be %d, is %d in API", *limits[0].RegionLimit.CPU, *spec.Limits[0].RegionLimit.CPU)
		}

		return nil
	}
}

func testResourceQuotaSpecification_checkExists(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		spec, _, err := client.Quotas().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back quota specification %q: %s", name, err)
		}
		if spec == nil {
			return fmt.Errorf("no quota specification returned for %q", name)
		}

		return nil
	}
}

func testResourceQuotaSpecification_checkDestroy(name string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		client := testProvider.Meta().(ProviderConfig).Client
		spec, _, err := client.Quotas().Info(name, nil)
		if err != nil && strings.Contains(err.Error(), "404") || spec == nil {
			return nil
		} else if err != nil {
			return fmt.Errorf("error checking if quota specification %q exists: %s", name, err.Error())
		}
		return fmt.Errorf("quota specification %q has not been deleted.", name)
	}
}

func testResourceQuotaSpecification_delete(t *testing.T, name string) func() {
	return func() {
		client := testProvider.Meta().(ProviderConfig).Client
		_, err := client.Quotas().Delete(name, nil)
		if err != nil {
			t.Fatalf("error deleting quota specification %q: %s", name, err)
		}
	}
}

func testResourceQuotaSpecification_updateConfig(name string) string {
	return fmt.Sprintf(`
resource "nomad_quota_specification" "test" {
  name = "%s"
  description = "An updated Terraform acctest quota specification"
  limits {
	  region = "global"
	  region_limit {
		  cpu = 2400
		  memory_mb = 1900
	  }
  }
}
`, name)
}

func testResourceQuotaSpecification_updateCheck(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		const description = "An updated Terraform acctest quota specification"
		cpu := 2400
		mem := 1900
		limits := []*api.QuotaLimit{
			{
				Region: "global",
				RegionLimit: &api.Resources{
					CPU:      &cpu,
					MemoryMB: &mem,
				},
			},
		}

		resourceState := s.Modules[0].Resources["nomad_quota_specification.test"]
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

		if instanceState.Attributes["limits.#"] != "1" {
			return fmt.Errorf("expected limits.# to be %q, is %q in state", "1", instanceState.Attributes["limits.#"])
		}

		var key, regKey string
		for k := range instanceState.Attributes {
			if !strings.HasPrefix(k, "limits.") {
				continue
			}
			parts := strings.Split(k, ".")
			if len(parts) < 4 {
				continue
			}
			if parts[1] == "#" || parts[3] == "#" {
				continue
			}
			key = parts[1]
			regKey = parts[3]
		}

		if instanceState.Attributes["limits."+key+".region"] != limits[0].Region {
			return fmt.Errorf("expected limits.%s.region to be %q, is %q in state", key, limits[0].Region, instanceState.Attributes["limits."+key+".region"])
		}

		if instanceState.Attributes["limits."+key+".region_limit."+regKey+".cpu"] != strconv.Itoa(*limits[0].RegionLimit.CPU) {
			return fmt.Errorf("expected limits.%s.region_limit.%s.cpu to be %q, is %q in state", key, regKey,
				strconv.Itoa(*limits[0].RegionLimit.CPU),
				instanceState.Attributes["limits."+key+".region_limit."+regKey+".cpu"])
		}

		if instanceState.Attributes["limits."+key+".region_limit."+regKey+".memory_mb"] != strconv.Itoa(*limits[0].RegionLimit.MemoryMB) {
			return fmt.Errorf("expected limits.%s.region_limit.%s.memory_mb to be %q, is %q in state", key, regKey,
				strconv.Itoa(*limits[0].RegionLimit.MemoryMB),
				instanceState.Attributes["limits."+key+".region_limit."+regKey+".memory_mb"])
		}

		client := testProvider.Meta().(ProviderConfig).Client
		spec, _, err := client.Quotas().Info(name, nil)
		if err != nil {
			return fmt.Errorf("error reading back quota specification %q: %s", name, err)
		}

		if spec.Name != name {
			return fmt.Errorf("expected name to be %q, is %q in API", name, spec.Name)
		}
		if spec.Description != description {
			return fmt.Errorf("expected description to be %q, is %q in API", description, spec.Description)
		}

		if len(spec.Limits) != len(limits) {
			return fmt.Errorf("expected %d limits, is %d in API", len(limits), len(spec.Limits))
		}
		if spec.Limits[0].Region != limits[0].Region {
			return fmt.Errorf("expected limits[0].Region to be %q, is %q in API", limits[0].Region, spec.Limits[0].Region)
		}
		if *spec.Limits[0].RegionLimit.CPU != *limits[0].RegionLimit.CPU {
			return fmt.Errorf("expected limits[0].RegionLimit.CPU to be %d, is %d in API", *limits[0].RegionLimit.CPU, *spec.Limits[0].RegionLimit.CPU)
		}
		if *spec.Limits[0].RegionLimit.MemoryMB != *limits[0].RegionLimit.MemoryMB {
			return fmt.Errorf("expected limits[0].RegionLimit.MemoryMB to be %d, is %d in API", *limits[0].RegionLimit.MemoryMB, *spec.Limits[0].RegionLimit.MemoryMB)
		}

		return nil
	}
}
