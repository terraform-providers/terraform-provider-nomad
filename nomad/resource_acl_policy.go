package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceACLPolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourceACLPolicyCreate,
		Update: resourceACLPolicyUpdate,
		Delete: resourceACLPolicyDelete,
		Read:   resourceACLPolicyRead,
		Exists: resourceACLPolicyExists,

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

			"rules_hcl": {
				Description: "HCL or JSON representation of the rules to enforce on this policy. Use file() to specify a file as input.",
				Required:    true,
				Type:        schema.TypeString,
			},
		},
	}
}

func resourceACLPolicyCreate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	policy := api.ACLPolicy{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Rules:       d.Get("rules_hcl").(string),
	}

	// upsert our policy
	log.Printf("[DEBUG] Creating ACL policy %q", policy.Name)
	_, err := client.ACLPolicies().Upsert(&policy, nil)
	if err != nil {
		return fmt.Errorf("error inserting ACLPolicy %q: %s", policy.Name, err.Error())
	}
	log.Printf("[DEBUG] Created ACL policy %q", policy.Name)
	d.SetId(policy.Name)

	return resourceACLPolicyRead(d, meta)
}

func resourceACLPolicyUpdate(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	policy := api.ACLPolicy{
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Rules:       d.Get("rules_hcl").(string),
	}

	// upsert our policy
	log.Printf("[DEBUG] Updating ACL policy %q", policy.Name)
	_, err := client.ACLPolicies().Upsert(&policy, nil)
	if err != nil {
		return fmt.Errorf("error updating ACLPolicy %q: %s", policy.Name, err.Error())
	}
	log.Printf("[DEBUG] Updated ACL policy %q", policy.Name)

	return resourceACLPolicyRead(d, meta)
}

func resourceACLPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client
	name := d.Id()

	// delete the policy
	log.Printf("[DEBUG] Deleting ACL policy %q", name)
	_, err := client.ACLPolicies().Delete(name, nil)
	if err != nil {
		return fmt.Errorf("error deleting ACLPolicy %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Deleted ACL policy %q", name)

	return nil
}

func resourceACLPolicyRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client
	name := d.Id()

	// retrieve the policy
	log.Printf("[DEBUG] Reading ACL policy %q", name)
	policy, _, err := client.ACLPolicies().Info(name, nil)
	if err != nil {
		// we have Exists, so no need to handle 404
		return fmt.Errorf("error reading ACLPolicy %q: %s", name, err.Error())
	}
	log.Printf("[DEBUG] Read ACL policy %q", name)

	d.Set("name", policy.Name)
	d.Set("description", policy.Description)
	d.Set("rules_hcl", policy.Rules)

	return nil
}

func resourceACLPolicyExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	name := d.Id()
	log.Printf("[DEBUG] Checking if ACL policy %q exists", name)
	_, _, err := client.ACLPolicies().Info(name, nil)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			return false, nil
		}

		return true, fmt.Errorf("error checking for ACL policy %q: %#v", name, err)
	}

	return true, nil
}
