package nomad

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceJob() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceJobRead,
		Schema: map[string]*schema.Schema{

			"job_id": {
				Description: "Job ID",
				Type:        schema.TypeString,
				Required:    true,
			},
			"namespace": {
				Description: "Job Namespace",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "default",
			},
			// computed attributes
			"name": {
				Description: "Job Name",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"type": {
				Description: "Job Type",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"version": {
				Description: "Job Version",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"region": {
				Description: "Job Region",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"datacenters": {
				Description: "Job Datacenters",
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"status": {
				Description: "Deployment Status",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"status_description": {
				Description: "Deployment Status Description",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"submit_time": {
				Description: "Job Submit Time",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"create_index": {
				Description: "Create Index",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"modify_index": {
				Description: "Modify Index",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"job_modify_index": {
				Description: "Job Modify Index",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"stop": {
				Description: "Job Stopped",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"priority": {
				Description: "Job Priority",
				Type:        schema.TypeInt,
				Computed:    true,
			},
			"parent_id": {
				Description: "Job Parent ID",
				Computed:    true,
				Type:        schema.TypeString,
			},
			"task_groups": taskGroupSchema(),
			"stable": {
				Description: "Job Stable",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"all_at_once": {
				Description: "Job All At Once",
				Type:        schema.TypeBool,
				Computed:    true,
			},
			"constraints": {
				Description: "Job Constraints",
				Computed:    true,
				Type:        schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ltarget": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"rtarget": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"operand": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"periodic_config": {
				Description: "Job Periodic Configuration",
				Computed:    true,
				Type:        schema.TypeList,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"spec": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"spec_type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"prohibit_overlap": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"timezone": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceJobRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	id := d.Get("job_id").(string)
	ns := d.Get("namespace").(string)
	if ns == "" {
		ns = "default"
	}
	log.Printf("[DEBUG] Getting job status: %q/%q", ns, id)
	job, _, err := client.Jobs().Info(id, &api.QueryOptions{
		Namespace: ns,
	})
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			return err
		}

		return fmt.Errorf("error checking for job: %#v", err)
	}

	d.SetId(*job.ID)
	d.Set("name", job.Name)
	d.Set("type", job.Type)
	d.Set("version", job.Version)
	d.Set("namespace", job.Namespace)
	d.Set("region", job.Region)
	d.Set("datacenters", job.Datacenters)
	d.Set("status", job.Status)
	d.Set("status_description", job.StatusDescription)
	d.Set("submit_time", job.SubmitTime)
	d.Set("create_index", job.CreateIndex)
	d.Set("modify_index", job.ModifyIndex)
	d.Set("job_modify_index", job.JobModifyIndex)
	d.Set("stop", job.Stop)
	d.Set("priority", job.Priority)
	d.Set("parent_id", job.ParentID)
	d.Set("task_groups", jobTaskGroupsRaw(job.TaskGroups))
	d.Set("stable", job.Stable)
	d.Set("all_at_once", job.AllAtOnce)
	d.Set("constraints", job.Constraints)
	if job.Periodic != nil {
		periodic := map[string]interface{}{}
		if job.Periodic.Enabled != nil {
			periodic["enabled"] = *job.Periodic.Enabled
		}
		if job.Periodic.Spec != nil {
			periodic["spec"] = *job.Periodic.Spec
		}
		if job.Periodic.SpecType != nil {
			periodic["spec_type"] = *job.Periodic.SpecType
		}
		if job.Periodic.ProhibitOverlap != nil {
			periodic["prohibit_overlap"] = *job.Periodic.ProhibitOverlap
		}
		if job.Periodic.TimeZone != nil {
			periodic["timezone"] = *job.Periodic.TimeZone
		}
		d.Set("periodic_config", []map[string]interface{}{periodic})
	}

	return nil
}
