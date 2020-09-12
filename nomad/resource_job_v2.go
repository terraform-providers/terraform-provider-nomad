package nomad

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-provider-nomad/nomad/core/helper"
)

func resourceJobV2() *schema.Resource {
	return &schema.Resource{
		Schema: getJobFields(),
		Create: resourceJobV2Register,
		Update: resourceJobV2Register,
		Read:   resourceJobV2Read,
		Delete: resourceJobV2Deregister,
	}
}

func resourceJobV2Register(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).client
	job, err := getJob(d)
	if err != nil {
		return fmt.Errorf("Failed to get job definition: %v", err)
	}

	_, _, err = client.Jobs().Register(job, nil)
	if err != nil {
		return fmt.Errorf("Failed to create the job: %v", err)
	}

	d.SetId(d.Get("name").(string))

	return resourceJobV2Read(d, meta)
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

	sw := helper.NewStateWriter(d)

	sw.Set("namespace", job.Namespace)
	sw.Set("priority", job.Priority)
	sw.Set("type", job.Type)
	sw.Set("region", job.Region)
	sw.Set("meta", job.Meta)
	sw.Set("all_at_once", job.AllAtOnce)
	sw.Set("datacenters", job.Datacenters)
	sw.Set("name", job.Name)
	// if err = d.Set("vault_token", job.VaultToken); err != nil {
	// 	return fmt.Errorf("Failed to set 'vault_token': %v", err)
	// }
	// if err = d.Set("consul_token", job.ConsulToken); err != nil {
	// 	return fmt.Errorf("Failed to set 'consul_token': %v", err)
	// }
	sw.Set("constraint", readConstraints(job.Constraints))
	sw.Set("affinity", readAffinities(job.Affinities))
	sw.Set("spread", readSpreads(job.Spreads))

	groups, err := readGroups(d, job.TaskGroups)
	if err != nil {
		return err
	}
	sw.Set("group", groups)

	parameterized := make([]interface{}, 0)
	if job.ParameterizedJob != nil {
		p := map[string]interface{}{
			"meta_optional": job.ParameterizedJob.MetaOptional,
			"meta_required": job.ParameterizedJob.MetaRequired,
			"payload":       job.ParameterizedJob.Payload,
		}
		parameterized = append(parameterized, p)
	}
	sw.Set("parameterized", parameterized)

	periodic := make([]interface{}, 0)
	if job.Periodic != nil {
		p := map[string]interface{}{
			"cron":             job.Periodic.Spec,
			"prohibit_overlap": job.Periodic.ProhibitOverlap,
			"time_zone":        job.Periodic.TimeZone,
		}
		periodic = append(periodic, p)
	}
	sw.Set("periodic", periodic)

	update, err := readUpdate(d, job.Update)
	if err != nil {
		return err
	}
	sw.Set("update", update)

	return sw.Error()
}

func resourceJobV2Deregister(d *schema.ResourceData, meta interface{}) error {
	client := meta.(ProviderConfig).client

	_, _, err := client.Jobs().Deregister(d.Id(), true, nil)
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

func boolToPtr(b bool) *bool {
	return &b
}

func intToPtr(i int) *int {
	return &i
}

func getString(d interface{}, name string) *string {
	if m, ok := d.(map[string]interface{}); ok {
		return strToPtr(m[name].(string))
	}
	return strToPtr(d.(*schema.ResourceData).Get(name).(string))
}

func getBool(d interface{}, name string) *bool {
	if m, ok := d.(map[string]interface{}); ok {
		return boolToPtr(m[name].(bool))
	}
	return boolToPtr(d.(*schema.ResourceData).Get(name).(bool))
}

func getInt(d interface{}, name string) *int {
	if m, ok := d.(map[string]interface{}); ok {
		return intToPtr(m[name].(int))
	}
	return intToPtr(d.(*schema.ResourceData).Get(name).(int))
}

func getMapOfString(d interface{}) map[string]string {
	res := make(map[string]string)
	for key, value := range d.(map[string]interface{}) {
		res[key] = value.(string)
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

func getJob(d *schema.ResourceData) (*api.Job, error) {
	datacenters := make([]string, 0)
	for _, dc := range d.Get("datacenters").([]interface{}) {
		datacenters = append(datacenters, dc.(string))
	}

	var parametrizedJob *api.ParameterizedJobConfig
	for _, pj := range d.Get("parameterized").([]interface{}) {
		p := pj.(map[string]interface{})

		metaRequired := make([]string, 0)
		for _, s := range p["meta_required"].([]interface{}) {
			metaRequired = append(metaRequired, s.(string))
		}

		metaOptional := make([]string, 0)
		for _, s := range p["meta_optional"].([]interface{}) {
			metaOptional = append(metaOptional, s.(string))
		}
		parametrizedJob = &api.ParameterizedJobConfig{
			Payload:      p["payload"].(string),
			MetaRequired: metaRequired,
			MetaOptional: metaOptional,
		}
	}

	var periodic *api.PeriodicConfig
	for _, pc := range d.Get("periodic").([]interface{}) {
		p := pc.(map[string]interface{})
		periodic = &api.PeriodicConfig{
			Enabled:         boolToPtr(true),
			Spec:            getString(p, "cron"),
			SpecType:        strToPtr("cron"),
			ProhibitOverlap: getBool(p, "prohibit_overlap"),
			TimeZone:        getString(p, "time_zone"),
		}
	}

	update, err := getUpdate(d.Get("update"))
	if err != nil {
		return nil, err
	}
	taskGroups, err := getTaskGroups(d.Get("group"))
	if err != nil {
		return nil, err
	}

	return &api.Job{
		Namespace:   getString(d, "namespace"),
		Priority:    getInt(d, "priority"),
		Type:        getString(d, "type"),
		Meta:        getMapOfString(d.Get("meta")),
		AllAtOnce:   getBool(d, "all_at_once"),
		Datacenters: datacenters,
		ID:          getString(d, "name"),
		Region:      getString(d, "region"),
		VaultToken:  getString(d, "vault_token"),
		ConsulToken: getString(d, "consul_token"),

		Constraints: getConstraints(d.Get("constraint")),
		Affinities:  getAffinities(d.Get("affinity")),
		Spreads:     getSpreads(d.Get("spread")),
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

	return nil
}

func getTaskGroups(d interface{}) ([]*api.TaskGroup, error) {
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

			var duration *time.Duration
			duration, err = getDuration(r["delay"])
			if err != nil {
				return nil, err
			}
			restartPolicy.Delay = duration

			duration, err = getDuration(r["interval"])
			if err != nil {
				return nil, err
			}
			restartPolicy.Interval = duration
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

		tasks, err := getTasks(g["task"])
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

		var duration *time.Duration
		duration, err = getDuration(g["shutdown_delay"])
		if err != nil {
			return nil, err
		}
		group.ShutdownDelay = duration

		duration, err = getDuration(g["stop_after_client_disconnect"])
		if err != nil {
			return nil, err
		}
		group.StopAfterClientDisconnect = duration

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

		var duration *time.Duration
		duration, err := getDuration(m["min_healthy_time"])
		if err != nil {
			return nil, err
		}
		migrateStrategy.MinHealthyTime = duration

		duration, err = getDuration(m["healthy_deadline"])
		if err != nil {
			return nil, err
		}
		migrateStrategy.HealthyDeadline = duration

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

		var duration *time.Duration
		duration, err := getDuration(r["interval"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.Interval = duration

		duration, err = getDuration(r["delay"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.Delay = duration

		duration, err = getDuration(r["max_delay"])
		if err != nil {
			return nil, err
		}
		reschedulePolicy.MaxDelay = duration

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

		var duration *time.Duration
		duration, err := getDuration(u["stagger"])
		if err != nil {
			return nil, err
		}
		update.Stagger = duration

		duration, err = getDuration(u["min_healthy_time"])
		if err != nil {
			return nil, err
		}
		update.MinHealthyTime = duration

		duration, err = getDuration(u["healthy_deadline"])
		if err != nil {
			return nil, err
		}
		update.HealthyDeadline = duration

		duration, err = getDuration(u["progress_deadline"])
		if err != nil {
			return nil, err
		}
		update.ProgressDeadline = duration

		return update, nil
	}
	return nil, nil
}

func getTasks(d interface{}) ([]*api.Task, error) {
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

			duration, err := getDuration(tp["splay"])
			if err != nil {
				return nil, err
			}
			template.Splay = duration

			duration, err = getDuration(tp["vault_grace"])
			if err != nil {
				return nil, err
			}
			template.VaultGrace = duration

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
			Vault:           getVault(t["vault"]),
			VolumeMounts:    volumeMounts,
		}

		duration, err := getDuration(t["kill_timeout"])
		if err != nil {
			return nil, err
		}
		task.KillTimeout = duration

		duration, err = getDuration(t["shutdown_delay"])
		if err != nil {
			return nil, err
		}
		if duration != nil {
			task.ShutdownDelay = *duration
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func getNetworks(d interface{}) []*api.NetworkResource {
	networks := make([]*api.NetworkResource, 0)

	for _, nr := range d.([]interface{}) {
		n := nr.(map[string]interface{})

		// TODO(remi): find what to do with the ports here
		network := &api.NetworkResource{
			Mode:  n["mode"].(string),
			MBits: getInt(n, "mbits"),
		}

		for _, dns := range n["dns"].([]interface{}) {
			d := dns.(map[string]interface{})

			network.DNS = &api.DNSConfig{}

			servers := make([]string, 0)
			for _, s := range d["servers"].([]interface{}) {
				network.DNS.Servers = append(servers, s.(string))
			}
			searches := make([]string, 0)
			for _, s := range d["searches"].([]interface{}) {
				network.DNS.Searches = append(searches, s.(string))
			}
			options := make([]string, 0)
			for _, s := range d["options"].([]interface{}) {
				network.DNS.Options = append(options, s.(string))
			}
		}

		networks = append(networks, network)
	}

	return networks
}

func getServices(d interface{}) ([]*api.Service, error) {
	services := make([]*api.Service, 0)

	for _, svc := range d.([]interface{}) {
		s := svc.(map[string]interface{})

		tags := make([]string, 0)
		for _, t := range s["tags"].([]interface{}) {
			tags = append(tags, t.(string))
		}

		canaryTags := make([]string, 0)
		for _, t := range s["canary_tags"].([]interface{}) {
			canaryTags = append(canaryTags, t.(string))
		}

		checks := make([]api.ServiceCheck, 0)
		for _, cks := range s["check"].([]interface{}) {
			c := cks.(map[string]interface{})

			args := make([]string, 0)
			for _, a := range c["args"].([]interface{}) {
				args = append(args, a.(string))
			}

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
				Args:                   args,
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

			duration, err := getDuration(c["timeout"])
			if err != nil {
				return nil, err
			}
			if duration != nil {
				check.Timeout = *duration
			}

			duration, err = getDuration(c["interval"])
			if err != nil {
				return nil, err
			}
			if duration != nil {
				check.Interval = *duration
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

				tags := make([]string, 0)
				for _, t := range ss["tags"].([]interface{}) {
					tags = append(tags, t.(string))
				}

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
					Tags:  tags,
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
			Tags:              tags,
			CanaryTags:        canaryTags,
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

		policies := make([]string, 0)
		for _, p := range v["policies"].([]interface{}) {
			policies = append(policies, p.(string))
		}

		return &api.Vault{
			Policies:     policies,
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

func readGroups(d *schema.ResourceData, groups []*api.TaskGroup) (interface{}, error) {
	res := make([]interface{}, 0)

	// we have to look for the groups the user created in its configuration as
	// we will need to set the "ephemeral_disk" and "restart" block only if they
	// created one or the value for the block is different from the default one

	groupsConfig := d.Get("group").([]interface{})

	for i, g := range groups {
		currentConfig := groupsConfig[i].(map[string]interface{})

		ephemeralDisk := make([]interface{}, 0)

		if g.EphemeralDisk != nil {
			disk := map[string]interface{}{
				"migrate": *g.EphemeralDisk.Migrate,
				"size":    *g.EphemeralDisk.SizeMB,
				"sticky":  *g.EphemeralDisk.Sticky,
			}

			defaultValue := map[string]interface{}{
				"migrate": false,
				"size":    300,
				"sticky":  false,
			}

			set := currentConfig["ephemeral_disk"].([]interface{})

			if len(set) > 0 || !reflect.DeepEqual(disk, defaultValue) {
				ephemeralDisk = append(ephemeralDisk, disk)
			}
		}

		restart := make([]interface{}, 0)
		if g.RestartPolicy != nil {
			r := map[string]interface{}{
				"attempts": *g.RestartPolicy.Attempts,
				"delay":    g.RestartPolicy.Delay.String(),
				"interval": g.RestartPolicy.Interval.String(),
				"mode":     *g.RestartPolicy.Mode,
			}

			// The default depends on the job type
			var defaultValue map[string]interface{}
			_type := d.Get("type").(string)
			if _type == "service" {
				defaultValue = map[string]interface{}{
					"attempts": 2,
					"delay":    "15s",
					"interval": "30m0s",
					"mode":     "fail",
				}
			} else if _type == "batch" {
				defaultValue = map[string]interface{}{
					"attempts": 3,
					"delay":    "15s",
					"interval": "24h0m0s",
					"mode":     "fail",
				}
			} else if _type == "system" {
				defaultValue = map[string]interface{}{
					"attempts": 2,
					"delay":    "15s",
					"interval": "30m0s",
					"mode":     "fail",
				}
			} else {
				return nil, fmt.Errorf("%q is not supported", _type)
			}

			set := currentConfig["restart"].([]interface{})

			if len(set) > 0 || !reflect.DeepEqual(r, defaultValue) {
				restart = append(restart, r)
			}
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

		isSet := len(currentConfig["reschedule"].([]interface{})) > 0
		reschedule, err := readReschedule(d, isSet, g.ReschedulePolicy)
		if err != nil {
			return nil, err
		}

		isSet = len(currentConfig["migrate"].([]interface{})) > 0

		group := map[string]interface{}{
			"name":           g.Name,
			"meta":           g.Meta,
			"count":          g.Count,
			"constraint":     readConstraints(g.Constraints),
			"affinity":       readAffinities(g.Affinities),
			"spread":         readSpreads(g.Spreads),
			"ephemeral_disk": ephemeralDisk,
			"migrate":        readMigrate(isSet, g.Migrate),
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

func readMigrate(isSet bool, migrate *api.MigrateStrategy) interface{} {
	if migrate == nil {
		return nil
	}

	res := map[string]interface{}{
		"max_parallel":     *migrate.MaxParallel,
		"health_check":     *migrate.HealthCheck,
		"min_healthy_time": migrate.MinHealthyTime.String(),
		"healthy_deadline": migrate.HealthyDeadline.String(),
	}

	defaultValue := map[string]interface{}{
		"max_parallel":     1,
		"health_check":     "checks",
		"min_healthy_time": "10s",
		"healthy_deadline": "5m0s",
	}

	if !isSet && reflect.DeepEqual(res, defaultValue) {
		return nil
	}

	return []interface{}{res}
}

func readReschedule(d *schema.ResourceData, isSet bool, reschedule *api.ReschedulePolicy) (interface{}, error) {
	if reschedule == nil {
		return nil, nil
	}

	res := map[string]interface{}{
		"attempts":       *reschedule.Attempts,
		"interval":       reschedule.Interval.String(),
		"delay":          reschedule.Delay.String(),
		"delay_function": *reschedule.DelayFunction,
		"max_delay":      reschedule.MaxDelay.String(),
		"unlimited":      *reschedule.Unlimited,
	}

	var defaultValue map[string]interface{}
	// The default value depends on the type of job
	_type := d.Get("type").(string)
	if _type == "service" {
		defaultValue = map[string]interface{}{
			"attempts":       0,
			"interval":       "0s",
			"delay":          "30s",
			"delay_function": "exponential",
			"max_delay":      "1h0m0s",
			"unlimited":      true,
		}
	} else if _type == "batch" {
		defaultValue = map[string]interface{}{
			"attempts":       1,
			"interval":       "24h0m0s",
			"delay":          "5s",
			"delay_function": "constant",
			"max_delay":      "0s",
			"unlimited":      false,
		}
	} else if _type == "system" {
		defaultValue = map[string]interface{}{}
	} else {
		// This should not happen
		return nil, fmt.Errorf("%q is not supported", _type)
	}

	if !isSet && reflect.DeepEqual(res, defaultValue) {
		return nil, nil
	}

	return []interface{}{res}, nil
}

func readUpdate(d *schema.ResourceData, update *api.UpdateStrategy) (interface{}, error) {
	if update == nil {
		return nil, nil
	}

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

	// If the value returned by the API and the user did not set the block
	// we must not create it
	var defaultValue map[string]interface{}
	_type := d.Get("type").(string)
	// The default depends ont he job type
	if _type == "service" {
		defaultValue = map[string]interface{}{
			"auto_promote":      false,
			"auto_revert":       false,
			"canary":            0,
			"health_check":      "",
			"healthy_deadline":  "0s",
			"max_parallel":      1,
			"min_healthy_time":  "0s",
			"progress_deadline": "0s",
			"stagger":           "30s",
		}
	} else if _type == "batch" || _type == "system" {
		defaultValue = map[string]interface{}{
			"auto_promote":      false,
			"auto_revert":       false,
			"canary":            0,
			"health_check":      "",
			"healthy_deadline":  "0s",
			"max_parallel":      0,
			"min_healthy_time":  "0s",
			"progress_deadline": "0s",
			"stagger":           "0s",
		}
	} else {
		return nil, fmt.Errorf("%q is not supported", _type)
	}

	isSet := len(d.Get("update").([]interface{})) > 0

	if !isSet && reflect.DeepEqual(res, defaultValue) {
		return nil, nil
	}
	return []interface{}{res}, nil
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

		// TODO(remi): check what to do for the port
		network := map[string]interface{}{
			"mbits": *n.MBits,
			"mode":  n.Mode,
			"port":  []interface{}{nil},
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
		return nil
	}

	res := map[string]interface{}{
		"max_files":     *logs.MaxFiles,
		"max_file_size": *logs.MaxFileSizeMB,
	}

	defaultValue := map[string]interface{}{
		"max_files":     10,
		"max_file_size": 10,
	}

	if reflect.DeepEqual(res, defaultValue) {
		return nil
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

	defaultValue := map[string]interface{}{
		"cpu":     100,
		"device":  []interface{}{},
		"memory":  300,
		"network": []interface{}{},
	}

	if reflect.DeepEqual(res, defaultValue) {
		return nil
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
