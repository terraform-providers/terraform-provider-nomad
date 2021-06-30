package nomad

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataSourceNamespace(t *testing.T) {
	resourceName := "data.nomad_namespace.test"

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceNamespaceConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", "default"),
					resource.TestCheckResourceAttr(resourceName, "description", "Default shared namespace"),
					resource.TestCheckResourceAttr(resourceName, "quota", ""),
				),
			},
			{
				Config:      testDataSourceNamespaceConfig_doesNotExists,
				ExpectError: regexp.MustCompile("Namespace not found"),
			},
		},
	})
}

const testDataSourceNamespaceConfig = `
data "nomad_namespace" "test" {
	name = "default"
}
`

const testDataSourceNamespaceConfig_doesNotExists = `
data "nomad_namespace" "test" {
	name = "does-not-exists"
}
`
