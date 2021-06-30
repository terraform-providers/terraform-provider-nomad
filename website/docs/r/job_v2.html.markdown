---
layout: "nomad"
page_title: "Nomad: nomad_job_v2"
sidebar_current: "docs-nomad-resource-job-v2"
description: |-
  Manages the lifecycle of registering and deregistering Nomad jobs
  (applications).
---

# nomad_job_v2

Manages a job registered in Nomad.

This can be used to initialize your cluster with system jobs, common services,
and more. In day to day Nomad use it is common for developers to submit jobs to
Nomad directly, such as for general app deployment. In addition to these apps, a
Nomad cluster often runs core system services that are ideally setup during
infrastructure creation. This resource is ideal for the latter type of job, but
can be used to manage any job within Nomad.

## Example Usage

Registering a job from a jobspec file:

```hcl
resource "nomad_job_v2" "app" {
  job "foo" {
    datacenters = ["dc1"]
    type        = "service"

    group "foo" {
      task "foo" {
        driver = "raw_exec"

        # The config stanza must be specified as a string as it cannot be
        # represented as a Terraform value
        config = jsonencode({
          command = "/bin/sleep"
          args    = ["1"]
        })

        resources {
          cpu    = 20
          memory = 10
        }

        logs {
          max_files     = 3
          max_file_size = 10
        }
      }
    }
  }
}

output "spec" {
  value = nomad_job_v2.app.out
}
```

## Argument Reference

The following arguments are supported:

- `job` `(string: <required>)` - The specification of the job to register.


## Import

The `nomad_job_v2` resource can be imported:

```sh
$ terraform import nomad_job_v2.app foo
```
