package nomad

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDeployments() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDeploymentsRead,
		Schema: map[string]*schema.Schema{

			"deployments": {
				Description: "Deployments",
				Computed:    true,
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeMap},
			},
		},
	}
}

func dataSourceDeploymentsRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	log.Printf("[DEBUG] Getting deployments...")
	deployment_list, _, err := client.Deployments().List(nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			return err
		}

		return fmt.Errorf("error checking for deployments: %#v", err)
	}

	var deployments []map[string]interface{}

	for _, deployment := range deployment_list {
		entry := make(map[string]interface{})
		entry["ID"] = deployment.ID
		entry["JobID"] = deployment.JobID
		entry["JobVersion"] = strconv.Itoa(int(deployment.JobVersion))
		entry["Status"] = deployment.Status
		entry["StatusDescription"] = deployment.StatusDescription
		deployments = append(deployments, entry)
	}

	d.SetId(client.Address() + "/deployments")

	return d.Set("deployments", deployments)
}
