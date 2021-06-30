package nomad

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestDataSourceScalingPolicy_Basic(t *testing.T) {
	dataSourceName := "data.nomad_scaling_policy.policy"

	resource.Test(t, resource.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.11.0") },
		Steps: []resource.TestStep{
			{
				Config: testDataSourceScalingPolicyConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSourceName, "id"),
					resource.TestCheckResourceAttr(dataSourceName, "enabled", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "type", "horizontal"),
					resource.TestCheckResourceAttr(dataSourceName, "min", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "max", "20"),
					resource.TestCheckResourceAttr(dataSourceName, "policy", `{"cooldown":"20s"}`),
					resource.TestCheckResourceAttr(dataSourceName, "target.Namespace", "default"),
					resource.TestCheckResourceAttr(dataSourceName, "target.Job", "foo-scaling-policy"),
					resource.TestCheckResourceAttr(dataSourceName, "target.Group", "foo"),
				),
			},
		},
	})
}

const testDataSourceScalingPolicyConfig = `
resource "nomad_job" "job" {
  purge_on_destroy = true

  jobspec = <<EOF
job "foo-scaling-policy" {
  datacenters = ["dc1"]
  group "foo" {

    scaling {
      enabled = false
      min     = 1
      max     = 20
	  type    = "horizontal"

      policy {
        cooldown = "20s"
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


data "nomad_scaling_policies" "policies" {
  job_id = nomad_job.job.name
}

data "nomad_scaling_policy" "policy" {
	id = data.nomad_scaling_policies.policies.policies[0].id
}
`
