package nomad

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDatacenters() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDatacentersRead,

		Schema: map[string]*schema.Schema{
			"prefix": {
				Description: "Prefix value used for filtering results.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"ignore_down_nodes": {
				Description: "If enabled, this flag will ignore nodes that are down when listing datacenters.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"datacenters": {
				Description: "The list of datacenters.",
				Computed:    true,
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceDatacentersRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	nodes, _, err := client.Nodes().List(nil)
	if err != nil {
		return fmt.Errorf("failed to query list of nodes: %v", err)
	}

	prefix := d.Get("prefix").(string)
	ignoreDown := d.Get("ignore_down_nodes").(bool)

	d.SetId(resource.UniqueId())
	if err := d.Set("datacenters", filterDatacenters(nodes, prefix, ignoreDown)); err != nil {
		return fmt.Errorf("error setting datacenters: %v", err)
	}

	return nil
}

func filterDatacenters(nodes []*api.NodeListStub, prefix string, ignoreDown bool) []string {
	datacentersSet := make(map[string]struct{})
	datacenters := []string{}

	for _, n := range nodes {
		ignore := n.Status == "down" && ignoreDown
		if ignore || !strings.HasPrefix(n.Datacenter, prefix) {
			continue
		}
		if _, ok := datacentersSet[n.Datacenter]; ok {
			continue
		}

		datacentersSet[n.Datacenter] = struct{}{}
		datacenters = append(datacenters, n.Datacenter)
	}

	// Sort output to keep it stable.
	sort.Strings(datacenters)

	return datacenters
}
