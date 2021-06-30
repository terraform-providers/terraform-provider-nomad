package nomad

import (
	"errors"
	"fmt"
	"log"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceSchedulerConfig() *schema.Resource {
	return &schema.Resource{
		Create: resourceSchedulerConfigurationCreate,
		Update: resourceSchedulerConfigurationCreate,
		Delete: resourceSchedulerConfigurationDelete,
		Read:   resourceSchedulerConfigurationRead,

		Schema: map[string]*schema.Schema{
			"memory_oversubscription_enabled": {
				Description: "When true, tasks may exceed their reserved memory limit.",
				Type:        schema.TypeBool,
				Default:     false,
				Optional:    true,
			},
			"scheduler_algorithm": {
				Description: "Specifies whether scheduler binpacks or spreads allocations on available nodes.",
				Type:        schema.TypeString,
				Default:     "binpack",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
					ValidateFunc: validation.StringInSlice([]string{
						"binpack",
						"spread",
					}, false),
				},
			},
			// TODO(jrasell) once the Terraform SDK has been updated within
			//  this provider, we should add validation.MapKeyMatch to this
			//  schema entry using a regex such as:
			//  "("^[service,system,batch]_scheduler_enabled")".
			"preemption_config": {
				Description: "Options to enable preemption for various schedulers.",
				Optional:    true,
				Type:        schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeBool,
				},
			},
		},
	}
}

func resourceSchedulerConfigurationCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	operator := client.Operator()

	config := api.SchedulerConfiguration{
		SchedulerAlgorithm:            api.SchedulerAlgorithm(d.Get("scheduler_algorithm").(string)),
		PreemptionConfig:              api.PreemptionConfig{},
		MemoryOversubscriptionEnabled: d.Get("memory_oversubscription_enabled").(bool),
	}

	// Unpack the preemption block.
	preempt, ok := d.GetOk("preemption_config")
	if ok {
		preemptMap, ok := preempt.(map[string]interface{})
		if !ok {
			return errors.New("failed to unpack preemption configuration block")
		}

		if val, ok := preemptMap["batch_scheduler_enabled"].(bool); ok {
			config.PreemptionConfig.BatchSchedulerEnabled = val
		}
		if val, ok := preemptMap["service_scheduler_enabled"].(bool); ok {
			config.PreemptionConfig.ServiceSchedulerEnabled = val
		}
		if val, ok := preemptMap["system_scheduler_enabled"].(bool); ok {
			config.PreemptionConfig.SystemSchedulerEnabled = val
		}
	}

	// Perform the config write.
	log.Printf("[DEBUG] Upserting Scheduler configuration")
	if _, _, err := operator.SchedulerSetConfiguration(&config, nil); err != nil {
		return fmt.Errorf("error upserting scheduler configuration: %s", err.Error())
	}
	log.Printf("[DEBUG] Upserted scheduler configuration")

	return resourceSchedulerConfigurationRead(d, meta)
}

// resourceSchedulerConfigurationDelete does not do anything:
//
// There is not a correct way to destroy this "resource" nor check it was
// destroyed.
//
// Consider the following:
//   1. an operator manually updates the Nomad scheduler config, or doesn't
//   2. they decide to move management over to Terraform
//   3. they get rid of Terraform management
//
// If the destroy reverts to the Nomad default configuration, we are over
// writing changes based on assumption. We cannot go back in time to the
// initial version as we do not have this information available, so we just
// have to leave it be.
func resourceSchedulerConfigurationDelete(_ *schema.ResourceData, _ interface{}) error { return nil }

func resourceSchedulerConfigurationRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).Client
	operator := client.Operator()

	// The scheduler config doesn't have a UUID, so the resource uses the agent
	// region. Grab this so we can set it later.
	reg, err := client.Agent().Region()
	if err != nil {
		return fmt.Errorf("error getting region: %s", err.Error())
	}
	// retrieve the config
	log.Printf("[DEBUG] Reading scheduler configuration")
	config, _, err := operator.SchedulerGetConfiguration(nil)
	if err != nil {
		return fmt.Errorf("error reading scheduler configuration: %s", err.Error())
	}
	log.Printf("[DEBUG] Read scheduler configuration")

	d.SetId(fmt.Sprintf("nomad-scheduler-configuration-%s", reg))

	// Set the algorithm, handling any error when performing the set.
	if err := d.Set("scheduler_algorithm", config.SchedulerConfig.SchedulerAlgorithm); err != nil {
		return err
	}

	premptMap := map[string]bool{
		"batch_scheduler_enabled":   config.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled,
		"service_scheduler_enabled": config.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled,
		"system_scheduler_enabled":  config.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled,
	}
	return d.Set("preemption_config", premptMap)
}
