package nomad

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourcePlugins() *schema.Resource {
	return &schema.Resource{
		Read: pluginsDataSourceRead,

		Schema: map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Volume Type (currently only 'csi')",
				Default:     "csi",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringInSlice([]string{"csi"}, false),
				},
			},

			"plugins": {
				Description: "Registered plugins",
				Computed:    true,
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeMap},
			},
		},
	}
}

func pluginsDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	log.Printf("[DEBUG] Reading list of dynamic plugins from Nomad")
	resp, _, err := client.CSIPlugins().List(nil)
	if err != nil {
		return fmt.Errorf("error reading plugins from Nomad: %s", err)
	}
	plugins := make([]map[string]interface{}, 0, len(resp))
	for _, p := range resp {
		plugin := map[string]interface{}{
			"id":                   p.ID,
			"provider":             p.Provider,
			"controller_required":  strconv.FormatBool(p.ControllerRequired),
			"controllers_healthy":  strconv.Itoa(p.ControllersHealthy),
			"controllers_expected": strconv.Itoa(p.ControllersExpected),
			"nodes_healthy":        strconv.Itoa(p.NodesHealthy),
			"nodes_expected":       strconv.Itoa(p.NodesExpected),
		}
		plugins = append(plugins, plugin)
	}
	log.Printf("[DEBUG] Finished reading plugins from Nomad")
	d.SetId(client.Address() + "/v1/plugins")

	return d.Set("plugins", plugins)
}
