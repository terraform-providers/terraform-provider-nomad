package nomad

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-provider-nomad/nomad/core/helper"
)

func resourceJobV2() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"purge_on_delete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"job": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: getJobFields(),
				},
			},
		},
		Create: resourceJobV2Create,
		Update: resourceJobV2Update,
		Read:   resourceJobV2Read,
		Delete: resourceJobV2Deregister,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
	}
}

func resourceJobV2Create(d *schema.ResourceData, meta interface{}) error {
	register := func(job *api.Job) error {
		client := meta.(ProviderConfig).client
		_, _, err := client.Jobs().EnforceRegister(job, 0, nil)
		if err != nil {
			return fmt.Errorf("Failed to create the job: %v", err)
		}
		return nil
	}

	return resourceJobV2Register(register, d, meta)
}

func resourceJobV2Update(d *schema.ResourceData, meta interface{}) error {
	register := func(job *api.Job) error {
		client := meta.(ProviderConfig).client
		_, _, err := client.Jobs().Register(job, nil)
		if err != nil {
			return fmt.Errorf("Failed to update the job: %v", err)
		}
		return nil
	}

	return resourceJobV2Register(register, d, meta)
}

func resourceJobV2Register(register func(job *api.Job) error, d *schema.ResourceData, meta interface{}) error {
	jobDefinition := d.Get("job").([]interface{})[0].(map[string]interface{})
	job, err := getJob(jobDefinition, meta)
	if err != nil {
		return fmt.Errorf("Failed to get job definition: %v", err)
	}

	err = register(job)
	if err != nil {
		return err
	}

	d.SetId(*job.ID)

	// Exceptionally in this resource we don't read after applying a change
	// because we can't save the computed attributes anyway.
	return nil
}

func hasChanges(diff *api.JobDiff) bool {
	// Ignore the Nomad token if it is the only change
	if len(diff.Objects) > 0 {
		return true
	}
	if len(diff.Fields) > 1 {
		return true
	}
	if len(diff.Fields) == 1 {
		field := diff.Fields[0]
		if field.Name != "NomadTokenID" || field.Type != "Deleted" {
			return true
		}
	}

	for _, tg := range diff.TaskGroups {
		if len(tg.Fields)+len(tg.Objects) > 0 {
			return true
		}
		for _, t := range tg.Tasks {
			if len(t.Fields)+len(t.Objects) > 0 {
				return true
			}
		}
	}
	return false
}

func resourceJobV2Read(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).client

	job, _, err := client.Jobs().Info(d.Id(), nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Failed to read the job: %v", err)
	}

	j := map[string]interface{}{
		"id":          *job.ID,
		"namespace":   job.Namespace,
		"priority":    job.Priority,
		"type":        job.Type,
		"region":      job.Region,
		"meta":        job.Meta,
		"all_at_once": job.AllAtOnce,
		"datacenters": job.Datacenters,
		"name":        job.Name,
		"constraint":  readConstraints(job.Constraints),
		"affinity":    readAffinities(job.Affinities),
		"spread":      readSpreads(job.Spreads),
	}

	if len(d.Get("job").([]interface{})) > 0 {
		jobDefinition := d.Get("job").([]interface{})[0].(map[string]interface{})

		wantedJob, err := getJob(jobDefinition, meta)
		if err != nil {
			return err
		}
		wantedJob.ConsulToken = nil
		wantedJob.VaultToken = nil
		plan, _, err := client.Jobs().Plan(wantedJob, true, nil)

		log.Printf("[DEBUG] Got the diff from Nomad: %s", spew.Sdump(plan.Diff))

		// Check for changes
		if !hasChanges(plan.Diff) {
			// When the job has not been changed we skip updating the state as it
			// is currently not possible to suppress diffs on complex attributes:
			// https://github.com/hashicorp/terraform-plugin-sdk/issues/477
			// I went down the rabbit hole of trying to find another semantically
			// equivalent representation that may be used to work around the
			// Terraform SDK quirks, but it gets very difficult quickly and may not
			// always be possible. The solution I choose here is to skip setting
			// the attributes that may cause issues. We will be able to improve this
			// when the Terraform SDK supports it, and should be an acceptable
			// compromise in the meantime.
			return nil
		}

		groups, err := readGroups(job.TaskGroups)
		if err != nil {
			return err
		}
		j["group"] = groups

		parameterized := make([]interface{}, 0)
		if job.ParameterizedJob != nil {
			p := map[string]interface{}{
				"meta_optional": job.ParameterizedJob.MetaOptional,
				"meta_required": job.ParameterizedJob.MetaRequired,
				"payload":       job.ParameterizedJob.Payload,
			}
			parameterized = append(parameterized, p)
		}
		j["parameterized"] = parameterized

		periodic := make([]interface{}, 0)
		if job.Periodic != nil {
			p := map[string]interface{}{
				"cron":             job.Periodic.Spec,
				"prohibit_overlap": job.Periodic.ProhibitOverlap,
				"time_zone":        job.Periodic.TimeZone,
			}
			periodic = append(periodic, p)
		}
		j["periodic"] = periodic

		j["update"] = readUpdate(job.Update)
	}

	sw := helper.NewStateWriter(d)
	sw.Set("job", []interface{}{j})

	return sw.Error()
}

func resourceJobV2Deregister(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).client

	purge := d.Get("purge_on_delete").(bool)
	_, _, err := client.Jobs().Deregister(d.Id(), purge, nil)
	if err != nil {
		return fmt.Errorf("Failed to deregister the job: %v", err)
	}

	d.SetId("")
	return nil
}

// Helpers to covert to representation used by the Nomad API

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func getString(d interface{}, name string) *string {
	if m, ok := d.(map[string]interface{}); ok {
		return strToPtr(m[name].(string))
	}
	return strToPtr(d.(*schema.ResourceData).Get(name).(string))
}

func getBool(d interface{}, name string) *bool {
	if m, ok := d.(map[string]interface{}); ok {
		return helper.BoolToPtr(m[name].(bool))
	}
	return helper.BoolToPtr(d.(*schema.ResourceData).Get(name).(bool))
}

func getInt(d interface{}, name string) *int {
	if m, ok := d.(map[string]interface{}); ok {
		return helper.IntToPtr(m[name].(int))
	}
	return helper.IntToPtr(d.(*schema.ResourceData).Get(name).(int))
}

func getMapOfString(d interface{}) map[string]string {
	res := make(map[string]string)
	for key, value := range d.(map[string]interface{}) {
		res[key] = value.(string)
	}
	return res
}

func getListOfString(d interface{}) []string {
	res := make([]string, 0)
	for _, e := range d.([]interface{}) {
		res = append(res, e.(string))
	}
	return res
}

func getDuration(d interface{}) (*time.Duration, error) {
	s := d.(string)

	if s == "" {
		return nil, nil
	}

	duration, err := time.ParseDuration(s)
	if duration.Seconds() == 0 {
		return nil, err
	}
	return &duration, err
}

// Those functions should have a 1 to 1 correspondance with the ones in
// resource_job_v2_fields to make it easy to check we did not forget anything

func getJob(d map[string]interface{}, meta interface{}) (*api.Job, error) {
	datacenters := getListOfString(d["datacenters"])

	var parametrizedJob *api.ParameterizedJobConfig
	for _, pj := range d["parameterized"].([]interface{}) {
		p := pj.(map[string]interface{})

		parametrizedJob = &api.ParameterizedJobConfig{
			Payload:      p["payload"].(string),
			MetaRequired: getListOfString(p["meta_required"]),
			MetaOptional: getListOfString(p["meta_optional"]),
		}
	}

	var periodic *api.PeriodicConfig
	for _, pc := range d["periodic"].([]interface{}) {
		p := pc.(map[string]interface{})
		periodic = &api.PeriodicConfig{
			Enabled:         helper.BoolToPtr(true),
			Spec:            getString(p, "cron"),
			SpecType:        strToPtr("cron"),
			ProhibitOverlap: getBool(p, "prohibit_overlap"),
			TimeZone:        getString(p, "time_zone"),
		}
	}

	update, err := getUpdate(d["update"])
	if err != nil {
		return nil, err
	}
	vault := getVault(d["vault"])
	taskGroups, err := getTaskGroups(d["group"], vault)
	if err != nil {
		return nil, err
	}

	ID := getString(d, "id")
	if ID == nil {
		ID = getString(d, "name")
	}

	region := getString(d, "region")
	if region == nil {
		region = meta.(ProviderConfig).region
	}

	migrate, err := getMigrate(d["migrate"])
	if err != nil {
		return nil, err
	}

	reschedule, err := getReschedule(d["reschedule"])
	if err != nil {
		return nil, err
	}

	return &api.Job{
		ID:          ID,
		Name:        getString(d, "name"),
		Namespace:   getString(d, "namespace"),
		Priority:    getInt(d, "priority"),
		Type:        getString(d, "type"),
		Meta:        getMapOfString(d["meta"]),
		AllAtOnce:   getBool(d, "all_at_once"),
		Datacenters: datacenters,
		Region:      region,
		VaultToken:  getString(d, "vault_token"),
		ConsulToken: getString(d, "consul_token"),
		Migrate:     migrate,
		Reschedule:  reschedule,

		Constraints: getConstraints(d["constraint"]),
		Affinities:  getAffinities(d["affinity"]),
		Spreads:     getSpreads(d["spread"]),
		TaskGroups:  taskGroups,

		ParameterizedJob: parametrizedJob,
		Periodic:         periodic,

		Update: update,
	}, nil
}

func getConstraints(d interface{}) []*api.Constraint {
	constraints := make([]*api.Constraint, 0)

	for _, ct := range d.([]interface{}) {
		c := ct.(map[string]interface{})
		constraints = append(
			constraints,
			api.NewConstraint(
				c["attribute"].(string),
				c["operator"].(string),
				c["value"].(string),
			),
		)
	}

	return constraints
}

func getAffinities(d interface{}) []*api.Affinity {
	affinities := make([]*api.Affinity, 0)

	for _, af := range d.([]interface{}) {
		a := af.(map[string]interface{})
		affinities = append(
			affinities,
			api.NewAffinity(
				a["attribute"].(string),
				a["operator"].(string),
				a["value"].(string),
				int8(a["weight"].(int)),
			),
		)
	}

	return affinities
}

func getSpreads(d interface{}) []*api.Spread {
	spreads := make([]*api.Spread, 0)

	for _, sp := range d.([]interface{}) {
		s := sp.(map[string]interface{})

		targets := make([]*api.SpreadTarget, 0)
		for _, tg := range s["target"].([]interface{}) {
			t := tg.(map[string]interface{})
			targets = append(
				targets,
				&api.SpreadTarget{
					Value:   t["value"].(string),
					Percent: uint8(t["percent"].(int)),
				},
			)
		}

		spreads = append(
			spreads,
			api.NewSpread(
				s["attribute"].(string),
				int8(s["weight"].(int)),
				targets,
			),
		)
	}

	return spreads
}

func getTaskGroups(d interface{}, jobVault *api.Vault) ([]*api.TaskGroup, error) {
	taskGroups := make([]*api.TaskGroup, 0)

	for _, tg := range d.([]interface{}) {
		g := tg.(map[string]interface{})

		migrate, err := getMigrate(g["migrate"])
		if err != nil {
			return nil, err
		}
		reschedule, err := getReschedule(g["reschedule"])
		if err != nil {
			return nil, err
		}

		var ephemeralDisk *api.EphemeralDisk
		for _, ed := range g["ephemeral_disk"].([]interface{}) {
			e := ed.(map[string]interface{})
			ephemeralDisk = &api.EphemeralDisk{
				Sticky:  getBool(e, "sticky"),
				Migrate: getBool(e, "migrate"),
				SizeMB:  getInt(e, "size"),
			}
		}

		var restartPolicy *api.RestartPolicy
		for _, rp := range g["restart"].([]interface{}) {
			r := rp.(map[string]interface{})
			restartPolicy = &api.RestartPolicy{
				Attempts: getInt(r, "attempts"),
				Mode:     getString(r, "mode"),
			}

			delay, err := getDuration(r["delay"])
			if err != nil {
				return nil, err
			}
			restartPolicy.Delay = delay

			interval, err := getDuration(r["interval"])
			if err != nil {
				return nil, err
			}
			restartPolicy.Interval = interval
		}
		volumes := make(map[string]*api.VolumeRequest)
		for _, vr := range g["volume"].([]interface{}) {
			v := vr.(map[string]interface{})
			name := v["name"].(string)
			volumes[name] = &api.VolumeRequest{
				Name:     name,
				Type:     v["type"].(string),
				Source:   v["source"].(string),
				ReadOnly: v["read_only"].(bool),
			}
		}

		vault := getVault(g["vault"])
		if vault == nil {
			vault = jobVault
		}

		tasks, err := getTasks(g["task"], vault)
		if err != nil {
			return nil, err
		}

		services, err := getServices(g["service"])
		if err != nil {
			return nil, err
		}

		group := &api.TaskGroup{
			Name:             getString(g, "name"),
			Meta:             getMapOfString(g["meta"]),
			Count:            getInt(g, "count"),
			Constraints:      getConstraints(g["constraint"]),
			Affinities:       getAffinities(g["affinity"]),
			Spreads:          getSpreads(g["spread"]),
			EphemeralDisk:    ephemeralDisk,
			Migrate:          migrate,
			Networks:         getNetworks(g["network"]),
			ReschedulePolicy: reschedule,
			RestartPolicy:    restartPolicy,
			Services:         services,
			Tasks:            tasks,
			Volumes:          volumes,
		}

		shutdownDelay, err := getDuration(g["shutdown_delay"])
		if err != nil {
			return nil, err
		}
		group.ShutdownDelay = shutdownDelay

		stopAfterClientDisconnect, err := getDuration(g["stop_after_client_disconnect"])
		if err != nil {
			return nil, err
		}
		group.StopAfterClientDisconnect = stopAfterClientDisconnect

		taskGroups = append(taskGroups, group)
	}

	return taskGroups, nil
}

func getMigrate(d interface{}) (*api.MigrateStrategy, error) {
	for _, mg := range d.([]interface{}) {
		m := mg.(map[string]interface{})

		migrateStrategy := &api.MigrateStrategy{
			MaxParallel: getInt(m, "max_parallel"),
			HealthCheck: getString(m, "health_check"),
		}

		minHealthyTime, err := getDuration(m["min_healthy_time"])
		if err != nil {
			return nil, err
		}
		migrateStrategy.MinHealthyTime = minHealthyTime

		healthyDeadline, err := getDuration(m["healthy_deadline"])
		if err != nil {
			return nil, err
		}
		migrateStrategy.HealthyDeadline = healthyDeadline

		return migrateStrategy, nil
	}

	return nil, nil
}

func getReschedule(d interface{}) (*api.ReschedulePolicy, error) {
	for _, re := range d.([]interface{}) {
		r := re.(map[string]interface{})

		reschedulePolicy := &api.ReschedulePolicy{
			Attempts:      getInt(r, "attempts"),
			DelayFunction: getString(r, "delay_function"),
			Unlimited:     getBool(r, "unlimited"),
		}

		interval, err := getDuration(r["interval"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.Interval = interval

		delay, err := getDuration(r["delay"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.Delay = delay

		maxDelay, err := getDuration(r["max_delay"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.MaxDelay = maxDelay

		return reschedulePolicy, nil
	}

	return nil, nil
}

func getUpdate(d interface{}) (*api.UpdateStrategy, error) {
	for _, up := range d.([]interface{}) {
		u := up.(map[string]interface{})

		update := &api.UpdateStrategy{
			MaxParallel: getInt(u, "max_parallel"),
			HealthCheck: getString(u, "health_check"),
			Canary:      getInt(u, "canary"),
			AutoRevert:  getBool(u, "auto_revert"),
			AutoPromote: getBool(u, "auto_promote"),
		}

		stagger, err := getDuration(u["stagger"])
		if err != nil {
			return nil, err
		}
		update.Stagger = stagger

		minHealthyTime, err := getDuration(u["min_healthy_time"])
		if err != nil {
			return nil, err
		}
		update.MinHealthyTime = minHealthyTime

		healthyDeadline, err := getDuration(u["healthy_deadline"])
		if err != nil {
			return nil, err
		}
		update.HealthyDeadline = healthyDeadline

		progressDeadline, err := getDuration(u["progress_deadline"])
		if err != nil {
			return nil, err
		}
		update.ProgressDeadline = progressDeadline

		return update, nil
	}
	return nil, nil
}

func getTasks(d interface{}, groupVault *api.Vault) ([]*api.Task, error) {
	tasks := make([]*api.Task, 0)

	for _, tk := range d.([]interface{}) {
		t := tk.(map[string]interface{})

		artifacts := make([]*api.TaskArtifact, 0)
		for _, af := range t["artifact"].([]interface{}) {
			a := af.(map[string]interface{})

			artifact := &api.TaskArtifact{
				GetterSource:  getString(a, "source"),
				GetterOptions: getMapOfString(a["options"]),
				GetterMode:    getString(a, "mode"),
				RelativeDest:  getString(a, "destination"),
			}

			artifacts = append(artifacts, artifact)
		}

		var dispatchPayloadConfig *api.DispatchPayloadConfig
		for _, dp := range t["dispatch_payload"].([]interface{}) {
			d := dp.(map[string]interface{})

			dispatchPayloadConfig = &api.DispatchPayloadConfig{
				File: d["file"].(string),
			}
		}

		var taskLifecycle *api.TaskLifecycle
		for _, tl := range t["lifecycle"].([]interface{}) {
			l := tl.(map[string]interface{})

			taskLifecycle = &api.TaskLifecycle{
				Hook:    l["hook"].(string),
				Sidecar: l["sidecar"].(bool),
			}
		}

		templates := make([]*api.Template, 0)
		for _, tpl := range t["template"].([]interface{}) {
			tp := tpl.(map[string]interface{})

			template := &api.Template{
				ChangeMode:   getString(tp, "change_mode"),
				ChangeSignal: getString(tp, "change_signal"),
				EmbeddedTmpl: getString(tp, "data"),
				DestPath:     getString(tp, "destination"),
				Envvars:      getBool(tp, "env"),
				LeftDelim:    getString(tp, "left_delimiter"),
				Perms:        getString(tp, "perms"),
				RightDelim:   getString(tp, "right_delimiter"),
				SourcePath:   getString(tp, "source"),
			}

			splay, err := getDuration(tp["splay"])
			if err != nil {
				return nil, err
			}
			template.Splay = splay

			vaultGrace, err := getDuration(tp["vault_grace"])
			if err != nil {
				return nil, err
			}
			template.VaultGrace = vaultGrace

			templates = append(templates, template)
		}

		volumeMounts := make([]*api.VolumeMount, 0)
		for _, vm := range t["volume_mount"].([]interface{}) {
			v := vm.(map[string]interface{})

			volumeMount := &api.VolumeMount{
				Volume:      getString(v, "volume"),
				Destination: getString(v, "destination"),
				ReadOnly:    getBool(v, "read_only"),
			}

			volumeMounts = append(volumeMounts, volumeMount)
		}

		var config map[string]interface{}
		err := json.Unmarshal([]byte(t["config"].(string)), &config)
		if err != nil {
			return nil, err
		}

		services, err := getServices(t["service"])
		if err != nil {
			return nil, err
		}

		vault := getVault(t["vault"])
		if vault == nil {
			vault = groupVault
		}

		task := &api.Task{
			Name:            t["name"].(string),
			Config:          config,
			Meta:            getMapOfString(t["meta"]),
			Driver:          t["driver"].(string),
			KillSignal:      t["kill_signal"].(string),
			Leader:          t["leader"].(bool),
			User:            t["user"].(string),
			Kind:            t["kind"].(string),
			Artifacts:       artifacts,
			Constraints:     getConstraints(t["constraint"]),
			Affinities:      getAffinities(t["affinity"]),
			DispatchPayload: dispatchPayloadConfig,
			Env:             getMapOfString(t["env"]),
			Lifecycle:       taskLifecycle,
			LogConfig:       getLogConfig(t["logs"]),
			Resources:       getResources(t["resources"]),
			Services:        services,
			Templates:       templates,
			Vault:           vault,
			VolumeMounts:    volumeMounts,
		}

		killTimeout, err := getDuration(t["kill_timeout"])
		if err != nil {
			return nil, err
		}
		task.KillTimeout = killTimeout

		shutdownDelay, err := getDuration(t["shutdown_delay"])
		if err != nil {
			return nil, err
		}
		if shutdownDelay != nil {
			task.ShutdownDelay = *shutdownDelay
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func getNetworks(d interface{}) []*api.NetworkResource {
	networks := make([]*api.NetworkResource, 0)

	for _, nr := range d.([]interface{}) {
		n := nr.(map[string]interface{})

		network := &api.NetworkResource{
			Mode:  n["mode"].(string),
			MBits: getInt(n, "mbits"),
		}

		for _, p := range n["port"].(*schema.Set).List() {
			pt := p.(map[string]interface{})
			port := api.Port{
				Label:       pt["label"].(string),
				Value:       pt["static"].(int),
				To:          pt["to"].(int),
				HostNetwork: pt["host_network"].(string),
			}

			if port.Value > 0 {
				network.ReservedPorts = append(network.ReservedPorts, port)
			} else {
				network.DynamicPorts = append(network.DynamicPorts, port)
			}
		}

		for _, dns := range n["dns"].([]interface{}) {
			d := dns.(map[string]interface{})

			network.DNS = &api.DNSConfig{}

			network.DNS.Servers = getListOfString(d["servers"])
			network.DNS.Searches = getListOfString(d["searches"])
			network.DNS.Options = getListOfString(d["options"])
		}

		networks = append(networks, network)
	}

	return networks
}

func getServices(d interface{}) ([]*api.Service, error) {
	services := make([]*api.Service, 0)

	for _, svc := range d.([]interface{}) {
		s := svc.(map[string]interface{})

		checks := make([]api.ServiceCheck, 0)
		for _, cks := range s["check"].([]interface{}) {
			c := cks.(map[string]interface{})

			var checkRestart *api.CheckRestart
			for _, crt := range c["check_restart"].([]interface{}) {
				cr := crt.(map[string]interface{})

				grace, err := getDuration(cr["grace"])
				if err != nil {
					return nil, err
				}
				checkRestart = &api.CheckRestart{
					Limit:          cr["limit"].(int),
					Grace:          grace,
					IgnoreWarnings: cr["ignore_warnings"].(bool),
				}
			}

			check := api.ServiceCheck{
				AddressMode:            c["address_mode"].(string),
				Args:                   getListOfString(c["args"]),
				Command:                c["command"].(string),
				GRPCService:            c["grpc_service"].(string),
				GRPCUseTLS:             c["grpc_use_tls"].(bool),
				InitialStatus:          c["initial_status"].(string),
				SuccessBeforePassing:   c["success_before_passing"].(int),
				FailuresBeforeCritical: c["failures_before_critical"].(int),
				Method:                 c["method"].(string),
				Name:                   c["name"].(string),
				Path:                   c["path"].(string),
				Expose:                 c["expose"].(bool),
				PortLabel:              c["port"].(string),
				Protocol:               c["protocol"].(string),
				TaskName:               c["task"].(string),
				Type:                   c["type"].(string),
				TLSSkipVerify:          c["tls_skip_verify"].(bool),
				CheckRestart:           checkRestart,
			}

			timeout, err := getDuration(c["timeout"])
			if err != nil {
				return nil, err
			}
			if timeout != nil {
				check.Timeout = *timeout
			}

			interval, err := getDuration(c["interval"])
			if err != nil {
				return nil, err
			}
			if interval != nil {
				check.Interval = *interval
			}

			checks = append(checks, check)
		}

		var connect *api.ConsulConnect
		for _, con := range s["connect"].([]interface{}) {
			cn := con.(map[string]interface{})

			var sidecarTask *api.SidecarTask
			for _, stask := range cn["sidecar_task"].([]interface{}) {
				st := stask.(map[string]interface{})

				var config map[string]interface{}
				err := json.Unmarshal([]byte(st["config"].(string)), &d)
				if err != nil {
					return nil, err
				}

				sidecarTask = &api.SidecarTask{
					Meta:       getMapOfString(st["meta"]),
					Name:       st["Name"].(string),
					Driver:     st["Driver"].(string),
					User:       st["User"].(string),
					Config:     config,
					Env:        getMapOfString(st["env"]),
					KillSignal: st["kill_signal"].(string),
					Resources:  getResources(st["resources"]),
					LogConfig:  getLogConfig(st["logs"]),
				}

				sidecarTask.KillTimeout, err = getDuration(st["kill_timeout"])
				if err != nil {
					return nil, err
				}

				sidecarTask.ShutdownDelay, err = getDuration(st["shutdown_delay"])
				if err != nil {
					return nil, err
				}
			}

			var sidecarService *api.ConsulSidecarService
			for _, sservice := range cn["sidecar_service"].([]interface{}) {
				ss := sservice.(map[string]interface{})

				var consulProxy *api.ConsulProxy
				for _, proxy := range ss["proxy"].([]interface{}) {
					p := proxy.(map[string]interface{})

					var config map[string]interface{}
					err := json.Unmarshal([]byte(p["config"].(string)), &config)
					if err != nil {
						return nil, err
					}

					upstreams := make([]*api.ConsulUpstream, 0)
					for _, up := range p["upstreams"].([]interface{}) {
						u := up.(map[string]interface{})

						upstream := &api.ConsulUpstream{
							DestinationName: u["destination_name"].(string),
							LocalBindPort:   u["local_bind_port"].(int),
						}

						upstreams = append(upstreams, upstream)
					}

					var exposeConfig *api.ConsulExposeConfig
					for _, cec := range p["expose"].([]interface{}) {
						ec := cec.(map[string]interface{})

						paths := make([]*api.ConsulExposePath, 0)
						for _, cep := range ec["path"].([]interface{}) {
							p := cep.(map[string]interface{})

							path := &api.ConsulExposePath{
								Path:          p["path"].(string),
								Protocol:      p["protocol"].(string),
								LocalPathPort: p["local_path_port"].(int),
								ListenerPort:  p["listener_port"].(string),
							}

							paths = append(paths, path)
						}

						exposeConfig = &api.ConsulExposeConfig{
							Path: paths,
						}
					}

					consulProxy = &api.ConsulProxy{
						LocalServiceAddress: p["local_service_address"].(string),
						LocalServicePort:    p["local_service_port"].(int),
						ExposeConfig:        exposeConfig,
						Upstreams:           upstreams,
						Config:              config,
					}
				}

				sidecarService = &api.ConsulSidecarService{
					Tags:  getListOfString(ss["tags"]),
					Port:  ss["port"].(string),
					Proxy: consulProxy,
				}
			}

			connect = &api.ConsulConnect{
				Native:         cn["native"].(bool),
				SidecarService: sidecarService,
				SidecarTask:    sidecarTask,
			}
		}

		service := &api.Service{
			Meta:              getMapOfString(s["meta"]),
			Name:              s["name"].(string),
			PortLabel:         s["port"].(string),
			Tags:              getListOfString(s["tags"]),
			CanaryTags:        getListOfString(s["canary_tags"]),
			EnableTagOverride: s["enable_tag_override"].(bool),
			AddressMode:       s["address_mode"].(string),
			TaskName:          s["task"].(string),
			Checks:            checks,
			Connect:           connect,
			CanaryMeta:        getMapOfString(s["canary_meta"]),
		}

		services = append(services, service)
	}

	return services, nil
}

func getLogConfig(d interface{}) *api.LogConfig {
	for _, lg := range d.([]interface{}) {
		l := lg.(map[string]interface{})

		return &api.LogConfig{
			MaxFiles:      getInt(l, "max_files"),
			MaxFileSizeMB: getInt(l, "max_file_size"),
		}
	}

	return nil
}

func getResources(d interface{}) *api.Resources {
	for _, rs := range d.([]interface{}) {
		r := rs.(map[string]interface{})

		devices := make([]*api.RequestedDevice, 0)
		for _, dv := range r["device"].([]interface{}) {
			d := dv.(map[string]interface{})

			count := uint64(d["count"].(int))
			device := &api.RequestedDevice{
				Name:        d["name"].(string),
				Count:       &count,
				Constraints: getConstraints(d["constraint"]),
				Affinities:  getAffinities(d["affinity"]),
			}

			devices = append(devices, device)
		}

		return &api.Resources{
			CPU:      getInt(r, "cpu"),
			MemoryMB: getInt(r, "memory"),
			Networks: getNetworks(r["network"]),
			Devices:  devices,
		}
	}

	return nil
}

func getVault(d interface{}) *api.Vault {
	for _, vlt := range d.([]interface{}) {
		v := vlt.(map[string]interface{})

		return &api.Vault{
			Policies:     getListOfString(v["policies"]),
			Namespace:    getString(v, "namespace"),
			Env:          getBool(v, "env"),
			ChangeMode:   getString(v, "change_mode"),
			ChangeSignal: getString(v, "change_signal"),
		}
	}

	return nil
}

// Readers

func readConstraints(constraints []*api.Constraint) interface{} {
	res := make([]interface{}, 0)

	for _, cn := range constraints {
		constraint := map[string]interface{}{
			"attribute": cn.LTarget,
			"operator":  cn.Operand,
			"value":     cn.RTarget,
		}

		res = append(res, constraint)
	}

	return res
}

func readAffinities(affinities []*api.Affinity) interface{} {
	res := make([]interface{}, 0)

	for _, af := range affinities {
		affinity := map[string]interface{}{
			"attribute": af.LTarget,
			"operator":  af.Operand,
			"value":     af.RTarget,
			"weight":    af.Weight,
		}

		res = append(res, affinity)
	}

	return res
}

func readSpreads(spreads []*api.Spread) interface{} {
	res := make([]interface{}, 0)

	for _, s := range spreads {
		targets := make([]interface{}, 0)

		for _, t := range s.SpreadTarget {
			target := map[string]interface{}{
				"value":   t.Value,
				"percent": t.Percent,
			}

			targets = append(targets, target)
		}

		spread := map[string]interface{}{
			"attribute": s.Attribute,
			"weight":    s.Weight,
			"target":    targets,
		}

		res = append(res, spread)
	}

	return res
}

func readGroups(groups []*api.TaskGroup) (interface{}, error) {
	res := make([]interface{}, 0)

	for _, g := range groups {

		ephemeralDisk := make([]interface{}, 0)

		if g.EphemeralDisk != nil {
			disk := map[string]interface{}{
				"migrate": *g.EphemeralDisk.Migrate,
				"size":    *g.EphemeralDisk.SizeMB,
				"sticky":  *g.EphemeralDisk.Sticky,
			}
			ephemeralDisk = append(ephemeralDisk, disk)
		}

		restart := make([]interface{}, 0)
		if g.RestartPolicy != nil {
			r := map[string]interface{}{
				"attempts": *g.RestartPolicy.Attempts,
				"delay":    g.RestartPolicy.Delay.String(),
				"interval": g.RestartPolicy.Interval.String(),
				"mode":     *g.RestartPolicy.Mode,
			}
			restart = append(restart, r)
		}

		volume := make([]interface{}, 0)
		for name, vlm := range g.Volumes {
			v := map[string]interface{}{
				"name":      name,
				"type":      vlm.Type,
				"source":    vlm.Source,
				"read_only": vlm.ReadOnly,
			}

			volume = append(volume, v)
		}

		tasks, err := readTasks(g.Tasks)
		if err != nil {
			return nil, err
		}

		services, err := readServices(g.Services)
		if err != nil {
			return nil, err
		}

		reschedule := readReschedule(g.ReschedulePolicy)

		group := map[string]interface{}{
			"name":           g.Name,
			"meta":           g.Meta,
			"count":          g.Count,
			"constraint":     readConstraints(g.Constraints),
			"affinity":       readAffinities(g.Affinities),
			"spread":         readSpreads(g.Spreads),
			"ephemeral_disk": ephemeralDisk,
			"migrate":        readMigrate(g.Migrate),
			"network":        readNetworks(g.Networks),
			"reschedule":     reschedule,
			"restart":        restart,
			"service":        services,
			"task":           tasks,
			"volume":         volume,
		}

		if g.ShutdownDelay != nil {
			group["shutdown_delay"] = g.ShutdownDelay.String()
		}
		if g.StopAfterClientDisconnect != nil {
			group["stop_after_client_disconnect"] = g.StopAfterClientDisconnect.String()
		}

		res = append(res, group)
	}

	return res, nil
}

func readMigrate(migrate *api.MigrateStrategy) interface{} {
	if migrate == nil {
		return []interface{}{}
	}

	res := map[string]interface{}{
		"max_parallel":     *migrate.MaxParallel,
		"health_check":     *migrate.HealthCheck,
		"min_healthy_time": migrate.MinHealthyTime.String(),
		"healthy_deadline": migrate.HealthyDeadline.String(),
	}

	return []interface{}{res}
}

func readReschedule(reschedule *api.ReschedulePolicy) interface{} {
	if reschedule == nil {
		return nil
	}

	res := map[string]interface{}{
		"attempts":       *reschedule.Attempts,
		"interval":       reschedule.Interval.String(),
		"delay":          reschedule.Delay.String(),
		"delay_function": *reschedule.DelayFunction,
		"max_delay":      reschedule.MaxDelay.String(),
		"unlimited":      *reschedule.Unlimited,
	}

	return []interface{}{res}
}

func readUpdate(update *api.UpdateStrategy) interface{} {
	res := map[string]interface{}{
		"max_parallel":      *update.MaxParallel,
		"health_check":      *update.HealthCheck,
		"min_healthy_time":  update.MinHealthyTime.String(),
		"healthy_deadline":  update.HealthyDeadline.String(),
		"progress_deadline": update.ProgressDeadline.String(),
		"auto_revert":       *update.AutoRevert,
		"auto_promote":      *update.AutoPromote,
		"canary":            *update.Canary,
		"stagger":           update.Stagger.String(),
	}

	return []interface{}{res}
}

func readNetworks(networks []*api.NetworkResource) interface{} {
	res := make([]interface{}, 0)

	for _, n := range networks {
		dns := make([]interface{}, 0)
		if n.DNS != nil {
			d := map[string]interface{}{
				"servers":  n.DNS.Servers,
				"searches": n.DNS.Searches,
				"options":  n.DNS.Options,
			}
			dns = append(dns, d)
		}

		ports := make([]interface{}, 0)
		for _, p := range n.DynamicPorts {
			ports = append(ports, map[string]interface{}{
				"label":        p.Label,
				"static":       p.Value,
				"to":           p.To,
				"host_network": p.HostNetwork,
			})
		}
		for _, p := range n.ReservedPorts {
			ports = append(ports, map[string]interface{}{
				"label":        p.Label,
				"static":       p.Value,
				"to":           p.To,
				"host_network": p.HostNetwork,
			})
		}

		network := map[string]interface{}{
			"mbits": *n.MBits,
			"mode":  n.Mode,
			"port":  ports,
			"dns":   dns,
		}

		res = append(res, network)
	}

	return res
}

func readServices(services []*api.Service) (interface{}, error) {
	res := make([]interface{}, 0)

	for _, sv := range services {
		checks := make([]interface{}, 0)
		for _, ck := range sv.Checks {
			checkRestart := make([]interface{}, 0)
			if ck.CheckRestart != nil {
				checkRestart = append(
					checkRestart,
					map[string]interface{}{
						"limit":           ck.CheckRestart.Limit,
						"grace":           ck.CheckRestart.Grace,
						"ignore_warnings": ck.CheckRestart.IgnoreWarnings,
					},
				)
			}

			c := map[string]interface{}{
				"address_mode":             ck.AddressMode,
				"args":                     ck.Args,
				"command":                  ck.Command,
				"grpc_service":             ck.GRPCService,
				"grpc_use_tls":             ck.GRPCUseTLS,
				"initial_status":           ck.InitialStatus,
				"success_before_passing":   ck.SuccessBeforePassing,
				"failures_before_critical": ck.FailuresBeforeCritical,
				"interval":                 ck.Interval.String(),
				"method":                   ck.Method,
				"name":                     ck.Name,
				"path":                     ck.Path,
				"expose":                   ck.Expose,
				"port":                     ck.PortLabel,
				"protocol":                 ck.Protocol,
				"task":                     ck.TaskName,
				"timeout":                  ck.Timeout.String(),
				"type":                     ck.Type,
				"tls_skip_verify":          ck.TLSSkipVerify,
				"check_restart":            checkRestart,
			}

			checks = append(checks, c)
		}

		connect := make([]interface{}, 0)
		if sv.Connect != nil {
			sidecarService := make([]interface{}, 0)
			if sv.Connect.SidecarService != nil {
				proxy := make([]interface{}, 0)
				if sv.Connect.SidecarService.Proxy != nil {
					p := sv.Connect.SidecarService.Proxy

					config, err := json.Marshal(p.Config)
					if err != nil {
						return nil, err
					}

					upstreams := make([]interface{}, 0)
					for _, up := range p.Upstreams {
						upstreams = append(upstreams, map[string]interface{}{
							"destination_name": up.DestinationName,
							"local_bind_port":  up.LocalBindPort,
						})
					}

					expose := make([]interface{}, 0)
					if p.ExposeConfig != nil {
						paths := make([]interface{}, 0)

						for _, path := range p.ExposeConfig.Path {
							paths = append(paths, map[string]interface{}{
								"path":            path.Path,
								"protocol":        path.Protocol,
								"local_path_port": path.LocalPathPort,
								"listener_port":   path.ListenerPort,
							})
						}

						expose = append(expose, map[string]interface{}{
							"path": paths,
						})
					}

					proxy = append(proxy, map[string]interface{}{
						"local_service_address": p.LocalServiceAddress,
						"local_service_port":    p.LocalServicePort,
						"config":                string(config),
						"upstreams":             upstreams,
						"expose":                expose,
					})
				}

				sidecarService = append(sidecarService, map[string]interface{}{
					"tags":  sv.Connect.SidecarService.Tags,
					"port":  sv.Connect.SidecarService.Port,
					"proxy": proxy,
				})
			}

			connect = append(
				connect,
				map[string]interface{}{
					"native":          sv.Connect.Native,
					"sidecar_service": sidecarService,
					"sidecar_task":    nil,
				},
			)
		}

		s := map[string]interface{}{
			"meta":                sv.Meta,
			"name":                sv.Name,
			"port":                sv.PortLabel,
			"tags":                sv.Tags,
			"canary_tags":         sv.CanaryTags,
			"enable_tag_override": sv.EnableTagOverride,
			"address_mode":        sv.AddressMode,
			"task":                sv.TaskName,
			"check":               checks,
			"connect":             connect,
			"canary_meta":         sv.CanaryMeta,
		}

		res = append(res, s)
	}

	return res, nil
}

func readTasks(tasks []*api.Task) (interface{}, error) {
	res := make([]interface{}, 0)

	for _, t := range tasks {

		config, err := json.Marshal(t.Config)
		if err != nil {
			return nil, err
		}

		artifacts := make([]interface{}, 0)
		for _, at := range t.Artifacts {
			a := map[string]interface{}{
				"destination": at.RelativeDest,
				"mode":        at.GetterMode,
				"options":     at.GetterOptions,
				"source":      at.GetterSource,
			}

			artifacts = append(artifacts, a)
		}

		dispatchPayload := make([]interface{}, 0)
		if t.DispatchPayload != nil {
			d := map[string]interface{}{
				"file": t.DispatchPayload.File,
			}
			dispatchPayload = append(dispatchPayload, d)
		}

		lifecycle := make([]interface{}, 0)
		if t.Lifecycle != nil {
			l := map[string]interface{}{
				"hook":    t.Lifecycle.Hook,
				"sidecar": t.Lifecycle.Sidecar,
			}

			lifecycle = append(lifecycle, l)
		}

		templates := make([]interface{}, 0)
		for _, tpl := range t.Templates {
			t := map[string]interface{}{
				"change_mode":     tpl.ChangeMode,
				"change_signal":   tpl.ChangeSignal,
				"data":            tpl.EmbeddedTmpl,
				"destination":     tpl.DestPath,
				"env":             tpl.Envvars,
				"left_delimiter":  tpl.LeftDelim,
				"perms":           tpl.Perms,
				"right_delimiter": tpl.RightDelim,
				"source":          tpl.SourcePath,
				"splay":           tpl.Splay.String(),
				"vault_grace":     tpl.VaultGrace.String(),
			}

			templates = append(templates, t)
		}

		volumeMounts := make([]interface{}, 0)
		for _, vm := range t.VolumeMounts {
			v := map[string]interface{}{
				"volume":      vm.Volume,
				"destination": vm.Destination,
				"read_only":   vm.ReadOnly,
			}

			volumeMounts = append(volumeMounts, v)
		}

		services, err := readServices(t.Services)
		if err != nil {
			return nil, err
		}

		task := map[string]interface{}{
			"name":             t.Name,
			"config":           string(config),
			"env":              t.Env,
			"meta":             t.Meta,
			"driver":           t.Driver,
			"kill_timeout":     t.KillTimeout.String(),
			"kill_signal":      t.KillSignal,
			"leader":           t.Leader,
			"shutdown_delay":   t.ShutdownDelay.String(),
			"user":             t.User,
			"kind":             t.Kind,
			"artifact":         artifacts,
			"constraint":       readConstraints(t.Constraints),
			"affinity":         readAffinities(t.Affinities),
			"dispatch_payload": dispatchPayload,
			"lifecycle":        lifecycle,
			"logs":             readLogs(t.LogConfig),
			"resources":        readResources(t.Resources),
			"service":          services,
			"template":         templates,
			"vault":            readVault(t.Vault),
			"volume_mount":     volumeMounts,
		}

		res = append(res, task)
	}

	return res, nil
}

func readLogs(logs *api.LogConfig) interface{} {
	if logs == nil {
		return []interface{}{}
	}

	res := map[string]interface{}{
		"max_files":     *logs.MaxFiles,
		"max_file_size": *logs.MaxFileSizeMB,
	}

	return []interface{}{res}
}

func readResources(resources *api.Resources) interface{} {
	if resources == nil {
		return nil
	}

	devices := make([]interface{}, 0)
	for _, dev := range resources.Devices {
		d := map[string]interface{}{
			"name":       dev.Name,
			"count":      dev.Count,
			"constraint": readConstraints(dev.Constraints),
			"affinity":   readAffinities(dev.Affinities),
		}

		devices = append(devices, d)
	}

	res := map[string]interface{}{
		"cpu":     *resources.CPU,
		"memory":  *resources.MemoryMB,
		"device":  devices,
		"network": readNetworks(resources.Networks),
	}

	return []interface{}{res}
}

func readVault(vault *api.Vault) interface{} {
	if vault == nil {
		return nil
	}

	return []interface{}{
		map[string]interface{}{
			"change_mode":   vault.ChangeMode,
			"change_signal": vault.ChangeSignal,
			"env":           vault.Env,
			"namespace":     vault.Namespace,
			"policies":      vault.Policies,
		},
	}
}
