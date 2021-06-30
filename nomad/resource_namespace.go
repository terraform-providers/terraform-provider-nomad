package nomad

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNamespace() *schema.Resource {
	return &schema.Resource{
		Create: resourceNamespaceWrite,
		Update: resourceNamespaceWrite,
		Delete: resourceNamespaceDelete,
		Read:   resourceNamespaceRead,
		Exists: resourceNamespaceExists,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Unique name for this namespace.",
				Required:    true,
				Type:        schema.TypeString,
				ForceNew:    true,
			},

			"description": {
				Description: "Description for this namespace.",
				Optional:    true,
				Type:        schema.TypeString,
			},

			"quota": {
				Description: "Quota to set for this namespace.",
				Optional:    true,
				Type:        schema.TypeString,
			},
		},
	}
}

func resourceNamespaceWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	namespace := api.Namespace{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Quota:       d.Get("quota").(string),
	}

	log.Printf("[DEBUG] Upserting namespace %q", namespace.Name)
	_, err := client.Namespaces().Register(&namespace, nil)
	if err != nil {
		return fmt.Errorf("error inserting namespace %q: %s", namespace.Name, err.Error())
	}
	log.Printf("[DEBUG] Created namespace %q", namespace.Name)
	d.SetId(namespace.Name)

	return resourceNamespaceRead(d, meta)
}

func resourceNamespaceDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	log.Printf("[DEBUG] Deleting namespace %q", name)
	retries := 0
	for {
		var err error
		if name == api.DefaultNamespace {
			log.Printf("[DEBUG] Can't delete default namespace, clearing attributes instead")
			d.Set("description", "Default shared namespace")
			d.Set("quota", "")
			err = resourceNamespaceWrite(d, meta)
		} else {
			_, err = client.Namespaces().Delete(name, nil)
		}

		if err == nil {
			break
		} else if retries < 10 {
			if strings.Contains(err.Error(), "has non-terminal jobs") {
				log.Printf("[WARN] could not delete namespace %q because of non-terminal jobs, will pause and retry", name)
				time.Sleep(5 * time.Second)
				retries++
				continue
			}
			return fmt.Errorf("error deleting namespace %q: %s", name, err.Error())
		} else {
			return fmt.Errorf("too many failures attempting to delete namespace %q: %s", name, err.Error())
		}
	}

	if name == api.DefaultNamespace {
		log.Printf("[DEBUG] %s namespace reset", name)
	} else {
		log.Printf("[DEBUG] Deleted namespace %q", name)
	}

	return nil
}

func resourceNamespaceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	log.Printf("[DEBUG] Reading namespace %q", name)
	namespace, _, err := client.Namespaces().Info(name, nil)
	if err != nil {
		// we have Exists, so no need to handle 404
		return fmt.Errorf("error reading namespace %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Read namespace %q", name)

	d.Set("name", namespace.Name)
	d.Set("description", namespace.Description)
	d.Set("quota", namespace.Quota)

	return nil
}

func resourceNamespaceExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(ProviderConfig).Client

	name := d.Id()
	log.Printf("[DEBUG] Checking if namespace %q exists", name)
	resp, _, err := client.Namespaces().Info(name, nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		// there's an open issue to resolve this situation:
		// https://github.com/hashicorp/nomad/issues/1849
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}

		return true, fmt.Errorf("error checking for namespace %q: %#v", name, err)
	}
	if resp == nil {
		// just to be sure
		log.Printf("[DEBUG] Response was nil, namespace %q doesn't exist", name)
		return false, nil
	}

	return true, nil
}
