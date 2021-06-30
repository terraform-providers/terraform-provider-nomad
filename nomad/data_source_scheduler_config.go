package nomad

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-nomad/nomad/helper"
)

func dataSourceSchedulerConfig() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceSchedulerConfigRead,

		Schema: map[string]*schema.Schema{
			"memory_oversubscription_enabled": {
				Description: "When true, tasks may exceed their reserved memory limit.",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"scheduler_algorithm": {
				Description: "Specifies whether scheduler binpacks or spreads allocations on available nodes.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"preemption_config": {
				Description: "Options to enable preemption for various schedulers.",
				Computed:    true,
				Type:        schema.TypeMap,
				Elem:        &schema.Schema{Type: schema.TypeBool},
			},
		},
	}
}

func dataSourceSchedulerConfigRead(d *schema.ResourceData, meta interface{}) error {

	client := meta.(ProviderConfig).Client

	schedCfg, _, err := client.Operator().SchedulerGetConfiguration(nil)
	if err != nil {
		return fmt.Errorf("failed to query scheduler config: %v", err)
	}

	// Set a unique ID, as we have nothing else to go on.
	d.SetId(resource.UniqueId())

	premptMap := map[string]bool{
		"batch_scheduler_enabled":   schedCfg.SchedulerConfig.PreemptionConfig.BatchSchedulerEnabled,
		"service_scheduler_enabled": schedCfg.SchedulerConfig.PreemptionConfig.ServiceSchedulerEnabled,
		"system_scheduler_enabled":  schedCfg.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled,
	}

	sw := helper.NewStateWriter(d)
	sw.Set("memory_oversubscription_enabled", schedCfg.SchedulerConfig.MemoryOversubscriptionEnabled)
	sw.Set("scheduler_algorithm", schedCfg.SchedulerConfig.SchedulerAlgorithm)
	sw.Set("preemption_config", premptMap)
	return sw.Error()
}
