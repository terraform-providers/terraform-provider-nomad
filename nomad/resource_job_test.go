package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceJob_basic(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfig,
				Check:  testResourceJob_initialCheck(t),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo"),
	})
}

func TestResourceJob_service(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfigService,
				Check:  testResourceJob_initialCheck(t),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo-service"),
	})
}

func TestResourceJob_namespace(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfigNamespace,
				Check:  testResourceJob_initialCheckNS(t, "jobresource-test-namespace"),
			},
		},

		CheckDestroy: testResourceJob_checkDestroyNS("foo", "jobresource-test-namespace"),
	})
}

func TestResourceJob_v086(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_v086config,
				Check:  testResourceJob_v086Check,
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foov086"),
	})
}

func TestResourceJob_v090(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_v090config,
				Check:  testResourceJob_v090Check,
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foov086"),
	})
}

func TestResourceJob_volumes(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.10.0-beta1") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_volumesConfig,
				Check:  testResourceJob_volumesCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-volumes"),
	})

}

func TestResourceJob_scalingPolicy(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.11.0-beta1") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_scalingPolicyConfig,
				Check:  testResourceJob_scalingPolicyCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-scaling"),
	})

	// Test Dynamic Application Sizing policies.
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t); testCheckMinVersion(t, "1.0.0-beta2+ent") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_scalingPolicyDASConfig,
				Check:  testResourceJob_scalingPolicyDASCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-scaling-das"),
	})
}

func TestResourceJob_lifecycle(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.11.0-beta1") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_lifecycle,
				Check:  testResourceJob_lifecycleCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-lifecycle"),
	})
}

func TestResourceJob_serviceDeploymentInfo(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_serviceDeploymentInfo,
				Check:  testResourceJob_serviceDeploymentInfoCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-service-with-deployment"),
	})
}

func TestResourceJob_batchNoDetach(t *testing.T) {
	resourceName := "nomad_job.batch_no_detach"
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_batchNoDetach,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "deployment_id", ""),
					resource.TestCheckResourceAttr(resourceName, "deployment_status", ""),
					resource.TestCheckResourceAttrSet(resourceName, "allocation_ids.#"),
				),
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-batch"),
	})
}

func TestResourceJob_serviceWithoutDeployment(t *testing.T) {
	resourceName := "nomad_job.service"
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_serviceNoDeployment,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "deployment_id", ""),
					resource.TestCheckResourceAttr(resourceName, "deployment_status", ""),
				),
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-service-without-deployment"),
	})
}

func TestResourceJob_multiregion(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck: func() {
			testAccPreCheck(t)
			testCheckMinVersion(t, "0.12.0-beta1")
			testEntFeatures(t, "Multiregion Deployments")
		},
		Steps: []r.TestStep{
			{
				Config: testResourceJob_multiregion,
				Check:  testResourceJob_multiregionCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-multiregion"),
	})
}

func TestResourceJob_csiController(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.11.0-beta1") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_csiController,
				Check:  testResourceJob_csiControllerCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-lifecycle"),
	})

}

func TestResourceJob_consulConnect(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck: func() {
			testAccPreCheck(t)
			testCheckConsulEnabled(t)
			testCheckMinVersion(t, "0.10.0-beta1")
		},
		Steps: []r.TestStep{
			{
				Config: testResourceJob_consulConnectConfig,
				Check:  testResourceJob_consulConnectCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-consul-connect"),
	})

	// Test Consul Ingress Gateways.
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "0.12.4") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_consulConnectIngressGatewayConfig,
				Check:  testResourceJob_consulConnectIngressGatewayCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-consul-connect"),
	})

	// Test Consul Terminating Gateways.
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "1.0.4") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_consulConnectTerminatingGatewayConfig,
				Check:  testResourceJob_consulConnectTerminatingGatewayCheck,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-consul-connect"),
	})
}

// TODO: Uncomment once the Terraform Provider SDK is updated to v2.
// func TestResourceJob_consulNamespace(t *testing.T) {
// 	r.Test(t, r.TestCase{
// 		Providers: testProviders,
// 		ExternalProviders: map[string]r.ExternalProvider{
// 			"consul": {},
// 		},
// 		// TODO: check for Consul Enterprise.
// 		PreCheck: func() { testAccPreCheck(t); testCheckEnt(t); testCheckMinVersion(t, "1.1.0-beta1+ent") },
// 		Steps: []r.TestStep{
// 			{
// 				Config: testResourceJob_consulNamespaceConfig,
// 				Check:  testResourceJob_consulNamespaceCheck,
// 			},
// 		},
// 		CheckDestroy: nil,
// 	})
// }

func TestResourceJob_cpuCores(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "1.1.0-beta1") },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_cpuCoresPolicyConfig,
				Check:  testResourceJob_cpuCoresCheck,
			},
		},
	})
}

func TestResourceJob_json(t *testing.T) {
	// Test invalid JSON inputs.
	re := regexp.MustCompile("error parsing jobspec")
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config:      testResourceJob_invalidJSONConfig,
				ExpectError: re,
			},
			{
				Config:      testResourceJob_invalidJSONConfig_notJobspec,
				ExpectError: re,
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo-json"),
	})

	// Test jobspec with "Job" root.
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_jsonConfigWithRoot,
				Check:  testResourceJob_initialCheck(t),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo-json"),
	})

	// Test plain jobspec.
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_jsonConfig,
				Check:  testResourceJob_initialCheck(t),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo-json"),
	})
}

func TestResourceJob_refresh(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfig,
				Check:  testResourceJob_initialCheck(t),
			},

			// This should successfully cause the job to be recreated,
			// testing the Exists function.
			{
				PreConfig: testResourceJob_deregister(t, "foo"),
				Config:    testResourceJob_initialConfig,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo"),
	})
}

func TestResourceJob_disableDestroyDeregister(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			// create the resource
			{
				Config: testResourceJob_noDestroy,
				Check:  testResourceJob_initialCheck(t),
			},
			// "Destroy" with 'deregister_on_destroy = false', check that it wasn't destroyed
			{
				Destroy: true,
				Config:  testResourceJob_noDestroy,
				Check: func(*terraform.State) error {
					providerConfig := testProvider.Meta().(ProviderConfig)
					client := providerConfig.Client
					job, _, err := client.Jobs().Info("foo-nodestroy", nil)
					if err != nil {
						return err
					}
					if *job.Stop == true {
						return fmt.Errorf("job was unexpectedly stopped")
					}
					return nil
				},
			},
		},

		// Somewhat-abuse CheckDestroy to clean up
		CheckDestroy: testResourceJob_forceDestroyWithPurge("foo", "default"),
	})
}

func TestResourceJob_rename(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfig,
				Check:  testResourceJob_initialCheck(t),
			},
			{
				Config: testResourceJob_renameConfig,
				Check: resource.ComposeTestCheckFunc(
					testResourceJob_checkDestroy("foo"),
					testResourceJob_checkExists("bar"),
				),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("bar"),
	})
}

func TestResourceJob_change_namespace(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_initialConfigNamespace,
				Check:  testResourceJob_initialCheckNS(t, "jobresource-test-namespace"),
			},
			{
				Config: testResourceJob_changeNamespaceConfig,
				Check: resource.ComposeTestCheckFunc(
					testResourceJob_checkDestroyNS("foo", "jobresource-test-namespace"),
					testResourceJob_checkExistsNS("foo", "jobresource-updated-namespace"),
				),
			},
		},

		CheckDestroy: resource.ComposeTestCheckFunc(
			testResourceJob_checkDestroyNS("bar", "jobresource-test-namespace"),
			testResourceJob_checkDestroyNS("bar", "jobresource-updated-namespace"),
		),
	})
}

func TestResourceJob_policyOverride(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckEnt(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_policyOverrideConfig(),
				Check:  testResourceJob_initialCheck(t),
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo"),
	})
}

func TestResourceJob_parameterizedJob(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceJob_parameterizedJob,
				Check:  testResourceJob_parameterizedCheck,
			},
		},
	})
}

func TestResourceJob_purgeOnDestroy(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			// create the resource
			{
				Config: testResourceJob_purgeOnDestroy,
				Check:  testResourceJob_initialCheck(t),
			},
			// make sure it is purged once deregistered
			{
				Destroy: true,
				Config:  testResourceJob_purgeOnDestroy,
				Check: func(s *terraform.State) error {
					providerConfig := testProvider.Meta().(ProviderConfig)
					client := providerConfig.Client
					job, _, err := client.Jobs().Info("purge-test", nil)
					if !assert.EqualError(t, err, "Unexpected response code: 404 (job not found)") {
						return fmt.Errorf("Job found: %#v", job)
					}
					return nil
				},
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo"),
	})
}

func testResourceJob_parameterizedCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["nomad_job.parameterized"]
	if resourceState == nil {
		return errors.New("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return errors.New("resource has no primary instance")
	}

	jobID := instanceState.ID

	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	return nil
}

func TestResourceJob_hcl2(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckMinVersion(t, "1.0.0") },
		Steps: []r.TestStep{
			{
				Config:      testResourceJob_hcl2_and_json,
				ExpectError: regexp.MustCompile("json is true and hcl2.enabled is true"),
			},
			{
				Config:      testResourceJob_hcl2_no_fs,
				ExpectError: regexp.MustCompile("filesystem function disabled"),
			},
			{
				Config: testResourceJob_hcl2,
				Check:  testResourceJob_hcl2Check,
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("foo-hcl2"),
	})
}

func testResourceJob_hcl2Check(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["nomad_job.hcl2"]
	if resourceState == nil {
		return errors.New("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return errors.New("resource has no primary instance")
	}

	jobID := instanceState.ID

	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	if diff := cmp.Diff(job.Datacenters, []string{"dc1", "dc2"}); diff != "" {
		return fmt.Errorf("datacenters mismatch (-want +got):\n%s", diff)
	}

	if len(job.TaskGroups) != 1 {
		return fmt.Errorf("wanted 1 group, got %d", len(job.TaskGroups))
	}

	tg := job.TaskGroups[0]
	if len(tg.Tasks) != 1 {
		return fmt.Errorf("wanted 1 task, got %d", len(tg.Tasks))
	}

	if got, want := *tg.RestartPolicy.Attempts, 5; got != want {
		return fmt.Errorf("reschedule -> attempts is %q; want %q", got, want)
	}

	task := tg.Tasks[0]
	if len(task.Templates) != 1 {
		return fmt.Errorf("wanted 1 template, got %d", len(task.Templates))
	}

	tpl := task.Templates[0]
	if tpl.EmbeddedTmpl == nil {
		return fmt.Errorf("template content is nil")
	}
	got := *tpl.EmbeddedTmpl

	want, err := ioutil.ReadFile("./test-fixtures/hello.txt")
	if err != nil {
		return fmt.Errorf("failed to open template data: %v", err)
	}

	if diff := cmp.Diff(string(want), got); diff != "" {
		return fmt.Errorf("template content mismatch (-want +got):\n%s", diff)
	}

	return nil
}

var testResourceJob_parameterizedJob = `
resource "nomad_job" "parameterized" {
	jobspec = <<EOT
		job "parameterized" {
			datacenters = ["dc1"]
			type = "batch"
			parameterized {
				payload = "required"
			}
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

var testResourceJob_initialConfig = `
resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foo" {
			datacenters = ["dc1"]
			type = "service"
			group "foo" {
				task "foo" {
					leader = true ## new in Nomad 0.5.6

					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["10"]
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

var testResourceJob_initialConfigNamespace = `
resource "nomad_namespace" "test-namespace" {
  name = "jobresource-test-namespace"
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foo" {
			datacenters = ["dc1"]
			type = "batch"
			namespace = "${nomad_namespace.test-namespace.name}"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["10"]
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
var testResourceJob_initialConfigService = `
resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foo-service" {
			datacenters = ["dc1"]
			type = "service"
			group "foo" {
				service {
					name = "foo-service"
					port = "8080"
					task = "foo"
					address_mode = "host"

					tags = ["foor", "test", "tf"]
					canary_tags = ["canary"]
					enable_tag_override = false

					meta {
						key = "value"
					}

					canary_meta {
						canary = "true"
					}

					check {
						type = "tcp"
						interval = "10s"
						timeout = "2s"

						address_mode = "host"
						port = "8080"

						initial_status = "passing"
						success_before_passing = 3
						failures_before_critical = 5

						check_restart {
							limit = 3
							grace = "90s"
							ignore_warnings = false
						}
					}

					check {
						type = "script"
						interval = "10s"
						timeout = "2s"

						task = "foo"

						command = "/bin/true"
						args = ["-h"]
					}

					check {
						type = "grpc"
						interval = "10s"
						timeout = "2s"

						task = "foo"

						grpc_service = "foo"
						grpc_use_tls = false
					}

					check {
						type = "http"
						interval = "10s"
						timeout = "2s"

						method = "GET"
						path = "/health"
						protocol = "https"
						tls_skip_verify = true
						header {
							Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
						}
					}
				}

				task "foo" {
					leader = true ## new in Nomad 0.5.6

					service {
						name = "foo-task-service"
						port = "db"
						address_mode = "driver"

						tags = ["foor", "test", "tf"]
						canary_tags = ["canary"]
						enable_tag_override = false

						meta {
							key = "value"
						}

						canary_meta {
							canary = "true"
						}

						check {
							type = "tcp"
							interval = "10s"
							timeout = "2s"
							name = "tcp task check"

							address_mode = "driver"
							port = "8080"

							initial_status = "passing"
							success_before_passing = 3
							failures_before_critical = 5

							check_restart {
								limit = 3
								grace = "90s"
								ignore_warnings = false
							}
						}

						check {
							type = "script"
							interval = "10s"
							timeout = "2s"
							name = "script task check"

							command = "/bin/true"
							args = ["-h"]
						}

						check {
							type = "grpc"
							interval = "10s"
							timeout = "2s"
							name = "grpc task check"

							grpc_service = "foo"
							grpc_use_tls = false
						}

						check {
							type = "http"
							interval = "10s"
							timeout = "2s"
							name = "http task check"

							method = "GET"
							path = "/health"
							protocol = "https"
							tls_skip_verify = true
							header {
								Authorization = ["Basic ZWxhc3RpYzpjaGFuZ2VtZQ=="]
							}
						}
					}

					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["10"]
					}

					resources {
						cpu = 100
						memory = 10
						network {
							port "db" {}
						}
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

var testResourceJob_changeNamespaceConfig = `
resource "nomad_namespace" "test-namespace" {
  name = "jobresource-test-namespace"
}

resource "nomad_namespace" "new-namespace" {
  name = "jobresource-updated-namespace"
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foo" {
			datacenters = ["dc1"]
			type = "batch"
			namespace = "${nomad_namespace.new-namespace.name}"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["10"]
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

var testResourceJob_invalidJSONConfig = `
resource "nomad_job" "test" {
  json = true
  jobspec = "not json"
}
`

var testResourceJob_invalidJSONConfig_notJobspec = `
resource "nomad_job" "test" {
  json = true
  jobspec = <<EOT
{
  "not": "job"
}
EOT
}
`

var testResourceJob_jsonConfigWithRoot = `
resource "nomad_job" "test" {
	json = true
	jobspec = <<EOT
{
  "Job": {
    "Datacenters": [ "dc1" ],
    "ID": "foo-json",
    "Name": "foo-json",
    "Type": "service",
    "TaskGroups": [
      {
        "Name": "foo",
        "Tasks": [{
          "Config": {
            "command": "/bin/sleep",
            "args": [ "1" ]
          },
          "Driver": "raw_exec",
          "Leader": true,
          "LogConfig": {
            "MaxFileSizeMB": 10,
            "MaxFiles": 3
          },
          "Name": "foo",
          "Resources": {
            "CPU": 100,
            "MemoryMB": 10
          }
        }
        ]
      }
    ]
  }
}
EOT
}
`

var testResourceJob_jsonConfig = `
resource "nomad_job" "test" {
	json = true
	jobspec = <<EOT
{
  "Datacenters": [ "dc1" ],
  "ID": "foo-json",
  "Name": "foo-json",
  "Type": "service",
  "TaskGroups": [
    {
      "Name": "foo",
      "Tasks": [{
        "Config": {
          "command": "/bin/sleep",
          "args": [ "1" ]
        },
        "Driver": "raw_exec",
        "Leader": true,
        "LogConfig": {
          "MaxFileSizeMB": 10,
          "MaxFiles": 3
        },
        "Name": "foo",
        "Resources": {
          "CPU": 100,
          "MemoryMB": 10
        }
      }
      ]
    }
  ]
}
EOT
}
`

var testResourceJob_renameConfig = `
resource "nomad_job" "test" {
    jobspec = <<EOT
		job "bar" {
		    datacenters = ["dc1"]
		    type = "service"
		    group "foo" {
		        task "foo" {
		            leader = true ## new in Nomad 0.5.6

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

var testResourceJob_noDestroy = `
resource "nomad_job" "test" {
    deregister_on_destroy = false
    jobspec = <<EOT
		job "foo-nodestroy" {
			datacenters = ["dc1"]
			type = "service"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["30"]
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

var testResourceJob_purgeOnDestroy = `
resource "nomad_job" "test" {
    purge_on_destroy = true
    jobspec = <<EOT
		job "foo" {
			datacenters = ["dc1"]
			type = "service"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/bin/sleep"
						args = ["30"]
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

func testResourceJob_initialCheck(t *testing.T) r.TestCheckFunc {
	return testResourceJob_initialCheckNS(t, "default")
}

func testResourceJob_initialCheckNS(t *testing.T, expectedNamespace string) r.TestCheckFunc {
	return func(s *terraform.State) error {

		resourceState := s.Modules[0].Resources["nomad_job.test"]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}

		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}

		jobID := instanceState.ID

		if setNamespace, ok := instanceState.Attributes["namespace"]; !ok || setNamespace != expectedNamespace {
			return errors.New("resource does not have expected namespace")
		}

		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client
		job, _, err := client.Jobs().Info(jobID, &api.QueryOptions{
			Namespace: expectedNamespace,
		})
		if err != nil {
			return fmt.Errorf("error reading back job: %s", err)
		}

		if got, want := *job.ID, jobID; got != want {
			return fmt.Errorf("jobID is %q; want %q", got, want)
		}

		if got, want := *job.Namespace, expectedNamespace; got != want {
			return fmt.Errorf("job namespace is %q; want %q", got, want)
		}

		wantAllocs, _, err := client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return fmt.Errorf("error reading back job: %s", err)
		}
		wantAllocIds := make([]string, 0, len(wantAllocs))
		for _, a := range wantAllocs {
			wantAllocIds = append(wantAllocIds, a.ID)
		}
		numGotAllocs, _ := strconv.Atoi(instanceState.Attributes["allocation_ids.#"])
		gotAllocs := make([]string, 0, numGotAllocs)
		for i := 0; i < numGotAllocs; i++ {
			id := instanceState.Attributes[fmt.Sprintf("allocation_ids.%d", i)]
			gotAllocs = append(gotAllocs, id)
		}
		if !assert.ElementsMatch(t, gotAllocs, wantAllocIds) {
			return fmt.Errorf("job 'allocation_ids' is '%v'; want '%v'", gotAllocs, wantAllocIds)
		}

		return nil
	}
}

func testResourceJob_v086Check(s *terraform.State) error {

	resourceState := s.Modules[0].Resources["nomad_job.test"]
	if resourceState == nil {
		return errors.New("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return errors.New("resource has no primary instance")
	}

	jobID := instanceState.ID

	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	if len(job.TaskGroups) != 1 {
		return fmt.Errorf("expected a single TaskGroup")
	}
	tg := job.TaskGroups[0]

	// 0.8.x jobs support migrate and update stanzas
	expUpdate := api.UpdateStrategy{}
	json.Unmarshal([]byte(`{
      "Stagger":  		   30000000000,
      "MaxParallel": 2,
      "HealthCheck": "checks",
      "MinHealthyTime":    12000000000,
      "HealthyDeadline":  360000000000,
      "ProgressDeadline": 720000000000,
      "AutoRevert": true,
      "AutoPromote": false,
      "Canary": 1
    }`), &expUpdate)
	if !reflect.DeepEqual(tg.Update, &expUpdate) {
		return fmt.Errorf("job update strategy not as expected")
	}

	expMigrate := api.MigrateStrategy{}
	json.Unmarshal([]byte(`{
      "MaxParallel": 2,
      "HealthCheck": "checks",
      "MinHealthyTime":   12000000000,
      "HealthyDeadline": 360000000000
	}`), &expMigrate)
	if !reflect.DeepEqual(tg.Migrate, &expMigrate) {
		return fmt.Errorf("job migrate strategy not as expected")
	}

	// 0.8.x TaskGroups support reschedule stanza
	expReschedule := api.ReschedulePolicy{}
	json.Unmarshal([]byte(`{
	  "Attempts": 0,
	  "Interval": 7200000000000,
	  "Delay": 	    12000000000,
	  "DelayFunction": "exponential",
	  "MaxDelay":  100000000000,
	  "Unlimited": true
	}`), &expReschedule)
	if !reflect.DeepEqual(tg.ReschedulePolicy, &expReschedule) {
		return fmt.Errorf("job reschedule strategy not as expected")
	}

	if len(tg.Tasks) != 1 {
		return fmt.Errorf("expected a single task in the task group")
	}
	t := tg.Tasks[0]

	// 0.8.x Task service stanza supports canary tags
	if len(t.Services) != 1 {
		return fmt.Errorf("expected task Services stanza with a single element")
	}
	if sv := t.Services[0]; reflect.DeepEqual(sv.CanaryTags, []string{"canary-tag-a"}) != true {
		return fmt.Errorf("expected task canary tags")
	}

	return nil
}

func testResourceJob_v090Check(s *terraform.State) error {

	resourceState := s.Modules[0].Resources["nomad_job.test"]
	if resourceState == nil {
		return errors.New("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return errors.New("resource has no primary instance")
	}

	jobID := instanceState.ID

	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client
	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// 0.9.x jobs support affinity stanzas
	expAffinities := []*api.Affinity{}
	json.Unmarshal([]byte(`[
        {
            "LTarget": "${node.datacenter}",
            "Operand": "=",
            "RTarget": "dc1",
            "Weight": 50
        },
        {
            "LTarget": "${meta.tag}",
            "Operand": "=",
            "RTarget": "foo",
            "Weight": 50
        }
    ]`), &expAffinities)
	if !reflect.DeepEqual(job.Affinities, expAffinities) {
		return fmt.Errorf("job affinities not as expected")
	}

	// 0.9.x jobs support spread stanzas
	expSpreads := []*api.Spread{}
	json.Unmarshal([]byte(`[
        {
            "Attribute": "${node.datacenter}",
            "SpreadTarget": [
                {
                    "Percent": 35,
                    "Value": "dc1"
                },
                {
                    "Percent": 65,
                    "Value": "dc2"
                }
            ],
            "Weight": 80
        }
    ]`), &expSpreads)
	if !reflect.DeepEqual(job.Spreads, expSpreads) {
		return fmt.Errorf("job spreads not as expected")
	}

	// 0.9.2 jobs support auto_promote in the update stanza
	if exp := job.TaskGroups[0].Update.AutoPromote; exp == nil || *exp != true {
		return fmt.Errorf("group auto_promote not as expected")
	}

	return nil
}

func testResourceJob_volumesCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has expected volume declared
	taskGroupName := "foo"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expVolumes := map[string]*api.VolumeRequest{}
	json.Unmarshal([]byte(`{
		"data": {
			"Name": "data",
			"Type": "host",
			"ReadOnly": true,
			"Source": "data"
		}
	}`), &expVolumes)
	if diff := cmp.Diff(expVolumes, taskGroup.Volumes); diff != "" {
		return fmt.Errorf("task group volume mismatch (-want +got):\n%s", diff)
	}

	// check if task has expected volume mount
	taskName := "foo"
	var task *api.Task
	for _, t := range taskGroup.Tasks {
		if t.Name == taskName {
			task = t
			break
		}
	}

	expVolumeMounts := []*api.VolumeMount{}
	json.Unmarshal([]byte(`[
		{
			"Volume": "data",
            "Destination": "/var/lib/data",
            "ReadOnly": true,
			"PropagationMode": "private"
		}
	]`), &expVolumeMounts)
	if diff := cmp.Diff(expVolumeMounts, task.VolumeMounts); diff != "" {
		return fmt.Errorf("task volume mount mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_scalingPolicyCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has expected volume declared
	taskGroupName := "foo"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expScaling := api.ScalingPolicy{}
	json.Unmarshal([]byte(`{
      "Min": 10,
      "Max": 20,
      "Enabled": false,
      "Type": "horizontal",
      "Policy": {
         "opaque": true
      },
      "Target": {
         "Namespace": "default",
  	     "Job": "foo-scaling",
         "Group": "foo"
      }
	}`), &expScaling)

	// ignore the following fields
	taskGroup.Scaling.ID = ""
	taskGroup.Scaling.ModifyIndex = 0
	taskGroup.Scaling.CreateIndex = 0

	if diff := cmp.Diff(expScaling, *taskGroup.Scaling); diff != "" {
		return fmt.Errorf("task group scaling policy mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_scalingPolicyDASCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test_das"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	taskGroupName := "foo"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	taskName := "foo"
	var task *api.Task
	for _, t := range taskGroup.Tasks {
		if t.Name == taskName {
			task = t
			break
		}
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskName)
	}

	scalingType := "vertical_cpu"
	var policy *api.ScalingPolicy
	for _, p := range task.ScalingPolicies {
		if p.Type == scalingType {
			policy = p
			break
		}
	}
	if policy == nil {
		return fmt.Errorf("policy %s not found", scalingType)
	}

	expScaling := &api.ScalingPolicy{}
	err = json.Unmarshal([]byte(`{
      "Min": 10,
      "Max": 20,
      "Enabled": false,
	  "Type": "vertical_cpu",
      "Policy": {
         "opaque": true
      },
      "Target": {
         "Namespace": "default",
         "Job": "foo-scaling-das",
         "Group": "foo",
		 "Task": "foo"
      }
	}`), expScaling)
	if err != nil {
		return err
	}

	// ignore the following fields
	policy.ID = ""
	policy.ModifyIndex = 0
	policy.CreateIndex = 0

	if diff := cmp.Diff(expScaling, policy); diff != "" {
		return fmt.Errorf("task scaling policy mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_serviceDeploymentInfoCheck(s *terraform.State) error {
	resourcePath := "nomad_job.service"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	deployment, _, err := client.Jobs().LatestDeployment(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}
	if deployment == nil {
		return fmt.Errorf("missing latest deployment")
	}

	if got, want := instanceState.Attributes["deployment_id"], deployment.ID; got != want {
		return fmt.Errorf("deployment_info is %q; want %q", got, want)
	}
	if got, want := instanceState.Attributes["deployment_status"], deployment.Status; got != want {
		return fmt.Errorf("deployment_info is %q; want %q", got, want)
	}

	return nil
}

func testResourceJob_lifecycleCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has expected volume declared
	taskGroupName := "foo"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expTaskLifecycle := api.TaskLifecycle{}
	json.Unmarshal([]byte(`{
        "Hook": "prestart",
        "Sidecar": true
	}`), &expTaskLifecycle)

	// merge of group.restart and task.restart
	expTaskRestart := api.RestartPolicy{}
	json.Unmarshal([]byte(`{
        "Interval": 600000000000,
		"Delay": 15000000000,
		"Mode": "delay",
 	    "Attempts": 10
	}`), &expTaskRestart)

	if diff := cmp.Diff(expTaskLifecycle, *taskGroup.Tasks[0].Lifecycle); diff != "" {
		return fmt.Errorf("task lifecycle mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(expTaskRestart, *taskGroup.Tasks[0].RestartPolicy); diff != "" {
		return fmt.Errorf("task restart policy mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_csiControllerCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"
	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has expected volume declared
	taskGroupName := "foo-controller"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expCSIPluginConfig := api.TaskCSIPluginConfig{}
	json.Unmarshal([]byte(`{
        "ID": "aws-ebs0",
        "Type": "controller",
        "MountDir": "/csi"
	}`), &expCSIPluginConfig)
	if taskGroup.Tasks[0].CSIPluginConfig == nil {
		return fmt.Errorf("error; actual CSIPluginConfig was nil")
	}

	if diff := cmp.Diff(expCSIPluginConfig, *taskGroup.Tasks[0].CSIPluginConfig); diff != "" {
		return fmt.Errorf("task csi plugin config mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_consulConnectCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has Service declaration
	taskGroupName := "dashboard"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expServices := []*api.Service{}
	json.Unmarshal([]byte(`[
		{
			"Name": "count-dashboard",
			"PortLabel": "9002",
			"AddressMode": "auto",
			"Connect": {
				"SidecarService": {
					"Tags": ["dashboard", "count"],
					"Proxy": {
						"Upstreams": [
							{
								"DestinationName": "count-api",
								"LocalBindPort": 8080
							}
						]
					}
				}
			},
			"OnUpdate": "require_healthy"
		}
	]`), &expServices)
	if diff := cmp.Diff(expServices, taskGroup.Services); diff != "" {
		return fmt.Errorf("task group services mismatch (-want +got):\n%s", diff)
	}

	// check if task has Consul Connect sidecar proxy
	proxyTaskName := "connect-proxy-count-dashboard"
	var proxyTask *api.Task
	for _, t := range taskGroup.Tasks {
		if t.Name == proxyTaskName {
			proxyTask = t
			break
		}
	}

	if proxyTask == nil {
		return fmt.Errorf("conect proxy task %s not found", proxyTaskName)
	}

	return nil
}

func testResourceJob_consulConnectIngressGatewayCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has Service declaration
	taskGroupName := "ingress-group"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expServices := []*api.Service{}
	err = json.Unmarshal([]byte(`[
		{
			"Name": "ingress-service",
			"PortLabel": "8080",
			"AddressMode": "auto",
			"Connect": {
				"Gateway": {
					"Proxy": {
						"ConnectTimeout": 500000000,
						"EnvoyGatewayBindAddresses": {
							"database": { "Address": "0.0.0.0", "Port": 3306 },
							"web": { "Address": "0.0.0.0", "Port": 8080 }
						},
						"EnvoyGatewayNoDefaultBind": true
					},
					"Ingress": {
						"TLS": {},
						"Listeners": [
							{
								"Port": 8080,
								"Protocol": "tcp",
								"Services": [{ "Name": "web" }]
							},
							{
								"Port": 3306,
								"Protocol": "tcp",
								"Services": [{ "Name": "database" }]
							}
						]
					}
				}
			},
		    "OnUpdate": "require_healthy"
		}
	]`), &expServices)
	if err != nil {
		return fmt.Errorf("failed to parse expected result: %v", err)
	}

	if diff := cmp.Diff(expServices, taskGroup.Services); diff != "" {
		return fmt.Errorf("task group services mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_consulConnectTerminatingGatewayCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test_consul_terminating_gateway"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check if task group has Service declaration
	taskGroupName := "gateway"
	var taskGroup *api.TaskGroup
	for _, tg := range job.TaskGroups {
		if *tg.Name == taskGroupName {
			taskGroup = tg
			break
		}
	}
	if taskGroup == nil {
		return fmt.Errorf("task group %s not found", taskGroupName)
	}

	expServices := []*api.Service{}
	err = json.Unmarshal([]byte(`[
		{
			"Name": "terminating-gateway-service",
			"PortLabel": "connect-terminating-terminating-gateway-service",
			"AddressMode": "auto",
			"Connect": {
				"Gateway": {
					"Proxy": {
						"ConnectTimeout": 5000000000,
						"EnvoyGatewayBindAddresses": {
							"default": { "Address": "0.0.0.0", "Port": -1}
						},
						"EnvoyGatewayNoDefaultBind": true
					},
					"Ingress": null,
					"Terminating": {
						"Services": [
							{ "Name": "api" }
						]
					}
				}
			},
			"OnUpdate": "require_healthy"
		}
	]`), &expServices)
	if err != nil {
		return fmt.Errorf("failed to parse expected result: %v", err)
	}

	if diff := cmp.Diff(expServices, taskGroup.Services); diff != "" {
		return fmt.Errorf("task group services mismatch (-want +got):\n%s", diff)
	}

	return nil
}

func testResourceJob_consulNamespaceCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test_consul_namespace"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	// Find alloc for out test job.
	allocs, _, err := client.Allocations().List(nil)
	if err != nil {
		return err
	}

	var foundAllocID string
	for _, alloc := range allocs {
		if alloc.JobID == jobID {
			foundAllocID = alloc.ID
			break
		}
	}

	foundAlloc, _, err := client.Allocations().Info(foundAllocID, nil)
	if err != nil {
		return err
	}
	if foundAlloc == nil {
		return fmt.Errorf("failed to find alloc for job %q", jobID)
	}

	// Wait for alloc to run.
	retries := 100
	for ; retries > 0; retries-- {
		foundAlloc, _, err = client.Allocations().Info(foundAllocID, nil)
		if err != nil {
			return err
		}

		if foundAlloc.ClientStatus == api.AllocClientStatusRunning {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	if retries == 0 {
		return fmt.Errorf("timeout waiting for alloc %q to run", foundAllocID)
	}

	// Read stdout log and check if key value is present.
	var stdoutString string

	retries = 100
	for ; retries > 0; retries-- {
		stdout, err := client.AllocFS().Cat(foundAlloc, "alloc/logs/sleep.stdout.0", nil)
		if err != nil {
			return err
		}

		stdoutBytes, err := ioutil.ReadAll(stdout)
		if len(stdoutBytes) > 0 {
			stdoutString = string(stdoutBytes)
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	if retries == 0 {
		return fmt.Errorf("timeout waiting for alloc %q stdout logs", foundAllocID)
	}

	if !strings.Contains(stdoutString, "hello") {
		return fmt.Errorf("alloc stdout doesn't have Consul key")
	}

	return nil
}

func testResourceJob_cpuCoresCheck(s *terraform.State) error {
	resourcePath := "nomad_job.test_cpu_cores"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	if len(job.TaskGroups) != 1 {
		return fmt.Errorf("expected %d task groups, got %d", 1, len(job.TaskGroups))
	}

	tg := job.TaskGroups[0]
	if len(tg.Tasks) != 1 {
		return fmt.Errorf("expected %d task in group %q, got %d", 1, *tg.Name, len(tg.Tasks))
	}

	task := tg.Tasks[0]
	if task.Resources.Cores == nil || *task.Resources.Cores != 1 {
		return fmt.Errorf("expected %d cores, got %v", 1, task.Resources.Cores)
	}

	return nil
}

func testResourceJob_multiregionCheck(s *terraform.State) error {
	resourcePath := "nomad_job.multiregion"

	resourceState := s.Modules[0].Resources[resourcePath]
	if resourceState == nil {
		return fmt.Errorf("resource %s not found in state", resourcePath)
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource %s has no primary instance", resourcePath)
	}

	jobID := instanceState.ID
	providerConfig := testProvider.Meta().(ProviderConfig)
	client := providerConfig.Client

	job, _, err := client.Jobs().Info(jobID, nil)
	if err != nil {
		return fmt.Errorf("error reading back job: %s", err)
	}

	if got, want := *job.ID, jobID; got != want {
		return fmt.Errorf("jobID is %q; want %q", got, want)
	}

	// check that job has a multiregion stanza
	if job.Multiregion == nil {
		return fmt.Errorf("multiregion config not found")
	}

	return nil
}

func testResourceJob_checkExistsNS(jobID, ns string) r.TestCheckFunc {
	return func(*terraform.State) error {
		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client
		_, _, err := client.Jobs().Info(jobID, &api.QueryOptions{
			Namespace: ns,
		})
		if err != nil {
			return fmt.Errorf("error reading back job: %s", err)
		}

		return nil
	}
}

func testResourceJob_checkExists(jobID string) r.TestCheckFunc {
	return testResourceJob_checkExistsNS(jobID, "default")
}

func testResourceJob_checkDestroy(jobID string) r.TestCheckFunc {
	return testResourceJob_checkDestroyNS(jobID, "default")
}

func testResourceJob_checkDestroyNS(jobID, ns string) r.TestCheckFunc {
	return func(*terraform.State) error {
		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client

		tries := 0
	TRY:
		for {
			job, _, err := client.Jobs().Info(jobID, &api.QueryOptions{
				Namespace: ns,
			})
			// This should likely never happen because we aren't purging jobs on delete
			if err != nil && strings.Contains(err.Error(), "404") || job == nil {
				return nil
			}

			switch {
			case *job.Status == "dead":
				return nil
			case tries < 5:
				tries++
				time.Sleep(time.Second)
			default:
				break TRY
			}
		}

		return fmt.Errorf("Job %q in namespace %q has not been stopped.", jobID, ns)
	}
}

func testResourceJob_forceDestroyWithPurge(jobID, namespace string) r.TestCheckFunc {
	return func(*terraform.State) error {
		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client
		_, _, err := client.Jobs().Deregister(jobID, true, &api.WriteOptions{
			Namespace: namespace,
		})
		if err != nil {
			return fmt.Errorf("failed to clean up job %q after test: %s", jobID, err)
		}
		return nil
	}
}

func testResourceJob_deregister(t *testing.T, jobID string) func() {
	return func() {
		providerConfig := testProvider.Meta().(ProviderConfig)
		client := providerConfig.Client
		_, _, err := client.Jobs().Deregister(jobID, false, nil)
		if err != nil {
			t.Fatalf("error deregistering job: %s", err)
		}
	}
}

func TestResourceJob_vault(t *testing.T) {
	re, err := regexp.Compile("bad token")
	if err != nil {
		t.Errorf("Error compiling regex: %s", err)
	}
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t); testCheckVaultEnabled(t) },
		Steps: []r.TestStep{
			{
				Config:      testResourceJob_invalidVaultConfig,
				Check:       testResourceJob_initialCheck(t),
				ExpectError: re,
			},
			{
				Config: testResourceJob_validVaultConfig,
				Check:  testResourceJob_initialCheck(t),
			},
		},
		CheckDestroy: testResourceJob_checkDestroy("test"),
	})
}

func TestResourceJob_vaultMultiNamespace(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck: func() {
			testAccPreCheck(t)
			testCheckVaultEnabled(t)
			testEntFeatures(t, "Multi-Vault Namespaces")
		},
		Steps: []r.TestStep{
			{
				Config: testResourceJob_validVaultNamspaceConfig,
				Check:  testResourceJob_initialCheck(t),
			},
		},

		CheckDestroy: testResourceJob_checkDestroy("foo"),
	})
}

func TestResourceJob_serverNotAvailableForPlan(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: testProviders,
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config:             testResourceJob_invalidNomadServerConfig,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestVolumeSorting(t *testing.T) {
	require := require.New(t)

	vols := []*api.VolumeRequest{
		{
			Name:     "red",
			Type:     "host",
			Source:   "/tmp/red",
			ReadOnly: false,
		},
		{
			Name:     "blue",
			Type:     "host",
			Source:   "/tmp/blue",
			ReadOnly: true,
		},
	}
	tgs := []*api.TaskGroup{
		{
			Name: helper.StringToPtr("group-with-volumes"),
			Volumes: map[string]*api.VolumeRequest{
				vols[0].Name: vols[0],
				vols[1].Name: vols[1],
			},
		},
	}
	tg1 := jobTaskGroupsRaw(tgs)
	tgs[0].Volumes = map[string]*api.VolumeRequest{
		vols[1].Name: vols[1],
		vols[0].Name: vols[0],
	}
	tg2 := jobTaskGroupsRaw(tgs)

	require.ElementsMatch(tg1, tg2)
}

var testResourceJob_validVaultConfig = `
provider "nomad" {
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "test" {
			datacenters = ["dc1"]
			type = "batch"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/usr/bin/true"
					}

					resources {
						cpu = 100
						memory = 10
					}

					logs {
						max_files = 3
						max_file_size = 10
					}

					vault {
						policies = ["default"]
					}
				}
			}
		}
	EOT
}
`

var testResourceJob_validVaultNamspaceConfig = `
provider "nomad" {
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "test" {
			datacenters = ["dc1"]
			type = "batch"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/usr/bin/true"
					}

					resources {
						cpu = 100
						memory = 10
					}

					logs {
						max_files = 3
						max_file_size = 10
					}

					vault {
						policies = ["default"]
						namespace = "vault-ns"
					}
				}
			}
		}
	EOT
}
`

var testResourceJob_invalidVaultConfig = `
provider "nomad" {
	vault_token = "bad-token"
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "test" {
			datacenters = ["dc1"]
			type = "batch"
			group "foo" {
				task "foo" {
					leader = true ## new in Nomad 0.5.6

					driver = "raw_exec"
					config {
						command = "/usr/bin/true"
					}

					resources {
						cpu = 100
						memory = 10
					}

					logs {
						max_files = 3
						max_file_size = 10
					}

					vault {
						policies = ["default"]
					}
				}
			}
		}
	EOT
}
`

var testResourceJob_invalidNomadServerConfig = `
provider "nomad" {
	address = "http://invalid.example.com"
}

resource "nomad_job" "test" {
	jobspec = <<EOT
		job "test" {
			datacenters = ["dc1"]
			type = "batch"
			group "foo" {
				task "foo" {
					driver = "raw_exec"
					config {
						command = "/usr/bin/true"
					}
				}
			}
		}
	EOT
}
`

func testResourceJob_policyOverrideConfig() string {
	return fmt.Sprintf(`
resource "nomad_sentinel_policy" "policy" {
  name = "%s"
  policy = "main = rule { false }"
  scope = "submit-job"
  enforcement_level = "soft-mandatory"
  description = "Fail all jobs for testing policy overrides in terraform acctests"
}

resource "nomad_job" "test" {
    depends_on = ["nomad_sentinel_policy.policy"]
    policy_override = true
    jobspec = <<EOT
job "foo" {
    datacenters = ["dc1"]
    type = "service"
    group "foo" {
        task "foo" {
            leader = true ## new in Nomad 0.5.6

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
`, acctest.RandomWithPrefix("tf-nomad-test"))
}

var testResourceJob_v086config = `
resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foov086" {
			datacenters = ["dc1"]
			type = "service"

			migrate {
				max_parallel = 2
				health_check = "checks"
				min_healthy_time = "11s"
				healthy_deadline = "6m"
			}

			update {
			    max_parallel = 2
				min_healthy_time = "11s"
				healthy_deadline = "6m"
				progress_deadline = "11m"
				auto_revert = true
				canary = 1
			}

			reschedule {
				attempts       = 11
				interval       = "2h"
				delay          = "11s"
				delay_function = "exponential"
				max_delay      = "100s"
				unlimited      = false
			}

			group "foo" {

				migrate {
					min_healthy_time = "12s"
				}

				update {
					min_healthy_time = "12s"
					progress_deadline = "12m"
				}

				reschedule {
					attempts       = 0
					delay          = "12s"
					unlimited 	   = true
				}

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

					service {
					  canary_tags = ["canary-tag-a"]
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

var testResourceJob_v090config = `
resource "nomad_job" "test" {
	jobspec = <<EOT
		job "foov090" {
			datacenters = ["dc1"]
			type = "service"

			migrate {
				max_parallel = 2
				health_check = "checks"
				min_healthy_time = "11s"
				healthy_deadline = "6m"
			}

			update {
			    max_parallel = 2
				min_healthy_time = "11s"
				healthy_deadline = "6m"
				progress_deadline = "11m"
				auto_revert = true
				auto_promote = true
				canary = 1
			}

			reschedule {
				attempts       = 11
				interval       = "2h"
				delay          = "11s"
				delay_function = "exponential"
				max_delay      = "100s"
				unlimited      = false
			}

			affinity {
			    attribute = "$${node.datacenter}"
				value = "dc1"
				weight = 50
			}

			affinity {
			    attribute = "$${meta.tag}"
				value = "foo"
				weight = 50
			}

			spread {
				attribute = "$${node.datacenter}"
				target "dc1" { percent = 35 }
				target "dc2" { percent = 65 }
				weight = 80
			}

			group "foo" {

				migrate {
					min_healthy_time = "12s"
				}

				update {
					min_healthy_time = "12s"
					progress_deadline = "12m"
				}

				reschedule {
					attempts       = 0
					delay          = "12s"
					unlimited 	   = true
				}

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

					service {
					  canary_tags = ["canary-tag-a"]
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

var testResourceJob_volumesConfig = `
resource "nomad_job" "test" {
	jobspec = <<EOT
	job "foo-volumes" {
		datacenters = ["dc1"]
		group "foo" {
			volume "data" {
				type = "host"
				read_only = true
				source = "data"
			}

			task "foo" {
				driver = "raw_exec"
				config {
					command = "/bin/sleep"
					args = ["10"]
				}

				volume_mount {
					volume = "data"
					destination = "/var/lib/data"
					read_only = true
					propagation_mode = "private"
				}
			}
		}
	}
	EOT
}
`

var testResourceJob_consulConnectConfig = `
resource "nomad_job" "test" {
    hcl2 {
        enabled = true
    }
	jobspec = <<EOT
job "foo-consul-connect" {

  datacenters = ["dc1"]

  group "api" {
    network {
      mode = "host"
      port "port" {
        static = "9001"
      }
    }

    service {
      name = "count-api"
      port = "port"
    }

    task "api" {
      driver = "docker"

      config {
        image        = "hashicorpnomad/counter-api:v3"
        network_mode = "host"
      }
    }
  }

  group "gateway" {
    network {
      mode = "bridge"
    }

    service {
      name = "api-gateway"

      connect {
        gateway {
          proxy {
          }

          terminating {
            service {
              name = "count-api"
            }
          }
        }
      }
    }
  }

  group "dashboard" {
    network {
      mode = "bridge"

      port "http" {
        static = 9002
        to     = 9002
      }
    }

    service {
      name = "count-dashboard"
      port = "9002"

      connect {
        sidecar_service {
          tags = ["dashboard", "count"]
          proxy {
            upstreams {
              destination_name = "count-api"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    task "dashboard" {
      driver = "docker"

      env {
        COUNTING_SERVICE_URL = "http://$${NOMAD_UPSTREAM_ADDR_count_api}"
      }

      config {
        image = "hashicorpnomad/counter-dashboard:v3"
      }
    }
  }
}
	EOT
}
`

var testResourceJob_consulConnectIngressGatewayConfig = `
resource "nomad_job" "test" {
	jobspec = <<EOT
	job "ingress-example" {
	  datacenters = ["dc1"]

	  group "ingress-group" {
		network {
		  mode = "bridge"
		  port "inbound" {
			static = 8080
		  }
		}

		service {
		  name = "ingress-service"
		  port = "8080"

		  connect {
			gateway {
			  proxy {
				connect_timeout = "500ms"
			  }

			  ingress {
				tls {
				  enabled = false
				}

				listener {
				  port     = 8080
				  protocol = "tcp"
				  service {
					name  = "web"
				  }
				}

				listener {
				  port = 3306
				  protocol = "tcp"
				  service {
					name = "database"
				  }
				}
			  }
			}
		  }
		}
	  }
	}
	EOT
}
`

var testResourceJob_consulConnectTerminatingGatewayConfig = `
resource "nomad_job" "test_consul_terminating_gateway" {
  hcl2 {
    enabled = true
  }

  jobspec = <<EOT
job "terminating-gateway" {
  datacenters = ["dc1"]

  group "gateway" {
    network {
      mode = "bridge"
    }

	service {
	  name = "terminating-gateway-service"

	  connect {
		gateway {
		  proxy {}

		  terminating {
			service {
              name = "api"
			}
		  }
		}
	  }
	}
  }
}
EOT
}
`

var testResourceJob_consulNamespaceConfig = `
resource "consul_namespace" "tf_test_consul_namespace" {
  name = "tf-test-consul-namespace"
}

resource "consul_keys" "tf_test_consul_namespace" {
  namespace = consul_namespace.tf_test_consul_namespace.name

  key {
    path  = "tf_test_consul_namespace"
    value = "hello"
  }
}

resource "nomad_job" "test_consul_namespace" {
  hcl2 {
    enabled = true
  }

  jobspec = <<EOF
job "test-consul-namespace" {
  datacenters = ["dc1"]

  group "sleep" {

    consul {
      namespace = "${consul_namespace.tf_test_consul_namespace.name}"
    }

    task "sleep" {
      driver = "raw_exec"

      config {
        command = "local/script.sh"
      }

      template {
        data        = <<EOT
#!/usr/bin/env bash

echo {{ key "tf_test_consul_namespace" }}
sleep 10
EOT
        destination = "local/script.sh"
      }
    }
  }
}
EOF
}
`

var testResourceJob_cpuCoresPolicyConfig = `
resource "nomad_job" "test_cpu_cores" {
  hcl2 {
    enabled = true
  }

  jobspec = <<EOT
job "test-cpu-cores" {
  datacenters = ["dc1"]

  group "test" {
    task "test" {
      driver = "raw_exec"

	  config {
        command = "/bin/sleep"
        args    = ["10"]
      }

      resources {
        cores = 1
	  }
	}
  }
}
EOT
}
`

var testResourceJob_scalingPolicyConfig = `
resource "nomad_job" "test" {
	jobspec = <<EOT
	job "foo-scaling" {
		datacenters = ["dc1"]
		group "foo" {
            scaling {
                min = 10
                max = 20
                enabled = false
                policy {
                   opaque = true
                }
            }
			task "foo" {
				driver = "raw_exec"
				config {
					command = "/bin/sleep"
					args = ["10"]
				}
			}
		}
	}
	EOT
}
`

var testResourceJob_scalingPolicyDASConfig = `
resource "nomad_job" "test_das" {
	jobspec = <<EOT
	job "foo-scaling-das" {
		datacenters = ["dc1"]
		group "foo" {
			task "foo" {
				driver = "raw_exec"
				config {
					command = "/bin/sleep"
					args = ["10"]
				}
				scaling "cpu" {
					min = 10
					max = 20
					enabled = false
					policy {
					   opaque = true
					}
				}
			}
		}
	}
	EOT
}
`

var testResourceJob_serviceDeploymentInfo = `
resource "nomad_job" "service" {
  detach = false
  jobspec = <<EOT
job "foo-service-with-deployment" {
  type          = "service"
  datacenters   = ["dc1"]
  group "service" {
    update {
      min_healthy_time = "1s"
      healthy_deadline = "2s"
      progress_deadline = "3s"
    }
    task "sleep" {
      driver = "raw_exec"
      config {
        command = "sleep"
        args = ["3600"]
      }
    }
  }
}
EOT
}`

var testResourceJob_serviceNoDeployment = `
resource "nomad_job" "service" {
  detach = false
  jobspec = <<EOT
job "foo-service-without-deployment" {
  type          = "service"
  datacenters   = ["dc1"]
  group "service" {
    update {
      max_parallel = 0
    }
    task "sleep" {
      driver = "raw_exec"
      env {
        version = 2
      }
      config {
        command = "sleep"
        args = ["3600"]
      }
    }
  }
}
EOT
}`

var testResourceJob_batchNoDetach = `
resource "nomad_job" "batch_no_detach" {
  detach = false
  jobspec = <<EOT
job "foo-batch" {
  type          = "batch"
  datacenters   = ["dc1"]
  group "service" {
    task "env" {
      driver = "raw_exec"
      config {
        command = "env"
      }
    }
  }
}
EOT
}`

var testResourceJob_lifecycle = `
resource "nomad_job" "test" {
	jobspec = <<EOT
	job "foo-lifecycle" {
		datacenters = ["dc1"]
		group "foo" {
            restart {
              attempts = 5
              interval = "10m"
              delay    = "15s"
              mode     = "delay"
            }

			task "sidecar" {
				driver = "raw_exec"
				config {
					command = "/bin/sleep"
					args = ["10"]
				}
                restart {
                  attempts = 10
                }
                lifecycle {
                  hook    = "prestart"
                  sidecar = true
                }
			}
		}
	}
	EOT
}
`

var testResourceJob_csiController = `
resource "nomad_job" "test" {
	jobspec = <<EOT
// from https://github.com/hashicorp/nomad/tree/main/e2e/csi/input
job "foo-csi-controller" {
  datacenters = ["dc1"]
  group "foo-controller" {
    stop_after_client_disconnect = "90s"
    task "plugin" {
      driver = "docker"

      config {
        image = "amazon/aws-ebs-csi-driver:latest"

        args = [
          "controller",
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--v=5",
        ]
      }

      csi_plugin {
        id        = "aws-ebs0"
        type      = "controller"
        mount_dir = "/csi"
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
	EOT
}
`

var testResourceJob_multiregion = `
resource "nomad_job" "multiregion" {
	jobspec = <<EOT
job "foo-multiregion" {
  multiregion {
    region "global" {
       datacenters = ["dc1"]
       count = 2
    }
  }
  group "foo" {
    task "foo" {
      driver = "docker"

      config {
        image = "nginx:alpine"
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
	EOT
}
`

var testResourceJob_hcl2 = `
resource "nomad_job" "hcl2" {
  hcl2 {
    enabled  = true
    allow_fs = true
    vars = {
      "restart_attempts" = "5",
      "datacenters"      = "[\"dc1\", \"dc2\"]",
    }
  }

  jobspec = <<EOT
variables {
  args = ["10"]
}

variable "datacenters" {
  type = list(string)
}

variable "restart_attempts" {
  type = number
}

job "foo-hcl2" {
  datacenters = var.datacenters
  group "hcl2" {
    restart {
      attempts = var.restart_attempts
      interval = "10m"
      delay    = "15s"
      mode     = "delay"
    }

    task "sleep" {
      driver = "raw_exec"
      config {
        command = "/bin/sleep"
        args    = var.args
      }
      restart {
        attempts = 10
      }

      template {
        data        = file("./test-fixtures/hello.txt")
        destination = "local/hello.txt"
      }
    }
  }
}
EOT
}
`

var testResourceJob_hcl2_no_fs = `
resource "nomad_job" "hcl2" {
	hcl2 {
	  enabled  = true
	}

	jobspec = <<EOT
variables {
	args = ["10"]
}

job "foo-hcl2" {
	datacenters = ["dc1"]
	group "hcl2" {
		restart {
			attempts = 5
			interval = "10m"
			delay    = "15s"
			mode     = "delay"
		}

		task "sleep" {
			driver = "raw_exec"
			config {
				command = "/bin/sleep"
				args    = var.args
			}
			restart {
				attempts = 10
			}

			template {
			  data        = file("./test-fixtures/hello.txt")
			  destination = "local/hello.txt"
			}
		}
	}
}
EOT
}
`

var testResourceJob_hcl2_and_json = `
resource "nomad_job" "hcl2" {
	hcl2 {
	  enabled  = true
	}

	json    = true
	jobspec = <<EOT
variables {
	args = ["10"]
}

job "foo-hcl2" {
	datacenters = ["dc1"]
	group "hcl2" {
		restart {
			attempts = 5
			interval = "10m"
			delay    = "15s"
			mode     = "delay"
		}

		task "sleep" {
			driver = "raw_exec"
			config {
				command = "/bin/sleep"
				args    = var.args
			}
			restart {
				attempts = 10
			}

			template {
			  data        = file("./test-fixtures/hello.txt")
			  destination = "local/hello.txt"
			}
		}
	}
}
EOT
}
`

func Test_ResourceJob_Parse_ConsulVaultToken(t *testing.T) {
	jobHCL := `
job "example" {
  datacenters = ["dc1"]
  task "example" {
    driver = "docker"
    config {
      image = "alpine"
    }
  }
}
`
	tests := []struct {
		name        string
		vaultToken  *string
		consulToken *string
	}{
		{
			name: "no consul, no vault",
		},
		{
			name:       "vault, no consul",
			vaultToken: helper.StringToPtr("test-vault-token"),
		},
		{
			name:        "consul, no vault",
			consulToken: helper.StringToPtr("test-consul-token"),
		},
		{
			name:        "consul and vault tokens",
			vaultToken:  helper.StringToPtr("test-vault-token"),
			consulToken: helper.StringToPtr("test-consul-token"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseJobspec(jobHCL, JobParserConfig{}, tt.vaultToken, tt.consulToken)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(tt.consulToken, got.ConsulToken))
			require.True(t, reflect.DeepEqual(tt.vaultToken, got.VaultToken))
		})
	}
}
