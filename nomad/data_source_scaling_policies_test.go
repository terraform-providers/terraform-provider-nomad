package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataSourceScalingPolicies_Basic(t *testing.T) {
	dataSourceName := "data.nomad_scaling_policies.policies"

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.11.0") },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceScalingPoliciesJobConfig,
			},
			{
				Config: testDataSourceScalingPoliciesJobConfig + testDataSourceScalingPoliciesConfig("foo-scaling-policies", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "policies.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.enabled", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.type", "horizontal"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Job", "foo-scaling-policies"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Group", "foo"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfig + testDataSourceScalingPoliciesConfig("", "horizontal"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "policies.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.enabled", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.type", "horizontal"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Job", "foo-scaling-policies"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Group", "foo"),
				),
			},
		},
	})

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t); testCheckMinVersion(t, "1.0.0-beta2") },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt,
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("", ""),
				Check: resource.ComposeTestCheckFunc(
					// We can't guarantee order, so test length only for now.
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "3"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("foo-scaling-policies-ent", ""),
				Check: resource.ComposeTestCheckFunc(
					// We can't guarantee order, so test length only for now.
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "3"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("", "vertical"),
				Check: resource.ComposeTestCheckFunc(
					// We can't guarantee order, so test length only for now.
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "2"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("", "horizontal"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "policies.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.enabled", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.type", "horizontal"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Job", "foo-scaling-policies-ent"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Group", "foo"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("", "vertical_cpu"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "policies.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.enabled", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.type", "vertical_cpu"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Job", "foo-scaling-policies-ent"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Group", "foo"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Task", "foo"),
				),
			},
			{
				Config: testDataSourceScalingPoliciesJobConfigEnt + testDataSourceScalingPoliciesConfig("", "vertical_mem"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "policies.#", "1"),
					resource.TestCheckResourceAttrSet(dataSourceName, "policies.0.id"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.enabled", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.type", "vertical_mem"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Job", "foo-scaling-policies-ent"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Group", "foo"),
					resource.TestCheckResourceAttr(dataSourceName, "policies.0.target.Task", "foo"),
				),
			},
		},
	})
}

const testDataSourceScalingPoliciesJobConfig = `
resource "nomad_job" "job" {
  purge_on_destroy = true

  jobspec = <<EOF
job "foo-scaling-policies" {
  datacenters = ["dc1"]
  group "foo" {

    scaling {
      enabled = false
      min     = 1
      max     = 20
      type    = "horizontal"

      policy {
        cooldown = "20s"
        check "avg_instance_sessions" {
          source = "prometheus"
          query  = "query"
          strategy "target-value" {
            target = 5
          }
        }
      }
    }

    task "foo" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["10"]
      }
    }
  }
}
EOF
}
`

const testDataSourceScalingPoliciesJobConfigEnt = `
resource "nomad_job" "job" {
  purge_on_destroy = true

  jobspec = <<EOF
job "foo-scaling-policies-ent" {
  datacenters = ["dc1"]
  group "foo" {

    scaling {
      enabled = false
      min     = 1
      max     = 20
      type    = "horizontal"

      policy {
        cooldown = "20s"
        check "avg_instance_sessions" {
          source = "prometheus"
          query  = "query"
          strategy "target-value" {
            target = 5
          }
        }
      }
    }

    task "foo" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = ["10"]
      }
      scaling "cpu" {
        policy {
          check "check" {
            query = "query"
          }
        }
      }

      scaling "mem" {
        policy {
          check "check" {
            query = "query"
          }
        }
      }
    }
  }
}
EOF
}
`

func testDataSourceScalingPoliciesConfig(jobID string, typeQuery string) string {
	var config string

	if jobID != "" {
		config += fmt.Sprintf("job_id = %q\n", jobID)
	}

	if typeQuery != "" {
		config += fmt.Sprintf("type = %q\n", typeQuery)
	}

	return fmt.Sprintf(`
data "nomad_scaling_policies" "policies" {
%s
}
`, config)
}
