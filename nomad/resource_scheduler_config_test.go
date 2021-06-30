package nomad

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestSchedulerConfig_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testProviders,
		CheckDestroy: testFinalConfiguration,
		Steps: []resource.TestStep{
			{
				Config: testAccNomadSchedulerConfigSpread,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"scheduler_algorithm",
						"spread",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.batch_scheduler_enabled",
						"true",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.service_scheduler_enabled",
						"true",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.system_scheduler_enabled",
						"true",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"memory_oversubscription_enabled",
						"false",
					),
				),
			},
			{
				Config: testAccNomadSchedulerConfigBinpack,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"scheduler_algorithm",
						"binpack",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.batch_scheduler_enabled",
						"false",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.service_scheduler_enabled",
						"true",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"preemption_config.system_scheduler_enabled",
						"false",
					),
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"memory_oversubscription_enabled",
						"false",
					),
				),
			},
			{
				Config: testAccNomadSchedulerConfigMemoryOversubscription,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"nomad_scheduler_config.config",
						"memory_oversubscription_enabled",
						"true",
					),
				),
			},
			{
				Config: testAccNomadSchedulerConfigDataSource,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"data.nomad_scheduler_config.config",
						"scheduler_algorithm",
						"binpack",
					),
					resource.TestCheckResourceAttr(
						"data.nomad_scheduler_config.config",
						"preemption_config.batch_scheduler_enabled",
						"false",
					),
					resource.TestCheckResourceAttr(
						"data.nomad_scheduler_config.config",
						"preemption_config.service_scheduler_enabled",
						"true",
					),
					resource.TestCheckResourceAttr(
						"data.nomad_scheduler_config.config",
						"preemption_config.system_scheduler_enabled",
						"false",
					),
					resource.TestCheckResourceAttr(
						"data.nomad_scheduler_config.config",
						"memory_oversubscription_enabled",
						"true",
					),
				),
			},
		},
	})
}

const testAccNomadSchedulerConfigSpread = `
resource "nomad_scheduler_config" "config" {
	scheduler_algorithm = "spread"
	preemption_config = {
		system_scheduler_enabled = true
		batch_scheduler_enabled = true
		service_scheduler_enabled = true
	}
}
`

const testAccNomadSchedulerConfigBinpack = `
resource "nomad_scheduler_config" "config" {
	scheduler_algorithm = "binpack"
	preemption_config = {
		system_scheduler_enabled = false
		batch_scheduler_enabled = false
		service_scheduler_enabled = true
	}
}
`

const testAccNomadSchedulerConfigMemoryOversubscription = `
resource "nomad_scheduler_config" "config" {
	memory_oversubscription_enabled = true
	scheduler_algorithm = "binpack"
	preemption_config = {
		system_scheduler_enabled = false
		batch_scheduler_enabled = false
		service_scheduler_enabled = true
	}
}
`

const testAccNomadSchedulerConfigDataSource = `
data "nomad_scheduler_config" "config" {}

resource "nomad_scheduler_config" "config" {
	memory_oversubscription_enabled = true
	scheduler_algorithm = "binpack"
	preemption_config = {
		system_scheduler_enabled = false
		batch_scheduler_enabled = false
		service_scheduler_enabled = true
	}
}
`

// for details on why this is the way it is, checkout the comments on
// resourceSchedulerConfigurationDelete.
func testFinalConfiguration(_ *terraform.State) error { return nil }
