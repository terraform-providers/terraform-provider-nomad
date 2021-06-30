package nomad

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// Test will
// * Create job with update stanza
// * Pass config with same job spec and the deployments data source
// * Check there is 1 deployment and Destroy after this step
// * Pass just the data source config
// * Verify deployment is cancelled
func TestAccDataSourceDeployments(t *testing.T) {
	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccCheckDataSourceNomadDeploymentsJobCfg,
			},
			{
				Config: testAccCheckDataSourceNomadDeploymentsCfgWithJob,
				Check: func(s *terraform.State) error {
					rs, _ := s.RootModule().Resources["data.nomad_deployments.foobar"]
					is := rs.Primary
					v, ok := is.Attributes["deployments.#"]
					if !ok {
						return fmt.Errorf("Attribute '%s' not found", "deployments.#")
					}
					numDeployments, err := strconv.Atoi(v)
					if err != nil {
						return fmt.Errorf("received error parsing 'deployments.#': %v", err)
					} else if numDeployments < 1 {
						return fmt.Errorf("Attribute 'deployments.#' should be >= 1, got %v", v)
					}
					return nil
				},
				Destroy: true,
			},
			{
				Config: testAccCheckDataSourceNomadDeploymentsCfg,
				Check: func(s *terraform.State) error {
					re := regexp.MustCompile(`^deployments.(\d+).JobID$`)
					rs, _ := s.RootModule().Resources["data.nomad_deployments.foobar"]
					is := rs.Primary
					index := -1
					for k, v := range is.Attributes {
						// any match of this job should be fine, all deployments should be "cancelled"
						if submatch := re.FindStringSubmatch(k); submatch != nil && v == "foo_deploy" {
							index, _ = strconv.Atoi(submatch[1])
							break
						}
					}
					if index < 0 {
						return fmt.Errorf("did not find expected deployment for job 'foo_deploy'")
					}
					statusAttr := fmt.Sprintf("deployments.%d.Status", index)
					if s, ok := is.Attributes[statusAttr]; !ok || s != "cancelled" {
						if !ok {
							return fmt.Errorf("did not find expected attributed '%v'", statusAttr)
						}
						return fmt.Errorf("'%v': expected 'cancelled', got '%v' to be 'cancelled'", statusAttr, s)
					}
					return nil
				},
			},
		},

		// Somewhat-abuse CheckDestroy to actually do our cleanup... :/
		CheckDestroy: testResourceJob_forceDestroyWithPurge("foo_deploy", "default"),
	})
}

var testAccCheckDataSourceNomadDeploymentsJobCfg = `
resource "nomad_job" "foobar" {
	jobspec = <<EOT
		job "foo_deploy" {
			update {} ## creates deployment
			datacenters = ["dc1"]
			type = "service"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["1"]
					}
					resources {
						cpu = 100
						memory = 10
					}
					logs {
						max_files = 3
						max_file_size = 10
					}
				}
			}
		}
	EOT
}
`
var testAccCheckDataSourceNomadDeploymentsCfg = `

data "nomad_deployments" "foobar" {}

`

var testAccCheckDataSourceNomadDeploymentsCfgWithJob = testAccCheckDataSourceNomadDeploymentsJobCfg + testAccCheckDataSourceNomadDeploymentsCfg
