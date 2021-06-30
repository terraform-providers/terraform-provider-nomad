package nomad

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestDataSourceNamespaces(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceNamespaces_config,
				Check:  testDataSourceNamespaces_check,
			},
		},
	})
}

var testDataSourceNamespaces_config = `

data "nomad_namespaces" "test" {
}

`

func testDataSourceNamespaces_check(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.nomad_namespaces.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state %v", s.Modules[0].Resources)
	}

	iState := resourceState.Primary
	if iState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	results, err := strconv.ParseInt(iState.Attributes["namespaces.#"], 10, 64)
	if err != nil {
		return fmt.Errorf("expected integer in state, got %s (%T)", iState.Attributes["namespaces.#"], iState.Attributes["namespaces.#"])
	}

	if results < 1 {
		return fmt.Errorf("got %d namespaces, expected at least 1", results)
	}

	return nil
}
