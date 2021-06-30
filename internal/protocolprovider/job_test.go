package protocol

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	tfmux "github.com/hashicorp/terraform-plugin-mux"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-nomad/nomad"
	"github.com/stretchr/testify/require"
)

var testAccProtoV5ProviderFactories = map[string]func() (tfprotov5.ProviderServer, error){}

func init() {
	if v := os.Getenv("NOMAD_ADDR"); v == "" {
		os.Setenv("NOMAD_ADDR", "http://127.0.0.1:4646")
	}

	testAccProtoV5ProviderFactories["nomad"] = func() (tfprotov5.ProviderServer, error) {
		ctx := context.Background()

		// the ProviderServer from SDKv2
		sdkv2 := nomad.Provider()

		// the terraform-plugin-go provider
		tpg := Server(sdkv2)

		factory, err := tfmux.NewSchemaServerFactory(ctx, sdkv2.GRPCProvider, tpg)
		if err != nil {
			return nil, err
		}
		return factory.Server(), nil
	}
}

func TestAccResourceJob(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testAccProtoV5ProviderFactories,
		CheckDestroy:             testJobIsAbsent("test-nomad-job-v2"),
		Steps: []resource.TestStep{
			{
				PreConfig:   createTestJob(t),
				Config:      testAccNomadJob,
				ExpectError: regexp.MustCompile("Enforcing job modify index 0"),
			},
			{
				PreConfig: removeJob(t, "test-nomad-job-v2"),
				Config:    testAccNomadJob,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nomad_job_v2.test", "id", "test-nomad-job-v2"),
					testJobIsPresent("test-nomad-job-v2"),
					// The out object should contain the expanded job configuration
					// after it has been applied
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.#", "2"),
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.0", "leader"),
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.1", "mysql"),
				),
			},
			{
				// Trying to change to a tag and getting the correct result in
				// 'out' ensure that we set it correctly during the plan
				Config: testAccNomadJobChangeTag,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.#", "1"),
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.0", "leader"),
				),
			},
			{
				// When the job has been externally removed, Terraform should
				// detect it and create it again
				PreConfig: removeJob(t, "test-nomad-job-v2"),
				Config:    testAccNomadJobChangeTag,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.#", "1"),
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.group.cache.service.0.tags.0", "leader"),
				),
			},
			{
				// When a Job is changed externally the change should be
				// detected and a plan should be created
				PreConfig:          changePriority(t, "test-nomad-job-v2", 75),
				Config:             testAccNomadJobChangeTag,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccNomadJobChangeId,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nomad_job_v2.test", "id", "test-nomad-job-v2-change-id"),
					testJobIsAbsent("test-nomad-job-v2"),
					testJobIsPresent("test-nomad-job-v2-change-id"),
				),
			},
			{
				Config: testAccNomadJobChangeType,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nomad_job_v2.test", "out.type", "system"),
				),
			},
			{
				ResourceName: "nomad_job_v2.test",
				ImportState:  true,
			},
		},
	})
}

func createTestJob(t *testing.T) func() {
	return func() {
		client, err := api.NewClient(api.DefaultConfig())
		require.NoError(t, err)
		job := api.NewServiceJob("test-nomad-job-v2", "test-nomad-job-v2", "", 50)
		job.AddDatacenter("dc1")
		task := api.NewTask("test", "exec")
		job.AddTaskGroup(api.NewTaskGroup("test", 1).AddTask(task))
		_, _, err = client.Jobs().Register(job, nil)
		require.NoError(t, err)
	}
}

func getJob(name string) (*api.Job, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate test client: %s", err)
	}
	job, _, err := client.Jobs().Info(name, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, fmt.Errorf("error while looking for %q: %s", name, err)
	}

	return job, nil
}

func testJobIsAbsent(name string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		job, err := getJob(name)
		if err != nil {
			return err
		}
		if job != nil {
			return fmt.Errorf("Job %q is still present.", name)
		}
		return nil
	}
}

func testJobIsPresent(name string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		job, err := getJob(name)
		if err != nil {
			return err
		}
		if job == nil {
			return fmt.Errorf("Job %q is absent", name)
		}
		return nil
	}
}

func changePriority(t *testing.T, name string, priority int) func() {
	return func() {
		client, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			t.Fatalf("failed to instantiate test client: %s", err)
		}
		job, _, err := client.Jobs().Info(name, nil)
		if err != nil {
			t.Fatalf("failed to fetch job: %s", err)
		}
		job.Priority = &priority
		_, _, err = client.Jobs().Register(job, nil)
		if err != nil {
			t.Fatalf("failed to update job: %s", err)
		}
	}
}

func removeJob(t *testing.T, name string) func() {
	return func() {
		client, err := api.NewClient(api.DefaultConfig())
		if err != nil {
			t.Fatalf("failed to instanciate test client: %s", err)
		}
		_, _, err = client.Jobs().Deregister(name, true, nil)
		if err != nil {
			t.Fatalf("failed to remove %s: %s", name, err)
		}
	}
}

const testAccNomadJob = `
resource "nomad_job_v2" "test" {
	job "test-nomad-job-v2" {

	  type = "service"
	  datacenters = ["dc1"]

	  meta = {
		foo = "bar"
	  }

	  group "cache" {

		service {
			tags = ["leader", "mysql"]

			port = "db"

			meta = {
			  meta = "for your service"
			}
		}

		network {
		  port "db" {
			to = 6379
		  }
		}

		task "redis" {
		  driver = "docker"

		  service {
			  tags = ["leader", "mysql"]

			  port = "db"

			  meta = {
				meta = "for your service"
			  }

			  check {
				type     = "http"
				port     = "db"
				path     = "/_healthz"
				interval = "5s"
				timeout  = "2s"
				header = {
				  Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
				}
			  }

		  }

		  config = jsonencode({
			image = "redis:3.2"

			ports = ["db"]
		  })

		  resources {
			cpu    = 500
			memory = 256
		  }
		}
	  }
	}
  }
`

const testAccNomadJobChangeTag = `
resource "nomad_job_v2" "test" {
	job "test-nomad-job-v2" {

	  type = "service"
	  datacenters = ["dc1"]

	  meta = {
		foo = "bar"
	  }

	  group "cache" {

		service {
			tags = ["leader"]

			port = "db"

			meta = {
			  meta = "for your service"
			}
		}

		network {
		  port "db" {
			to = 6379
		  }
		}

		task "redis" {
		  driver = "docker"

		  service {
			  tags = ["leader"]

			  port = "db"

			  meta = {
				meta = "for your service"
			  }

			  check {
				type     = "http"
				port     = "db"
				path     = "/_healthz"
				interval = "5s"
				timeout  = "2s"
				header = {
				  Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
				}
			  }

		  }

		  config = jsonencode({
			image = "redis:3.2"

			ports = ["db"]
		  })

		  resources {
			cpu    = 500
			memory = 256
		  }
		}
	  }
	}
  }
`

const testAccNomadJobChangeId = `
resource "nomad_job_v2" "test" {
	job "test-nomad-job-v2-change-id" {
	  type = "service"
	  datacenters = ["dc1"]


	  group "cache" {

		task "redis" {
		  driver = "docker"

		  config = jsonencode({
			image = "redis:3.2"

			ports = ["db"]
		  })
		}
	  }
	}
  }
`

const testAccNomadJobChangeType = `
resource "nomad_job_v2" "test" {
	job "test-nomad-job-v2-change-id" {
	  type = "system"
	  datacenters = ["dc1"]


	  group "cache" {

		task "redis" {
		  driver = "docker"

		  config = jsonencode({
			image = "redis:3.2"

			ports = ["db"]
		  })
		}
	  }
	}
  }
`
