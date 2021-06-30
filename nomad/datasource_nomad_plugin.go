package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePlugin() *schema.Resource {
	return &schema.Resource{
		Read: dataSourcePluginRead,
		Schema: map[string]*schema.Schema{
			"plugin_id": {
				Description: "Plugin ID",
				Type:        schema.TypeString,
				Required:    true,
			},
			"wait_for_registration": {
				Description: "Wait for the plugin to be registered in Noamd",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"wait_for_healthy": {
				Description: "Wait for to be backed by a specified number of controllers",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},

			// computed attributes
			"plugin_provider": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"plugin_provider_version": {
				Computed: true,
				Type:     schema.TypeString,
			},
			"controller_required": {
				Computed: true,
				Type:     schema.TypeBool,
			},
			"controllers_expected": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"controllers_healthy": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"nodes_healthy": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"nodes_expected": {
				Computed: true,
				Type:     schema.TypeInt,
			},
			"nodes": {
				Description: "Available nodes for this plugin",
				Computed:    true,
				Type:        schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"healthy": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"healthy_description": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourcePluginRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	wait := d.Get("wait_for_registration").(bool)
	waitForHealthy := d.Get("wait_for_healthy").(bool)
	if wait || waitForHealthy {
		resource.Retry(d.Timeout(schema.TimeoutRead), func() *resource.RetryError {
			return getPluginInfo(client, d)
		})
	} else {
		err := getPluginInfo(client, d)
		if err != nil {
			return err.Err
		}
	}

	return nil
}

func getPluginInfo(client *api.Client, d *schema.ResourceData) *resource.RetryError {
	id := d.Get("plugin_id").(string)
	waitForHealthy := d.Get("wait_for_healthy").(bool)

	log.Printf("[DEBUG] Getting plugin %q...", id)
	plugin, _, err := client.CSIPlugins().Info(id, nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		log.Printf("[ERROR] error checking for plugin: %#v", err)
		if strings.Contains(err.Error(), "404") {
			return resource.RetryableError(fmt.Errorf("plugin %q not found", id))
		}
		return resource.NonRetryableError(fmt.Errorf("error checking for plugin: %#v", err))
	}

	healthy := plugin.ControllersExpected == plugin.ControllersHealthy &&
		plugin.NodesExpected == plugin.NodesHealthy

	if waitForHealthy && !healthy {
		log.Printf("[DEBUG] plugin %s not yet healthy: %d/%d controllers healthy  %d/%d nodes healthy",
			id,
			plugin.ControllersHealthy, plugin.ControllersExpected,
			plugin.NodesHealthy, plugin.NodesExpected)

		return resource.RetryableError(fmt.Errorf("plugin %s not yet healthy", id))
	}

	d.SetId(plugin.ID)
	d.Set("plugin_id", plugin.ID)
	d.Set("plugin_provider", plugin.Provider)
	d.Set("plugin_provider_version", plugin.Version)
	d.Set("controller_required", plugin.ControllerRequired)
	d.Set("controllers_expected", plugin.ControllersExpected)
	d.Set("controllers_healthy", plugin.ControllersHealthy)
	d.Set("nodes_expected", plugin.NodesExpected)
	d.Set("nodes_healthy", plugin.NodesHealthy)
	nodes := make([]map[string]interface{}, 0, len(plugin.Nodes))
	for name, info := range plugin.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"name":                name,
			"healthy":             info.Healthy,
			"healthy_description": info.HealthDescription,
		})
	}
	d.Set("nodes", nodes)
	return nil
}
