package nomad

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestDataSourceRegions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceRegions_config,
				Check:  testDataSourceRegions_check,
			},
		},
	})
}

var testDataSourceRegions_config = `

data "nomad_regions" "test" {
}

`

func testDataSourceRegions_check(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.nomad_regions.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state %v", s.Modules[0].Resources)
	}

	iState := resourceState.Primary
	if iState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	results, err := strconv.ParseInt(iState.Attributes["regions.#"], 10, 64)
	if err != nil {
		return fmt.Errorf("expected integer in state, got %s (%T)", iState.Attributes["regions.#"], iState.Attributes["regions.#"])
	}

	if results < 1 {
		return fmt.Errorf("got %d regions, expected at least 1", results)
	}

	return nil
}
