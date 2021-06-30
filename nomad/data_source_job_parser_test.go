package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceNomadJobParser_Basic(t *testing.T) {
	resourceName := "data.nomad_job_parser.test_job"

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config: testJobParserConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						resourceName, "hcl", strings.TrimSpace(testDataSourceJobParserHCL)),
					resource.TestCheckResourceAttr(
						resourceName, "canonicalize", "false"),
					checkJobFromString(resourceName, testDataSourceJobParserJSON),
				),
			},
		},
	})
}

func checkJobFromString(resourceName, expectedJson string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState := s.Modules[0].Resources[resourceName]
		if resourceState == nil {
			return errors.New("resource not found in state")
		}
		instanceState := resourceState.Primary
		if instanceState == nil {
			return errors.New("resource has no primary instance")
		}
		actualJson, hadJson := instanceState.Attributes["json"]
		if !hadJson {
			return errors.New("resource had no \"json\" field")
		}
		actualJob := api.Job{}
		expectedJob := api.Job{}
		if err := json.Unmarshal([]byte(expectedJson), &expectedJob); err != nil {
			return errors.New("error parsing expected json")
		}
		if err := json.Unmarshal([]byte(actualJson), &actualJob); err != nil {
			return errors.New("error parsing actual json")
		}
		if !reflect.DeepEqual(actualJob, expectedJob) {
			return errors.New(fmt.Sprintf(
				`jobs are not equal:
expected: %#v
actual: %#v
`, expectedJob, actualJob))
		}
		return nil
	}
}

func TestAccDataSourceNomadJobParser_InvalidHCL(t *testing.T) {
	re := regexp.MustCompile("error parsing job")

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config:      testDataSourceJobParserInvalidHCLConfig,
				ExpectError: re,
			},
		},
	})
}

func TestAccDataSourceNomadJobParser_EmptyHCL(t *testing.T) {
	re := regexp.MustCompile("error parsing job")

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config:      testDataSourceJobParserEmptyHCLConfig,
				ExpectError: re,
			},
		},
	})
}

func TestAccDataSourceNomadJobParser_MissingHCL(t *testing.T) {
	re := regexp.MustCompile(`The argument "hcl" is required, but no definition was found.`)

	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config:      testDataSourceJobParserMissingHCLConfig,
				ExpectError: re,
			},
		},
	})
}

func testJobParserConfig() string {
	return fmt.Sprintf(`
data "nomad_job_parser" "test_job" {
  hcl = <<EOT
%s
EOT
}`, testDataSourceJobParserHCL)

}

const testDataSourceJobParserHCL = `
job "example" {
  datacenters = ["dc1"]

  group "cache" {
    task "redis" {
      driver = "docker"

      config {
        image = "redis:3.2"

        port_map {
          db = 6379
        }
      }

      resources {
        cpu    = 500
        memory = 256

        network {
          mbits = 10
          port "db" {}
        }
      }
    }
  }
}`

const testDataSourceJobParserJSON = `{
  "Stop": null,
  "Region": null,
  "Namespace": null,
  "ID": "example",
  "ParentID": null,
  "Name": "example",
  "Type": null,
  "Priority": null,
  "AllAtOnce": null,
  "Datacenters": [
    "dc1"
  ],
  "Constraints": null,
  "Affinities": null,
  "TaskGroups": [
    {
    "Name": "cache",
    "Count": null,
    "Constraints": null,
    "Affinities": null,
    "Tasks": [
      {
      "Name": "redis",
      "Driver": "docker",
      "User": "",
      "Lifecycle": null,
      "Config": {
        "image": "redis:3.2",
        "port_map": [
        {
          "db": 6379
        }
        ]
      },
      "Constraints": null,
      "Affinities": null,
      "Env": null,
      "ScalingPolicies": null,
      "Services": null,
      "Resources": {
        "CPU": 500,
        "MemoryMB": 256,
        "DiskMB": null,
        "Networks": [
        {
          "Mode": "",
          "Device": "",
          "CIDR": "",
          "IP": "",
          "MBits": 10,
          "DNS": null,
          "ReservedPorts": null,
          "DynamicPorts": [
          {
            "Label": "db",
            "Value": 0,
            "To": 0,
            "HostNetwork": ""
          }
          ]
        }
        ],
        "Devices": null,
        "IOPS": null
      },
      "RestartPolicy": null,
      "Meta": null,
      "KillTimeout": null,
      "LogConfig": null,
      "Artifacts": null,
      "Vault": null,
      "Templates": null,
      "DispatchPayload": null,
      "VolumeMounts": null,
      "Leader": false,
      "ShutdownDelay": 0,
      "KillSignal": "",
      "Kind": ""
      }
    ],
    "Spreads": null,
    "Volumes": null,
    "RestartPolicy": null,
    "ReschedulePolicy": null,
    "EphemeralDisk": null,
    "Update": null,
    "Migrate": null,
    "Networks": null,
    "Meta": null,
    "Services": null,
    "ShutdownDelay": null,
    "StopAfterClientDisconnect": null,
    "Scaling": null
    }
  ],
  "Update": null,
  "Multiregion": null,
  "Spreads": null,
  "Periodic": null,
  "ParameterizedJob": null,
  "Dispatched": false,
  "Payload": null,
  "Reschedule": null,
  "Migrate": null,
  "Meta": null,
  "ConsulToken": null,
  "VaultToken": null,
  "VaultNamespace": null,
  "NomadTokenID": null,
  "Status": null,
  "StatusDescription": null,
  "Stable": null,
  "Version": null,
  "SubmitTime": null,
  "CreateIndex": null,
  "ModifyIndex": null,
  "JobModifyIndex": null
}`

const testDataSourceJobParserInvalidHCLConfig = `
data "nomad_job_parser" "test_job" {
	hcl = "invalid"
}`

const testDataSourceJobParserEmptyHCLConfig = `
data "nomad_job_parser" "test_job" {
	hcl = ""
}`

const testDataSourceJobParserMissingHCLConfig = `
data "nomad_job_parser" "test_job" {
}`
