package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAclPolicy() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAclPolicyRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Name",
				Type:        schema.TypeString,
				Required:    true,
			},
			"description": {
				Description: "Description",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"rules": {
				Description: "ACL Rules in HCL format",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func dataSourceAclPolicyRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	name := d.Get("name").(string)

	log.Printf("[DEBUG] Getting ACL Policy %q", name)
	policy, _, err := client.ACLPolicies().Info(name, &api.QueryOptions{})
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			d.SetId("")
			return nil
		}

		return fmt.Errorf("error getting ACL policy: %#v", err)
	}

	d.SetId(policy.Name)
	d.Set("name", policy.Name)
	d.Set("description", policy.Description)
	d.Set("rules", policy.Rules)

	return nil
}
