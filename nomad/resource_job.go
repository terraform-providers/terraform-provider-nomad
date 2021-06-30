package nomad

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/jobspec2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceJob() *schema.Resource {
	return &schema.Resource{
		Create: resourceJobRegister,
		Update: resourceJobRegister,
		Delete: resourceJobDeregister,
		Read:   resourceJobRead,

		CustomizeDiff: resourceJobCustomizeDiff,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"jobspec": {
				Description:      "Job specification. If you want to point to a file use the file() function.",
				Required:         true,
				Type:             schema.TypeString,
				DiffSuppressFunc: jobspecDiffSuppress,
			},

			"policy_override": {
				Description: "Override any soft-mandatory Sentinel policies that fail.",
				Optional:    true,
				Type:        schema.TypeBool,
			},

			"deregister_on_destroy": {
				Description: "If true, the job will be deregistered on destroy.",
				Optional:    true,
				Default:     true,
				Type:        schema.TypeBool,
			},

			"deregister_on_id_change": {
				Description: "If true, the job will be deregistered when the job ID changes.",
				Optional:    true,
				Default:     true,
				Type:        schema.TypeBool,
			},

			"detach": {
				Description: "If true, the provider will return immediately after creating or updating, instead of monitoring.",
				Optional:    true,
				Default:     true,
				Type:        schema.TypeBool,
			},

			"deployment_id": {
				Description: "If detach = false, the ID for the deployment associated with the last job create/update, if one exists.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"deployment_status": {
				Description: "If detach = false, the status for the deployment associated with the last job create/update, if one exists.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"hcl2": {
				Description: "Configuration for the HCL2 jobspec parser.",
				Optional:    true,
				Type:        schema.TypeList,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": {
							Description: "If true, the `jobspec` will be parsed as HCL2 instead of HCL.",
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
						},
						"allow_fs": {
							Description: "If true, HCL2 file system functions will be enabled when parsing the `jobspec`.",
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
						},
						"vars": {
							Description: "Additional variables to use when templating the job with HCL2",
							Type:        schema.TypeMap,
							Optional:    true,
						},
					},
				},
			},

			"json": {
				Description: "If true, the `jobspec` will be parsed as json instead of HCL.",
				Optional:    true,
				Type:        schema.TypeBool,
			},

			"modify_index": {
				Description: "Integer that increments for each change. Used to detect any changes between plan and apply.",
				Computed:    true,
				Type:        schema.TypeString, // it's an int64, so won't fit in our TypeInt
			},

			"name": {
				Description: "The name of the job, as derived from the jobspec.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"namespace": {
				Description: "The namespace of the job, as derived from the jobspec.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"type": {
				Description: "The type of the job, as derived from the jobspec.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"region": {
				Description: "The target region for the job, as derived from the jobspec.",
				Computed:    true,
				Type:        schema.TypeString,
			},

			"datacenters": {
				Description: "The target datacenters for the job, as derived from the jobspec.",
				Computed:    true,
				Type:        schema.TypeSet,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"allocation_ids": {
				Description: "The IDs for allocations associated with this job.",
				Computed:    true,
				Type:        schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},

			"task_groups": taskGroupSchema(),

			"purge_on_destroy": {
				Description: "Whether to purge the job when the resource is destroyed.",
				Optional:    true,
				Type:        schema.TypeBool,
			},
		},
	}
}

const (
	MonitoringEvaluation = "monitoring_evaluation"
	EvaluationComplete   = "evaluation_complete"
	MonitoringDeployment = "monitoring_deployment"
	DeploymentSuccessful = "deployment_successful"
)

func taskGroupSchema() *schema.Schema {
	return &schema.Schema{
		Computed: true,
		Type:     schema.TypeList,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Computed: true,
					Type:     schema.TypeString,
				},
				"count": {
					Computed: true,
					Type:     schema.TypeInt,
				},
				// "scaling": {
				// 	Computed: true,
				// 	Type:     schema.TypeList,
				// 	MinItems: 0,
				// 	MaxItems: 1,
				// 	Elem:     scalingPolicySchema(),
				// },
				"task": {
					Computed: true,
					Type:     schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Computed: true,
								Type:     schema.TypeString,
							},
							"driver": {
								Computed: true,
								Type:     schema.TypeString,
							},
							"meta": {
								Computed: true,
								Type:     schema.TypeMap,
							},
							// "scaling": {
							// 	Computed: true,
							// 	Type:     schema.TypeList,
							// 	Elem:     scalingPolicySchema(),
							// },
							"volume_mounts": {
								Computed: true,
								Type:     schema.TypeList,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"volume": {
											Computed: true,
											Type:     schema.TypeString,
										},
										"destination": {
											Computed: true,
											Type:     schema.TypeString,
										},
										"read_only": {
											Computed: true,
											Type:     schema.TypeBool,
										},
									},
								},
							},
						},
					},
				},
				"volumes": {
					Computed: true,
					Type:     schema.TypeList,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Computed: true,
								Type:     schema.TypeString,
							},
							"type": {
								Computed: true,
								Type:     schema.TypeString,
							},
							"read_only": {
								Computed: true,
								Type:     schema.TypeBool,
							},
							"source": {
								Computed: true,
								Type:     schema.TypeString,
							},
						},
					},
				},
				"meta": {
					Computed: true,
					Type:     schema.TypeMap,
				},
			},
		},
	}
}

// JobParserConfig stores configuration options for how to parse the jobspec.
type JobParserConfig struct {
	JSON JSONJobParserConfig
	HCL2 HCL2JobParserConfig
}

// JSONJobParserConfig stores configuration options for the JSON jobspec parser.
type JSONJobParserConfig struct {
	Enabled bool
}

// HCL2JobParserConfig stores configuration options for the HCL2 jobspec parser.
type HCL2JobParserConfig struct {
	Enabled bool
	AllowFS bool
	Vars    map[string]string
}

// ResourceFieldGetter are able to retrieve field values.
// Examples: *schema.ResourceData and *schema.ResourceDiff
type ResourceFieldGetter interface {
	Get(string) interface{}
}

func resourceJobRegister(d *schema.ResourceData, meta interface{}) error {
	timeout := d.Timeout(schema.TimeoutCreate)
	if !d.IsNewResource() {
		timeout = d.Timeout(schema.TimeoutUpdate)
	}

	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	// Get the jobspec itself.
	jobspecRaw := d.Get("jobspec").(string)

	// Read job parsing config.
	jobParserConfig, err := parseJobParserConfig(d)
	if err != nil {
		return err
	}

	// Parse jobspec.
	job, err := parseJobspec(jobspecRaw, jobParserConfig, providerConfig.vaultToken, providerConfig.consulToken)
	if err != nil {
		return err
	}

	if job.Namespace == nil || *job.Namespace == "" {
		defaultNamespace := "default"
		job.Namespace = &defaultNamespace
	}

	// Register the job
	wantModifyIndexStrI, _ := d.GetChange("modify_index")
	wantModifyIndex, err := strconv.ParseUint(wantModifyIndexStrI.(string), 10, 64)
	if err != nil {
		wantModifyIndex = 0
	}

	resp, _, err := client.Jobs().RegisterOpts(job, &api.RegisterOptions{
		PolicyOverride: d.Get("policy_override").(bool),
		ModifyIndex:    wantModifyIndex,
	}, nil)
	if err != nil {
		return fmt.Errorf("error applying jobspec: %s", err)
	}

	log.Printf("[DEBUG] job '%s' registered in namespace '%s'", *job.ID, *job.Namespace)
	d.SetId(*job.ID)
	d.Set("name", job.ID)
	d.Set("namespace", job.Namespace)
	d.Set("modify_index", strconv.FormatUint(resp.JobModifyIndex, 10))

	if d.Get("detach") == false && resp.EvalID != "" {
		log.Printf("[DEBUG] will monitor scheduling/deployment of job '%s'", *job.ID)
		deployment, err := monitorDeployment(client, timeout, resp.EvalID)
		if err != nil {
			return fmt.Errorf(
				"error waiting for job '%s' to schedule/deploy successfully: %s",
				*job.ID, err)
		}
		if deployment != nil {
			d.Set("deployment_id", deployment.ID)
			d.Set("deployment_status", deployment.Status)
		} else {
			d.Set("deployment_id", nil)
			d.Set("deployment_status", nil)
		}
	}

	return resourceJobRead(d, meta) // populate other computed attributes
}

// monitorDeployment monitors the evalution(s) from a job create/update and,
// if they result in a deployment, monitors that deployment until completion.
func monitorDeployment(client *api.Client, timeout time.Duration, initialEvalID string) (*api.Deployment, error) {

	stateConf := &resource.StateChangeConf{
		Pending:    []string{MonitoringEvaluation},
		Target:     []string{EvaluationComplete},
		Refresh:    evaluationStateRefreshFunc(client, initialEvalID),
		Timeout:    timeout,
		Delay:      0,
		MinTimeout: 3 * time.Second,
	}

	state, err := stateConf.WaitForState()
	if err != nil {
		return nil, fmt.Errorf("error waiting for evaluation: %s", err)
	}

	evaluation := state.(*api.Evaluation)
	if evaluation.DeploymentID == "" {
		log.Printf("[WARN] job has been scheduled, but there is no deployment to monitor")
		return nil, nil
	}

	stateConf = &resource.StateChangeConf{
		Pending:    []string{MonitoringDeployment},
		Target:     []string{DeploymentSuccessful},
		Refresh:    deploymentStateRefreshFunc(client, evaluation.DeploymentID),
		Timeout:    timeout,
		Delay:      0,
		MinTimeout: 5 * time.Second,
	}

	state, err = stateConf.WaitForState()
	if err != nil {
		return nil, fmt.Errorf("error waiting for evaluation: %s", err)
	}
	return state.(*api.Deployment), nil
}

// evaluationStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch
// the evaluation(s) from a job create/update
func evaluationStateRefreshFunc(client *api.Client, initialEvalID string) resource.StateRefreshFunc {

	// evalID is the evaluation that we are currently monitoring. This will change
	// along with follow-up evaluations.
	evalID := initialEvalID

	return func() (interface{}, string, error) {
		// monitor the eval
		log.Printf("[DEBUG] monitoring evaluation '%s'", evalID)
		eval, _, err := client.Evaluations().Info(evalID, nil)
		if err != nil {
			log.Printf("[ERROR] error on Evaluation.Info during deploymentStateRefresh: %s", err)
			return nil, "", err
		}

		var state string
		switch eval.Status {
		case "complete":
			// Monitor the next eval in the chain, if present
			log.Printf("[DEBUG] evaluation '%v' complete", eval.ID)
			if eval.NextEval != "" {
				log.Printf("[DEBUG] will monitor follow-up eval '%v'", eval.ID)
				evalID = eval.NextEval
				state = MonitoringEvaluation
			} else {
				state = EvaluationComplete
			}
		case "failed", "cancelled":
			return nil, "", fmt.Errorf("evaluation failed: %v", eval.StatusDescription)
		default:
			state = MonitoringEvaluation
		}
		return eval, state, nil
	}
}

// deploymentStateRefreshFunc returns a resource.StateRefreshFunc that is used to watch
// the deployment from a job create/update
func deploymentStateRefreshFunc(client *api.Client, deploymentID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		// monitor the deployment
		var state string
		deployment, _, err := client.Deployments().Info(deploymentID, nil)
		if err != nil {
			log.Printf("[ERROR] error on Deployment.Info during deploymentStateRefresh: %s", err)
			return nil, "", err
		}
		switch deployment.Status {
		case "successful":
			log.Printf("[DEBUG] deployment '%s' successful", deployment.ID)
			state = DeploymentSuccessful
		case "failed", "cancelled":
			log.Printf("[DEBUG] deployment unsuccessful: %s", deployment.StatusDescription)
			return deployment, "",
				fmt.Errorf("deployment '%s' terminated with status '%s': '%s'",
					deployment.ID, deployment.Status, deployment.StatusDescription)
		default:
			// don't overwhelm the API server
			state = MonitoringDeployment
		}
		return deployment, state, nil
	}
}

func resourceJobDeregister(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	// If deregistration is disabled, then do nothing
	deregister_on_destroy := d.Get("deregister_on_destroy").(bool)
	if !deregister_on_destroy {
		log.Printf(
			"[WARN] job %q will not deregister since "+
				"'deregister_on_destroy' is %t", d.Id(), deregister_on_destroy)
		return nil
	}

	id := d.Id()
	log.Printf("[DEBUG] deregistering job: %q", id)
	opts := &api.WriteOptions{
		Namespace: d.Get("namespace").(string),
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	purge := d.Get("purge_on_destroy").(bool)
	_, _, err := client.Jobs().Deregister(id, purge, opts)
	if err != nil {
		return fmt.Errorf("error deregistering job: %s", err)
	}

	return nil
}

func resourceJobRead(d *schema.ResourceData, meta interface{}) error {
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	id := d.Id()
	opts := &api.QueryOptions{
		Namespace: d.Get("namespace").(string),
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	log.Printf("[DEBUG] reading information for job %q in namespace %q", id, opts.Namespace)
	job, _, err := client.Jobs().Info(id, opts)
	if err != nil {
		// As of Nomad 0.4.1, the API client returns an error for 404
		// rather than a nil result, so we must check this way.
		if strings.Contains(err.Error(), "404") {
			log.Printf("[DEBUG] job %q does not exist, so removing", id)
			d.SetId("")
			return nil
		}

		return fmt.Errorf("error checking for job: %s", err)
	}
	log.Printf("[DEBUG] found job %q in namespace %q", *job.Name, *job.Namespace)

	allocStubs, _, err := client.Jobs().Allocations(id, false, nil)
	if err != nil {
		log.Printf("[WARN] error listing allocations for Job %q, will return empty list", id)
	}
	allocIDs := make([]string, 0, len(allocStubs))
	for _, a := range allocStubs {
		allocIDs = append(allocIDs, a.ID)
	}

	d.Set("name", job.ID)
	d.Set("type", job.Type)
	d.Set("region", job.Region)
	d.Set("datacenters", job.Datacenters)
	d.Set("task_groups", jobTaskGroupsRaw(job.TaskGroups))
	d.Set("allocation_ids", allocIDs)
	d.Set("namespace", job.Namespace)
	if job.JobModifyIndex != nil {
		d.Set("modify_index", strconv.FormatUint(*job.JobModifyIndex, 10))
	} else {
		d.Set("modify_index", "0")
	}

	return nil
}

func resourceJobCustomizeDiff(_ context.Context, d *schema.ResourceDiff, meta interface{}) error {
	log.Printf("[DEBUG] resourceJobCustomizeDiff")
	providerConfig := meta.(ProviderConfig)
	client := providerConfig.Client

	if !d.NewValueKnown("jobspec") {
		d.SetNewComputed("name")
		d.SetNewComputed("modify_index")
		d.SetNewComputed("namespace")
		d.SetNewComputed("type")
		d.SetNewComputed("region")
		d.SetNewComputed("datacenters")
		d.SetNewComputed("allocation_ids")
		d.SetNewComputed("task_groups")
		d.SetNewComputed("deployment_id")
		d.SetNewComputed("deployment_status")
		return nil
	}

	oldSpecRaw, newSpecRaw := d.GetChange("jobspec")

	if oldSpecRaw.(string) == newSpecRaw.(string) {
		// nothing to do!
		return nil
	}

	// Read job parsing config.
	jobParserConfig, err := parseJobParserConfig(d)
	if err != nil {
		return err
	}

	// Parse jobspec
	// Catch syntax errors client-side during plan
	job, err := parseJobspec(newSpecRaw.(string), jobParserConfig, providerConfig.vaultToken, providerConfig.consulToken)
	if err != nil {
		return err
	}

	defaultNamespace := "default"
	if job.Namespace == nil || *job.Namespace == "" {
		job.Namespace = &defaultNamespace
	}

	resp, _, err := client.Jobs().PlanOpts(job, &api.PlanOptions{
		Diff:           false,
		PolicyOverride: d.Get("policy_override").(bool),
	}, nil)
	if err != nil {
		log.Printf("[WARN] failed to validate Nomad plan: %s", err)
	}

	// If we were able to successfully plan then we can safely populate our
	// diff with new values based on the job object we got from parsing,
	// causing the Terraform diff to correctly reflect the planned changes
	// to the subset of job attributes we include in our schema.

	d.SetNew("name", job.ID)
	d.SetNew("type", job.Type)
	d.SetNew("region", job.Region)
	d.SetNew("datacenters", job.Datacenters)

	// If the identity has changed and the config asks us to deregister on identity
	// change then the id field "forces new resource".
	if d.Get("namespace").(string) != *job.Namespace {
		log.Printf("[DEBUG] namespace change forces new resource")
		d.SetNew("namespace", job.Namespace)
		d.ForceNew("namespace")
	} else if d.Id() != *job.ID {
		if d.Get("deregister_on_id_change").(bool) {
			log.Printf("[DEBUG] name change forces new resource because deregister_on_id_change is set")
			d.ForceNew("id")
			d.ForceNew("name")
		} else {
			log.Printf("[DEBUG] allowing name change as update because deregister_on_id_change is not set")
		}
	} else {
		d.SetNew("namespace", job.Namespace)

		// If the identity (namespace+name) _isn't_ changing, then we require consistency of the
		// job modify index to ensure that the "old" part of our diff
		// will show what Nomad currently knows.
		wantModifyIndexStr := d.Get("modify_index").(string)
		wantModifyIndex, err := strconv.ParseUint(wantModifyIndexStr, 10, 64)
		if err != nil {
			// should never happen, because we always write with FormatUint
			// in Read above.
			return fmt.Errorf("invalid modify_index in state: %s", err)
		}

		if resp != nil && resp.JobModifyIndex != wantModifyIndex {
			// Should rarely happen, but might happen if there was a concurrent
			// other process writing to Nomad since our Read call.
			return fmt.Errorf("job modify index has changed since last refresh")
		}
	}

	// We know that applying changes here will change the modify index
	// _somehow_, but we won't know how much it will increment until
	// after we complete registration.
	d.SetNewComputed("modify_index")
	// similarly, we won't know the allocation ids until after the job registration eval
	d.SetNewComputed("allocation_ids")

	d.SetNew("task_groups", jobTaskGroupsRaw(job.TaskGroups))

	return nil
}

func parseJobParserConfig(d ResourceFieldGetter) (JobParserConfig, error) {
	config := JobParserConfig{}

	// Read JSON parser configuration.
	jsonEnabled := d.Get("json").(bool)
	config.JSON = JSONJobParserConfig{
		Enabled: jsonEnabled,
	}

	// Read HCL2 parser configuration.
	hcl2Config, err := parseHCL2JobParserConfig(d.Get("hcl2"))
	if err != nil {
		return config, err
	}
	config.HCL2 = hcl2Config

	// JSON and HCL2 parsing are conflicting options.
	if config.JSON.Enabled && config.HCL2.Enabled {
		return config, fmt.Errorf("invalid combination. json is %t and hcl2.enabled is %t", config.JSON.Enabled, config.HCL2.Enabled)
	}

	return config, nil
}

func parseHCL2JobParserConfig(raw interface{}) (HCL2JobParserConfig, error) {
	config := HCL2JobParserConfig{}

	// `hcl2` must be a list with only one element.
	hcl2List, ok := raw.([]interface{})
	if !ok || len(hcl2List) > 1 {
		return config, fmt.Errorf("failed to unpack hcl2 configuration block")
	}

	// If the list is empty, it means we don't have a `hcl2` block.
	if len(hcl2List) == 0 {
		return config, nil
	}

	// The only element in the list must be a map.
	hcl2Map, ok := hcl2List[0].(map[string]interface{})
	if !ok {
		return config, nil
	}

	// Read map fields into parser config struct.
	if enabled, ok := hcl2Map["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	if allowFS, ok := hcl2Map["allow_fs"].(bool); ok {
		config.AllowFS = allowFS
	}
	if vars, ok := hcl2Map["vars"].(map[string]interface{}); ok {
		config.Vars = make(map[string]string)
		for k, v := range vars {
			config.Vars[k] = v.(string)
		}
	}

	return config, nil
}

func parseJobspec(raw string, config JobParserConfig, vaultToken *string, consulToken *string) (*api.Job, error) {
	var job *api.Job
	var err error

	switch {
	case config.JSON.Enabled:
		job, err = parseJSONJobspec(raw)
	case config.HCL2.Enabled:
		job, err = parseHCL2Jobspec(raw, config.HCL2)
	default:
		job, err = jobspec.Parse(strings.NewReader(raw))
	}

	if err != nil {
		return nil, fmt.Errorf("error parsing jobspec: %s", err)
	}

	// If job is empty after parsing, the input is not a valid Nomad job.
	if job == nil || reflect.DeepEqual(job, &api.Job{}) {
		return nil, fmt.Errorf("error parsing jobspec: input JSON is not a valid Nomad jobspec")
	}

	// Inject the Vault and Consul tokens
	job.VaultToken = vaultToken
	job.ConsulToken = consulToken

	return job, nil
}

func parseJSONJobspec(raw string) (*api.Job, error) {
	// `nomad job run -output` returns a jobspec with a "Job" root, so
	// partially parse the input JSON to detect if we have this root.
	var root map[string]json.RawMessage

	err := json.Unmarshal([]byte(raw), &root)
	if err != nil {
		return nil, err
	}

	jobBytes, ok := root["Job"]
	if !ok {
		// Parse the input as is if there's no "Job" root.
		jobBytes = []byte(raw)
	}

	// Parse actual job.
	var job api.Job
	err = json.Unmarshal(jobBytes, &job)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func parseHCL2Jobspec(raw string, config HCL2JobParserConfig) (*api.Job, error) {
	argVars := []string{}
	for k, v := range config.Vars {
		argVars = append(argVars, fmt.Sprintf("%s=%s", k, v))
	}

	return jobspec2.ParseWithConfig(&jobspec2.ParseConfig{
		Path:    "",
		Body:    []byte(raw),
		AllowFS: config.AllowFS,
		ArgVars: argVars,
		Strict:  true,
	})
}

func jobTaskGroupsRaw(tgs []*api.TaskGroup) []interface{} {
	ret := make([]interface{}, 0, len(tgs))

	for _, tg := range tgs {
		tgM := make(map[string]interface{})

		if tg.Name != nil {
			tgM["name"] = *tg.Name
		} else {
			tgM["name"] = ""
		}
		if tg.Count != nil {
			tgM["count"] = *tg.Count
		} else {
			tgM["count"] = 1
		}
		if tg.Meta != nil {
			tgM["meta"] = tg.Meta
		} else {
			tgM["meta"] = make(map[string]interface{})
		}

		tasksI := make([]interface{}, 0, len(tg.Tasks))
		for _, task := range tg.Tasks {
			taskM := make(map[string]interface{})

			taskM["name"] = task.Name
			taskM["driver"] = task.Driver
			if task.Meta != nil {
				taskM["meta"] = task.Meta
			} else {
				taskM["meta"] = make(map[string]interface{})
			}

			volumeMountsI := make([]interface{}, 0, len(task.VolumeMounts))
			for _, vm := range task.VolumeMounts {
				volumeMountM := make(map[string]interface{})

				volumeMountM["volume"] = vm.Volume
				volumeMountM["destination"] = vm.Destination
				volumeMountM["read_only"] = vm.ReadOnly

				volumeMountsI = append(volumeMountsI, volumeMountM)
			}
			taskM["volume_mounts"] = volumeMountsI

			tasksI = append(tasksI, taskM)
		}
		tgM["task"] = tasksI

		volumesI := make([]interface{}, 0, len(tg.Volumes))
		for _, v := range tg.Volumes {
			volumeM := make(map[string]interface{})

			volumeM["name"] = v.Name
			volumeM["type"] = v.Type
			volumeM["read_only"] = v.ReadOnly
			volumeM["source"] = v.Source

			volumesI = append(volumesI, volumeM)
		}
		sort.Slice(volumesI, func(i, j int) bool {
			return volumesI[i].(map[string]interface{})["name"].(string) <
				volumesI[j].(map[string]interface{})["name"].(string)
		})

		tgM["volumes"] = volumesI

		ret = append(ret, tgM)
	}

	return ret
}

// jobspecDiffSuppress is the DiffSuppressFunc used by the schema to
// check if two jobspecs are equal.
func jobspecDiffSuppress(k, old, new string, d *schema.ResourceData) bool {
	var oldJob *api.Job
	var newJob *api.Job
	var oldErr error
	var newErr error

	// Read job parsing config.
	jobParserConfig, err := parseJobParserConfig(d)
	if err != nil {
		log.Printf("[ERROR] %v", err)
		return false
	}

	switch {
	case jobParserConfig.JSON.Enabled:
		oldJob, oldErr = parseJSONJobspec(old)
		newJob, newErr = parseJSONJobspec(new)
	case jobParserConfig.HCL2.Enabled:
		oldJob, oldErr = parseHCL2Jobspec(old, jobParserConfig.HCL2)
		newJob, newErr = parseHCL2Jobspec(new, jobParserConfig.HCL2)
	default:
		oldJob, oldErr = jobspec.Parse(strings.NewReader(old))
		newJob, newErr = jobspec.Parse(strings.NewReader(new))
	}
	if oldErr != nil {
		log.Println("error parsing old jobspec")
		log.Printf("%v\n", oldJob)
		log.Printf("%v", oldErr)
		return false
	}
	if newErr != nil {
		log.Println("error parsing new jobspec")
		log.Printf("%v\n", newJob)
		log.Printf("%v", newErr)
		return false
	}

	// Init
	oldJob.Canonicalize()
	newJob.Canonicalize()

	// Check for jobspec equality
	return reflect.DeepEqual(oldJob, newJob)
}
