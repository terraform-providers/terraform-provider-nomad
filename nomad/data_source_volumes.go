package nomad

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceVolumes() *schema.Resource {
	return &schema.Resource{
		Read: volumesDataSourceRead,

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
			"node_id": {
				Type:        schema.TypeString,
				Description: "Volume node filter",
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"plugin_id": {
				Type:        schema.TypeString,
				Description: "Plugin ID filter",
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"namespace": {
				Description: "Volume namespace filter",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "default",
			},

			"volumes": {
				Description: "Volumes",
				Computed:    true,
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeMap},
			},
		},
	}
}

func volumesDataSourceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	ns := d.Get("namespace").(string)
	if ns == "" {
		ns = "default"
	}
	q := &api.QueryOptions{
		Namespace: ns,
	}
	if v, ok := d.GetOk("node_id"); ok && v != "" {
		q.Params["node_id"] = v.(string)
	}
	if v, ok := d.GetOk("plugin_id"); ok && v != "" {
		q.Params["plugin_id"] = v.(string)
	}

	log.Printf("[DEBUG] Reading list of volumes from Nomad")
	resp, _, err := client.CSIVolumes().List(q)
	if err != nil {
		return fmt.Errorf("error reading volumes from Nomad: %s", err)
	}
	volumes := make([]map[string]interface{}, 0, len(resp))
	for _, v := range resp {
		volume := map[string]interface{}{
			"id":                   v.ID,
			"namespace":            v.Namespace,
			"name":                 v.Name,
			"external_id":          v.ExternalID,
			"access_mode":          v.AccessMode,
			"attachement_mode":     v.AttachmentMode,
			"schedulable":          strconv.FormatBool(v.Schedulable),
			"plugin_id":            v.PluginID,
			"plugin_provider":      v.Provider,
			"controller_required":  strconv.FormatBool(v.ControllerRequired),
			"controllers_healthy":  strconv.Itoa(v.ControllersHealthy),
			"controllers_expected": strconv.Itoa(v.ControllersExpected),
			"nodes_healthy":        strconv.Itoa(v.NodesHealthy),
			"nodes_expected":       strconv.Itoa(v.NodesExpected),
		}
		volumes = append(volumes, volume)
	}
	log.Printf("[DEBUG] Finished reading volumes from Nomad")
	d.SetId(client.Address() + "/v1/volumes")

	return d.Set("volumes", volumes)
}
