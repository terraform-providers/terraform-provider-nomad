package nomad

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceNamespace() *schema.Resource {
	return &schema.Resource{
		Read: namespaceDataSourceRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"quota": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func namespaceDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	name := d.Get("name").(string)
	ns, _, err := client.Namespaces().Info(name, nil)
	if err != nil {
		return fmt.Errorf("Failed to get information about %q: %v", name, err)
	}

	if err = d.Set("description", ns.Description); err != nil {
		return fmt.Errorf("Failed to set 'description': %v", err)
	}
	if err = d.Set("quota", ns.Quota); err != nil {
		return fmt.Errorf("Failed to set 'quota': %v", err)
	}

	d.SetId(client.Address() + "/namespace/" + name)
	return nil
}
