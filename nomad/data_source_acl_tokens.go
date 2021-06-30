package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceACLTokens() *schema.Resource {
	return &schema.Resource{
		Read: aclTokensDataSourceRead,

		Schema: map[string]*schema.Schema{
			"prefix": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"acl_tokens": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"accessor_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"policies": {
							Type:     schema.TypeList,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"global": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"create_time": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func aclTokensDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	qOpts := &api.QueryOptions{
		Prefix: d.Get("prefix").(string),
	}
	tokens, _, err := client.ACLTokens().List(qOpts)
	if err != nil {
		return fmt.Errorf("error while getting the list of tokens: %v", err)
	}

	result := make([]map[string]interface{}, len(tokens))
	for i, t := range tokens {
		result[i] = map[string]interface{}{
			"accessor_id": t.AccessorID,
			"name":        t.Name,
			"type":        t.Type,
			"policies":    t.Policies,
			"global":      t.Global,
			"create_time": t.CreateTime.String(),
		}
	}

	d.SetId("nomad-tokens")
	return d.Set("acl_tokens", result)
}
