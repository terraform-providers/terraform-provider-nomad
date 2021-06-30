package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceScalingPolicies() *schema.Resource {
	return &schema.Resource{
		Read: scalingPoliciesDataSourceRead,

		Schema: map[string]*schema.Schema{
			"job_id": {
				Description: "Job ID to use to filter scaling policies.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"type": {
				Description: "Scaling policy type used to filter scaling policies.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"policies": {
				Description: "The list of policies that match the search criteria.",
				Type:        schema.TypeList,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Description: "The scaling policy ID.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"enabled": {
							Description: "Whether or not the scaling policy is enabled.",
							Type:        schema.TypeBool,
							Computed:    true,
						},
						"type": {
							Description: "The scaling policy type.",
							Type:        schema.TypeString,
							Computed:    true,
						},
						"target": {
							Description: "The scaling policy target.",
							Type:        schema.TypeMap,
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func scalingPoliciesDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	jobID := d.Get("job_id").(string)
	typeQuery := d.Get("type").(string)

	q := &api.QueryOptions{
		Params: map[string]string{
			"job":  jobID,
			"type": typeQuery,
		},
	}
	policies, _, err := client.Scaling().ListPolicies(q)
	if err != nil {
		return fmt.Errorf("failed to query scaling policies: %v", err)
	}

	d.SetId(resource.UniqueId())

	if err := d.Set("policies", flattenScalingPolicies(policies)); err != nil {
		return fmt.Errorf("failed to set policies: %v", err)
	}

	return nil
}

func flattenScalingPolicies(policies []*api.ScalingPolicyListStub) []interface{} {
	out := make([]interface{}, 0, len(policies))

	for _, policy := range policies {
		p := map[string]interface{}{
			"id":      policy.ID,
			"enabled": policy.Enabled,
			"type":    policy.Type,
			"target":  policy.Target,
		}
		out = append(out, p)
	}

	return out
}
