package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceSentinelPolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourceSentinelPolicyWrite,
		Update: resourceSentinelPolicyWrite,
		Delete: resourceSentinelPolicyDelete,
		Read:   resourceSentinelPolicyRead,
		Exists: resourceSentinelPolicyExists,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Unique name for this policy.",
				Required:    true,
				Type:        schema.TypeString,
				ForceNew:    true,
			},

			"description": {
				Description: "Description for this policy.",
				Optional:    true,
				Type:        schema.TypeString,
			},

			"scope": {
				Description:  "Specifies the scope for this policy. Only 'submit-job' is currently supported.",
				Required:     true,
				Type:         schema.TypeString,
				ValidateFunc: validation.StringInSlice([]string{"submit-job"}, false),
			},

			"enforcement_level": {
				Description: "Specifies the enforcement level of the policy.",
				Required:    true,
				Type:        schema.TypeString,
				ValidateFunc: validation.StringInSlice([]string{
					"advisory",
					"hard-mandatory",
					"soft-mandatory",
				}, false),
			},

			"policy": {
				Description: "The Sentinel policy.",
				Required:    true,
				Type:        schema.TypeString,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					// TODO: this should probably parse the AST to avoid false positives
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
		},
	}
}

func resourceSentinelPolicyWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client

	policy := api.SentinelPolicy{
		Name:             d.Get("name").(string),
		Description:      d.Get("description").(string),
		Scope:            d.Get("scope").(string),
		EnforcementLevel: d.Get("enforcement_level").(string),
		Policy:           d.Get("policy").(string),
	}

	log.Printf("[DEBUG] Creating Sentinel policy %q", policy.Name)
	_, err := client.SentinelPolicies().Upsert(&policy, nil)
	if err != nil {
		return fmt.Errorf("error upserting Sentinel policy %q: %s", policy.Name, err.Error())
	}
	log.Printf("[DEBUG] Upserted Sentinel policy %q", policy.Name)
	d.SetId(policy.Name)

	return resourceSentinelPolicyRead(d, meta)
}

func resourceSentinelPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	log.Printf("[DEBUG] Deleting Sentinel policy %q", name)
	_, err := client.SentinelPolicies().Delete(name, nil)
	if err != nil {
		return fmt.Errorf("error deleting Sentinel policy %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Deleted Sentinel policy %q", name)

	return nil
}

func resourceSentinelPolicyRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	name := d.Id()

	log.Printf("[DEBUG] Reading Sentinel policy %q", name)
	policy, _, err := client.SentinelPolicies().Info(name, nil)
	if err != nil {
		// we have Exists, so no need to handle 404
		return fmt.Errorf("error reading Sentinel policy %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Read Sentinel policy %q", name)

	d.Set("name", policy.Name)
	d.Set("description", policy.Description)
	d.Set("scope", policy.Scope)
	d.Set("enforcement_level", policy.EnforcementLevel)
	d.Set("policy", policy.Policy)

	return nil
}

func resourceSentinelPolicyExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(ProviderConfig).Client

	name := d.Id()
	log.Printf("[DEBUG] Checking if Sentinel policy %q exists", name)
	resp, _, err := client.SentinelPolicies().Info(name, nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}

		return true, fmt.Errorf("error checking for Sentinel policy %q: %#v", name, err)
	}
	// just to be safe
	if resp == nil {
		log.Printf("[DEBUG] Resp is nil, assuming Sentinel policy %q doesn't exist", name)
		return false, nil
	}

	return true, nil
}
