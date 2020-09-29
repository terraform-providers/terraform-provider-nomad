package nomad

import (
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func diffSupressDuration(k, old, new string, d *schema.ResourceData) bool {
	o, err := time.ParseDuration(old)
	if err != nil {
		return false
	}
	n, err := time.ParseDuration(new)
	if err != nil {
		return false
	}
	return o.Seconds() == n.Seconds()
}

func getJobFields() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"id": {
			Type:     schema.TypeString,
			ForceNew: true,
			Computed: true,
			Optional: true,
		},
		"namespace": {
			Type:     schema.TypeString,
			ForceNew: true,
			Default:  "default",
			Optional: true,
		},
		"priority": {
			Type:     schema.TypeInt,
			Default:  50,
			Optional: true,
		},
		"type": {
			Type:     schema.TypeString,
			ForceNew: true,
			Default:  "service",
			Optional: true,
		},
		"region": {
			Type:     schema.TypeString,
			Computed: true,
			Optional: true,
		},
		"meta": {
			Type:     schema.TypeMap,
			Elem:     &schema.Schema{Type: schema.TypeString},
			Optional: true,
		},
		"all_at_once": {
			Type:     schema.TypeBool,
			Optional: true,
		},
		"datacenters": {
			Type:     schema.TypeList,
			Elem:     &schema.Schema{Type: schema.TypeString},
			Optional: true,
		},
		"name": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"vault_token": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"consul_token": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"vault":      getVaultFields(),
		"migrate":    getMigrateFields(),
		"reschedule": getRescheduleFields(),
		"constraint": getConstraintFields(),
		"affinity":   getAffinityFields(),
		"spread":     getSpreadFields(),
		"group":      getGroupFields(),
		"parameterized": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"meta_optional": {
						Type:     schema.TypeList,
						Elem:     &schema.Schema{Type: schema.TypeString},
						Optional: true,
					},
					"meta_required": {
						Type:     schema.TypeList,
						Elem:     &schema.Schema{Type: schema.TypeString},
						Optional: true,
					},
					"payload": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
		"periodic": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"cron": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"prohibit_overlap": {
						Type:     schema.TypeBool,
						Optional: true,
					},
					"time_zone": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
		"update": getUpdateFields(),
	}
}

func getSpreadFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"attribute": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"weight": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"target": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"value": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"percent": {
								Type:     schema.TypeInt,
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
}

func getConstraintFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"attribute": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"operator": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"value": {
					Type:     schema.TypeString,
					Optional: true,
				},
			},
		},
	}
}

func getAffinityFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"attribute": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"operator": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"value": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"weight": {
					Type:     schema.TypeInt,
					Optional: true,
				},
			},
		},
	}
}

func getVaultFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"change_mode": {
					Type:     schema.TypeString,
					Default:  "restart",
					Optional: true,
				},
				"change_signal": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"env": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"namespace": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"policies": {
					Type:     schema.TypeList,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
			},
		},
	}
}

func getMigrateFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"health_check": {
					Type:     schema.TypeString,
					Default:  "checks",
					Optional: true,
				},
				"healthy_deadline": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "5m0s",
					Optional:         true,
				},
				"max_parallel": {
					Type:     schema.TypeInt,
					Default:  1,
					Optional: true,
				},
				"min_healthy_time": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "10s",
					Optional:         true,
				},
			},
		},
	}
}

func getRescheduleFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"interval": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Optional:         true,
				},
				"delay": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Optional:         true,
				},
				"max_delay": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Optional:         true,
				},
				"attempts": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"delay_function": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"unlimited": {
					Type:     schema.TypeBool,
					Optional: true,
				},
			},
		},
	}
}

func getResourcesFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"cpu": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"memory": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"device": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"count": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"constraint": getConstraintFields(),
							"affinity":   getAffinityFields(),
						},
					},
				},
				"network": getNetworkFields(),
			},
		},
	}
}

func getLogsFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"max_files": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"max_file_size": {
					Type:     schema.TypeInt,
					Optional: true,
				},
			},
		},
	}
}

func getServiceFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"canary_meta": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"address_mode": {
					Type:     schema.TypeString,
					Default:  "auto",
					Optional: true,
				},
				"meta": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"port": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"tags": {
					Type:     schema.TypeList,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"canary_tags": {
					Type:     schema.TypeList,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"enable_tag_override": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"task": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"check": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"interval": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Optional:         true,
							},
							"timeout": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Optional:         true,
							},
							"name": {
								Type:     schema.TypeString,
								Computed: true,
								Optional: true,
							},
							"address_mode": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"args": {
								Type:     schema.TypeList,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Optional: true,
							},
							"command": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"grpc_service": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"grpc_use_tls": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"initial_status": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"success_before_passing": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"failures_before_critical": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"method": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"path": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"expose": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"port": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"protocol": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"task": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"type": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"tls_skip_verify": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"check_restart": {
								Type:     schema.TypeList,
								Optional: true,
								MaxItems: 1,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"grace": {
											Type:             schema.TypeString,
											DiffSuppressFunc: diffSupressDuration,
											Optional:         true,
										},
										"limit": {
											Type:     schema.TypeInt,
											Optional: true,
										},
										"ignore_warnings": {
											Type:     schema.TypeBool,
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
				"connect": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"native": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"sidecar_service": {
								Type:     schema.TypeList,
								Optional: true,
								MaxItems: 1,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"tags": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"port": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"proxy": {
											Type:     schema.TypeList,
											Optional: true,
											MaxItems: 1,
											Elem: &schema.Resource{
												Schema: map[string]*schema.Schema{
													"local_service_address": {
														Type:     schema.TypeString,
														Optional: true,
													},
													"local_service_port": {
														Type:     schema.TypeInt,
														Optional: true,
													},
													"config": {
														Type:     schema.TypeMap,
														Elem:     &schema.Schema{Type: schema.TypeString},
														Optional: true,
													},
													"upstreams": {
														Type:     schema.TypeList,
														Optional: true,
														MaxItems: 1,
														Elem: &schema.Resource{
															Schema: map[string]*schema.Schema{
																"local_bind_port": {
																	Type:     schema.TypeInt,
																	Optional: true,
																},
																"destination_name": {
																	Type:     schema.TypeString,
																	Optional: true,
																},
															},
														},
													},
													"expose": {
														Type:     schema.TypeList,
														Optional: true,
														MaxItems: 1,
														Elem: &schema.Resource{
															Schema: map[string]*schema.Schema{
																"path": {
																	Type:     schema.TypeList,
																	Optional: true,
																	Elem: &schema.Resource{
																		Schema: map[string]*schema.Schema{
																			"path": {
																				Type:     schema.TypeString,
																				Optional: true,
																			},
																			"protocol": {
																				Type:     schema.TypeString,
																				Optional: true,
																			},
																			"local_path_port": {
																				Type:     schema.TypeInt,
																				Optional: true,
																			},
																			"listener_port": {
																				Type:     schema.TypeString,
																				Optional: true,
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							"sidecar_task": {
								Type:     schema.TypeList,
								Optional: true,
								MaxItems: 1,
								Elem: &schema.Resource{
									Schema: map[string]*schema.Schema{
										"kill_timeout": {
											Type:             schema.TypeString,
											DiffSuppressFunc: diffSupressDuration,
											Optional:         true,
										},
										"shutdown_delay": {
											Type:             schema.TypeString,
											DiffSuppressFunc: diffSupressDuration,
											Optional:         true,
										},
										"meta": {
											Type:     schema.TypeMap,
											Elem:     &schema.Schema{Type: schema.TypeString},
											Optional: true,
										},
										"name": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"driver": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"user": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"config": {
											Type:     schema.TypeMap,
											Elem:     &schema.Schema{Type: schema.TypeString},
											Optional: true,
										},
										"env": {
											Type:     schema.TypeMap,
											Elem:     &schema.Schema{Type: schema.TypeString},
											Optional: true,
										},
										"kill_signal": {
											Type:     schema.TypeString,
											Optional: true,
										},
										"resources": getResourcesFields(),
										"logs":      getLogsFields(),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func getGroupFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MinItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"shutdown_delay": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Optional:         true,
				},
				"stop_after_client_disconnect": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Optional:         true,
				},
				"count": {
					Type:     schema.TypeInt,
					Default:  1,
					Optional: true,
				},
				"meta": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"constraint": getConstraintFields(),
				"affinity":   getAffinityFields(),
				"spread":     getSpreadFields(),
				"ephemeral_disk": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"migrate": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"size": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"sticky": {
								Type:     schema.TypeBool,
								Optional: true,
							},
						},
					},
				},
				"migrate":    getMigrateFields(),
				"network":    getNetworkFields(),
				"reschedule": getRescheduleFields(),
				"restart": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"delay": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Optional:         true,
							},
							"interval": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Optional:         true,
							},
							"attempts": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"mode": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"service": getServiceFields(),
				"task":    getTaskFields(),
				"vault":   getVaultFields(),
				"volume": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"name": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"type": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"source": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"read_only": {
								Type:     schema.TypeBool,
								Optional: true,
							},
						},
					},
				},
				"update": getUpdateFields(),
			},
		},
	}
}

func getTaskFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Required: true,
		MinItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"config": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"kill_timeout": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "5s",
					Optional:         true,
				},
				"shutdown_delay": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "0s",
					Optional:         true,
				},
				"env": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"meta": {
					Type:     schema.TypeMap,
					Elem:     &schema.Schema{Type: schema.TypeString},
					Optional: true,
				},
				"driver": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"kill_signal": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"leader": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"user": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"kind": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"artifact": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"destination": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"mode": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"options": {
								Type:     schema.TypeMap,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Optional: true,
							},
							"source": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"constraint": getConstraintFields(),
				"affinity":   getAffinityFields(),
				"dispatch_payload": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"file": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"lifecycle": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"hook": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"sidecar": {
								Type:     schema.TypeBool,
								Optional: true,
							},
						},
					},
				},
				"logs":      getLogsFields(),
				"resources": getResourcesFields(),
				"service":   getServiceFields(),
				"template": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"change_mode": {
								Type:     schema.TypeString,
								Default:  "restart",
								Optional: true,
							},
							"left_delimiter": {
								Type:     schema.TypeString,
								Default:  "{{",
								Optional: true,
							},
							"perms": {
								Type:     schema.TypeString,
								Default:  "0644",
								Optional: true,
							},
							"right_delimiter": {
								Type:     schema.TypeString,
								Default:  "}}",
								Optional: true,
							},
							"splay": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Default:          "5s",
								Optional:         true,
							},
							"vault_grace": {
								Type:             schema.TypeString,
								DiffSuppressFunc: diffSupressDuration,
								Default:          "15s",
								Optional:         true,
							},
							"change_signal": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"data": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"destination": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"env": {
								Type:     schema.TypeBool,
								Optional: true,
							},
							"source": {
								Type:     schema.TypeString,
								Optional: true,
							},
						},
					},
				},
				"vault": getVaultFields(),
				"volume_mount": {
					Type:     schema.TypeList,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"volume": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"destination": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"read_only": {
								Type:     schema.TypeBool,
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
}

func getNetworkFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"mbits": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"mode": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"port": {
					Type:     schema.TypeSet,
					Optional: true,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"label": {
								Type:     schema.TypeString,
								Optional: true,
							},
							"to": {
								Type:     schema.TypeInt,
								Optional: true,
							},
							"host_network": {
								Type:     schema.TypeString,
								Default:  "default",
								Optional: true,
							},
							"static": {
								Type:     schema.TypeInt,
								Optional: true,
							},
						},
					},
				},
				"dns": {
					Type:     schema.TypeList,
					Optional: true,
					MaxItems: 1,
					Elem: &schema.Resource{
						Schema: map[string]*schema.Schema{
							"servers": {
								Type:     schema.TypeList,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Optional: true,
							},
							"searches": {
								Type:     schema.TypeList,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Optional: true,
							},
							"options": {
								Type:     schema.TypeList,
								Elem:     &schema.Schema{Type: schema.TypeString},
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
}

func getUpdateFields() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"healthy_deadline": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "5m",
					Optional:         true,
				},
				"min_healthy_time": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "10s",
					Optional:         true,
				},
				"progress_deadline": {
					Type:             schema.TypeString,
					DiffSuppressFunc: diffSupressDuration,
					Default:          "10m",
					Optional:         true,
				},
				"stagger": {
					Type:     schema.TypeString,
					Default:  "30s",
					Optional: true,
				},
				"max_parallel": {
					Type:     schema.TypeInt,
					Default:  1,
					Optional: true,
				},
				"health_check": {
					Type:     schema.TypeString,
					Optional: true,
				},
				"auto_revert": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"auto_promote": {
					Type:     schema.TypeBool,
					Optional: true,
				},
				"canary": {
					Type:     schema.TypeInt,
					Optional: true,
				},
			},
		},
	}
}
