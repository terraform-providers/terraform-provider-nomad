package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceNomadJob_Basic(t *testing.T) {
	job := "testjobds"
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testProviders,
		CheckDestroy: testResourceJob_forceDestroyWithPurge(job, "default"),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDataSourceConfig(job),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourceNomadJobExists("data.nomad_job.test-job", "default"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "name", job),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "type", "service"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "priority", "50"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "namespace", "default"),
				),
			},
		},
	})
}

func TestAccDataSourceNomadJob_Periodic(t *testing.T) {
	job := "testjobds_periodic"
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testProviders,
		CheckDestroy: testResourceJob_forceDestroyWithPurge(job, "default"),
		Steps: []resource.TestStep{
			{
				Config: testAccJobDataSourceConfigPeriodic(job),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourceNomadJobExists("data.nomad_job.test-job-periodic", "default"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job-periodic", "name", job),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job-periodic", "type", "batch"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job-periodic", "priority", "50"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job-periodic", "namespace", "default"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job-periodic", "periodic_config.#", "1"),
				),
			},
		},
	})
}

func TestAccDataSourceNomadJob_Namespaced(t *testing.T) {
	ns := "jobds-test-namespace"
	job := "testjobds-namespace"
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testCheckEnt(t) },
		Providers:    testProviders,
		CheckDestroy: testResourceJob_forceDestroyWithPurge(job, ns),
		Steps: []resource.TestStep{
			{
				Config: testAccNSJobDataSourceConfig(job, ns),
				Check: resource.ComposeTestCheckFunc(
					testAccDataSourceNomadJobExists("data.nomad_job.test-job", ns),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "name", job),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "type", "batch"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "priority", "50"),
					resource.TestCheckResourceAttr(
						"data.nomad_job.test-job", "namespace", ns),
				),
			},
		},
	})
}

func testAccDataSourceNomadJobExists(n, namespace string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Job ID is set")
		}

		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client

		id := rs.Primary.ID

		// Try to find the job
		test_job, _, err := client.Jobs().Info(id, &api.QueryOptions{
			Namespace: namespace,
		})

		if err != nil {
			return err
		}

		if *test_job.ID != rs.Primary.ID {
			return fmt.Errorf("Job not found")
		}

		return nil
	}
}

func testAccNSJobDataSourceConfig(job, ns string) string {
	return `
resource "nomad_namespace" "ns-instance" {
  name = "` + ns + `"
}

resource "nomad_job" "job-instance" {
	jobspec = <<EOT
		job "` + job + `" {
			datacenters = ["dc1"]
			type = "batch"
			namespace = "${nomad_namespace.ns-instance.name}"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/echo"
						args = ["test"]
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

data "nomad_job" "test-job" {
  job_id    = "${nomad_job.job-instance.id}"
  namespace = "${nomad_job.job-instance.namespace}"
}
`
}

func testAccJobDataSourceConfig(job string) string {
	return `
resource "nomad_job" "job-instance" {
	jobspec = <<EOT
		job "` + job + `" {
			datacenters = ["dc1"]
			type = "service"
			update {
			    max_parallel = 2
				min_healthy_time = "11s"
				healthy_deadline = "6m"
				progress_deadline = "11m"
				auto_revert = true
				auto_promote = true
				canary = 1
			}
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/echo"
						args = ["test"]
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

data "nomad_job" "test-job" {
  job_id    = "${nomad_job.job-instance.id}"
}
`
}

func testAccJobDataSourceConfigPeriodic(job string) string {
	return `
resource "nomad_job" "job-instance-periodic" {
	jobspec = <<EOT
		job "` + job + `" {
			datacenters = ["dc1"]
			type = "batch"
			periodic {
        		cron             = "*/15 * * * * *"
				prohibit_overlap = true
			}
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/echo"
						args = ["test"]
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

data "nomad_job" "test-job-periodic" {
  job_id    = "${nomad_job.job-instance-periodic.id}"
}
`
}
