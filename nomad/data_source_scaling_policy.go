package nomad

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-nomad/nomad/helper"
)

func dataSourceScalingPolicy() *schema.Resource {
	return &schema.Resource{
		Read: scalingPolicyDataSourceRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Description: "The scaling policy ID.",
				Type:        schema.TypeString,
				Required:    true,
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
			"min": {
				Description: "The minimum value defined in the scaling policy.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"max": {
				Description: "The maximum value defined in the scaling policy.",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"policy": {
				Description: "The policy defined in the scaling policy as a JSON string.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"target": {
				Description: "The scaling policy target.",
				Type:        schema.TypeMap,
				Computed:    true,
			},
		},
	}
}

func scalingPolicyDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	id := d.Get("id").(string)
	p, _, err := client.Scaling().GetPolicy(id, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to get scaling policy %q: %v", id, err)
	}

	d.SetId(p.ID)

	sw := helper.NewStateWriter(d)
	sw.Set("enabled", p.Enabled)
	sw.Set("type", p.Type)
	sw.Set("min", p.Min)
	sw.Set("max", p.Max)
	sw.Set("target", p.Target)

	policyJSON, err := json.Marshal(p.Policy)
	if err != nil {
		return fmt.Errorf("failed to parse scaling policy %q: %v", id, err)
	}
	sw.Set("policy", string(policyJSON))

	if sw.Error() != nil {
		return sw.Error()
	}

	return nil
}
