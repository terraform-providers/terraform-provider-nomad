package nomad

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAclPolicies() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAclPoliciesRead,

		Schema: map[string]*schema.Schema{
			"prefix": {
				Description: "ACL Policy Name Prefix",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"policies": {
				Description: "ACL Policies",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Description: "ACL Policy Name",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"description": {
							Description: "ACL Policy Description",
							Type:        schema.TypeString,
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func dataSourceAclPoliciesRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client
	queryOpts := &api.QueryOptions{}
	if v, ok := d.GetOk("prefix"); ok && v.(string) != "" {
		queryOpts.Prefix = v.(string)
	}

	debugMsg := "[DEBUG] Getting ACL Policies"
	if queryOpts.Prefix != "" {
		debugMsg += fmt.Sprintf(" for prefix: %s", queryOpts.Prefix)
	}
	log.Print(debugMsg)

	policies, _, err := client.ACLPolicies().List(queryOpts)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		return fmt.Errorf("error getting ACL policies: %#v", err)
	}

	d.SetId(resource.UniqueId())
	if err := d.Set("policies", flattenAclPolicies(policies)); err != nil {
		return fmt.Errorf("error setting policies: %#v", err)
	}

	return nil
}

func flattenAclPolicies(policies []*api.ACLPolicyListStub) []interface{} {
	output := make([]interface{}, 0, len(policies))
	for _, policy := range policies {
		p := map[string]interface{}{
			"name":        policy.Name,
			"description": policy.Description,
		}
		output = append(output, p)
	}
	return output
}
