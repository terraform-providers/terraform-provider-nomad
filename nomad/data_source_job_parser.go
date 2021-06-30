package nomad

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceJobParser() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceJobParserRead,
		Schema: map[string]*schema.Schema{
			"hcl": {
				Description: "Specifies the HCL definition of the job encoded in a JSON string.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"canonicalize": {
				Description: "Flag to enable setting any unset fields to their default values.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"json": {
				Description: "The parsed job as JSON string.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func dataSourceJobParserRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	hcl := d.Get("hcl").(string)
	canonicalize := d.Get("canonicalize").(bool)

	log.Printf("[DEBUG] Parsing Job with Canonicalize set to %t", canonicalize)
	job, err := client.Jobs().ParseHCL(hcl, canonicalize)
	if err != nil {
		return fmt.Errorf("error parsing job: %#v", err)
	}

	jobJSON, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("error parsing job: %#v", err)
	}

	jobJSONString := string(jobJSON)

	d.SetId(*job.ID)
	d.Set("hcl", strings.TrimSpace(hcl))
	d.Set("canonicalize", canonicalize)
	d.Set("json", strings.TrimSpace(jobJSONString))

	return nil
}
