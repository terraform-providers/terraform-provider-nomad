package nomad

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceNamespaces() *schema.Resource {
	return &schema.Resource{
		Read: namespacesDataSourceRead,

		Schema: map[string]*schema.Schema{
			"namespaces": {
				Type:     schema.TypeList,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Computed: true,
			},
		},
	}
}

func namespacesDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	log.Printf("[DEBUG] Reading namespaces from Nomad")
	resp, _, err := client.Namespaces().List(nil)
	if err != nil {
		return fmt.Errorf("error reading namespaces from Nomad: %s", err)
	}
	namespaces := make([]string, 0, len(resp))
	for _, v := range resp {
		namespaces = append(namespaces, v.Name)
	}
	log.Printf("[DEBUG] Read namespaces from Nomad")
	d.SetId(client.Address() + "/namespaces")

	return d.Set("namespaces", namespaces)
}
