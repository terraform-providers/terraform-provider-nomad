package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceQuotaSpecification() *schema.Resource {
	return &schema.Resource{
		Create: resourceQuotaSpecificationWrite,
		Update: resourceQuotaSpecificationWrite,
		Delete: resourceQuotaSpecificationDelete,
		Read:   resourceQuotaSpecificationRead,
		Exists: resourceQuotaSpecificationExists,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Unique name for this quota specification.",
				Required:    true,
				Type:        schema.TypeString,
				ForceNew:    true,
			},

			"description": {
				Description: "Description for this quota specification.",
				Optional:    true,
				Type:        schema.TypeString,
			},

			"limits": {
				Description: "Limits encapsulated by this quota specification.",
				Required:    true,
				Type:        schema.TypeSet,
				Elem:        resourceQuotaSpecificationLimits(),
			},
		},
	}
}

func resourceQuotaSpecificationLimits() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"region": {
				Description: "Region in which this limit has affect.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"region_limit": {
				Required:    true,
				Description: "The limit applied to this region.",
				Type:        schema.TypeSet,
				MaxItems:    1,
				Elem:        resourceQuotaSpecificationRegionLimits(),
			},
		},
	}
}

func resourceQuotaSpecificationRegionLimits() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"cpu": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"memory_mb": {
				Type:     schema.TypeInt,
				Optional: true,
			},
		},
	}
}

func resourceQuotaSpecificationWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	spec := api.QuotaSpec{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
	}
	limits, err := expandQuotaLimits(d)
	if err != nil {
		return err
	}
	spec.Limits = limits

	log.Printf("[DEBUG] Upserting quota specification %q", spec.Name)
	_, err = client.Quotas().Register(&spec, nil)
	if err != nil {
		return fmt.Errorf("error upserting quota specification %q: %s", spec.Name, err.Error())
	}
	log.Printf("[DEBUG] Upserted quota specification %q", spec.Name)
	d.SetId(spec.Name)

	return resourceQuotaSpecificationRead(d, meta)
}

func resourceQuotaSpecificationDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	// delete the quota spec
	log.Printf("[DEBUG] Deleting quota specification %q", name)
	_, err := client.Quotas().Delete(name, nil)
	if err != nil {
		return fmt.Errorf("error deleting quota specification %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Deleted quota specification %q", name)

	return nil
}

func resourceQuotaSpecificationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	// retrieve the policy
	log.Printf("[DEBUG] Reading quota specification %q", name)
	spec, _, err := client.Quotas().Info(name, nil)
	if err != nil {
		// we have Exists, so no need to handle 404
		return fmt.Errorf("error reading quota specification %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Read quota specification %q", name)

	d.Set("name", spec.Name)
	d.Set("description", spec.Description)
	err = d.Set("limits", flattenQuotaLimits(spec.Limits))
	if err != nil {
		return fmt.Errorf("error setting quota specification limits for %q: %s", name, err.Error())
	}

	return nil
}

func resourceQuotaSpecificationExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(ProviderConfig).Client

	name := d.Id()
	log.Printf("[DEBUG] Checking if quota specification %q exists", name)
	resp, _, err := client.Quotas().Info(name, nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}

		return true, fmt.Errorf("error checking for quota specification %q: %#v", name, err)
	}
	// just to be safe
	if resp == nil {
		log.Printf("[DEBUG] Response was nil, so assuming quota specification %q doesn't exist", name)
		return false, nil
	}

	return true, nil
}

func flattenQuotaLimits(limits []*api.QuotaLimit) *schema.Set {
	var results []interface{}
	for _, limit := range limits {
		res := map[string]interface{}{
			"region":       limit.Region,
			"region_limit": flattenQuotaRegionLimit(limit.RegionLimit),
		}
		results = append(results, res)
	}
	return schema.NewSet(schema.HashResource(resourceQuotaSpecificationLimits()), results)
}

func flattenQuotaRegionLimit(limit *api.Resources) *schema.Set {
	if limit == nil {
		return nil
	}
	result := map[string]interface{}{}
	if limit.CPU != nil {
		result["cpu"] = *limit.CPU
	}
	if limit.MemoryMB != nil {
		result["memory_mb"] = *limit.MemoryMB
	}
	return schema.NewSet(schema.HashResource(resourceQuotaSpecificationRegionLimits()),
		[]interface{}{result})
}

func expandQuotaLimits(d *schema.ResourceData) ([]*api.QuotaLimit, error) {
	configs := d.Get("limits")
	limits := configs.(*schema.Set).List()
	results := make([]*api.QuotaLimit, 0, len(limits))
	for _, lim := range limits {
		limit, ok := lim.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected limit to be a map[string]interface{}, got %T instead", lim)
		}
		region, ok := limit["region"].(string)
		if !ok {
			return nil, fmt.Errorf("expected region to be string, got %T instead", limit["region"])
		}
		res := &api.QuotaLimit{
			Region: region,
		}
		regLimit, err := expandRegionLimit(limit["region_limit"])
		if err != nil {
			return nil, fmt.Errorf("error parsing region limit for region %q: %s", limit["region"], err.Error())
		}
		res.RegionLimit = regLimit
		results = append(results, res)
	}
	return results, nil
}

func expandRegionLimit(limit interface{}) (*api.Resources, error) {
	regLimits := limit.(*schema.Set).List()
	if len(regLimits) < 1 {
		return nil, nil
	}
	regLimit, ok := regLimits[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map[string]interface{} for region limit, got %T", regLimits[0])
	}
	var res api.Resources
	if cpu, ok := regLimit["cpu"]; ok {
		c, ok := cpu.(int)
		if !ok {
			return nil, fmt.Errorf("expected CPU to be int, got %T", cpu)
		}
		res.CPU = &c
	}
	if mem, ok := regLimit["memory_mb"]; ok {
		m, ok := mem.(int)
		if !ok {
			return nil, fmt.Errorf("expected memory to be int, got %T", mem)
		}
		res.MemoryMB = &m
	}
	return &res, nil
}
