package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type nomadJob struct {
	converter *Converter
	s         *server
}

func (j nomadJob) Apply(modifyIndex uint64, job *api.Job) error {
	_, _, err := j.s.getClient().Jobs().EnforceRegister(job, modifyIndex, nil)
	return err
}

func (j nomadJob) Read(id string) (*api.Job, error) {
	job, _, err := j.s.getClient().Jobs().Info(id, nil)

	if err != nil && strings.Contains(err.Error(), "404") {
		return nil, nil
	}

	return job, err
}

func (j nomadJob) Destroy(id string) error {
	_, _, err := j.s.getClient().Jobs().Deregister(id, true, nil)
	return err
}

func (j nomadJob) unmarshal(state *tfprotov5.DynamicValue) (string, *api.Job, error) {
	schema := j.Schema(false)
	_, v, err := unmarshalDynamicValueObject(state, schema)
	if err != nil {
		return "", nil, fmt.Errorf("error while decoding state: %s", err)
	}

	var id string
	if err := v["id"].As(&id); err != nil {
		return "", nil, fmt.Errorf("failed to decode id: %s", err)
	}

	var val map[string]tftypes.Value
	if err := v["job"].As(&val); err != nil {
		return "", nil, fmt.Errorf("failed to decode job: %s", err)
	}

	if len(val) > 1 {
		return "", nil, fmt.Errorf("wrong number of jobs found: %#v", val)
	}
	job := &api.Job{}
	for ID, v := range val {
		if err := j.converter.AsGo(v, job); err != nil {
			return "", nil, fmt.Errorf("failed to convert job: %s", err)
		}
		// We now override the ID here, it means that the user could set some
		// weird configuration like
		//
		//  resource "nomad_job_v2" "test" {
		//    job "foo" {
		//      id = "bar"
		//      // ...
		//    }
		//  }
		//
		// and the "bar" ID would be overwritten by "foo" but this is how the
		// Nomad CLI client works too so it's not an issue as long as we are
		// consistent
		job.ID = &ID
	}

	return id, job, nil
}

func (j nomadJob) ReadResource(_ context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	id, _, err := j.unmarshal(req.CurrentState)
	if err != nil {
		return nil, err
	}

	var diff api.JobDiff
	if len(req.Private) > 0 {
		var p private
		if err = json.Unmarshal(req.Private, &p); err != nil {
			return nil, err
		}
		config := &tfprotov5.DynamicValue{
			MsgPack: p.Config,
		}
		_, job, err := j.unmarshal(config)
		if err != nil {
			return nil, err
		}
		plan, _, err := j.s.getClient().Jobs().Plan(job, true, nil)
		if err != nil {
			return nil, err
		}
		diff = *plan.Diff
	}

	job, err := j.Read(id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		// The job does not exist anymore, we return an empty state to signal it
		return &tfprotov5.ReadResourceResponse{
			Private: req.Private,
		}, nil
	}

	// Export the read job in the 'out' attribute
	schema := j.Schema(false)
	val, err := req.CurrentState.Unmarshal(schema)
	if err != nil {
		return nil, err
	}
	var v map[string]tftypes.Value
	if err = val.As(&v); err != nil {
		return nil, err
	}

	out := j.converter.AsValue(job)
	jobValue := v["job"]
	if hasChanges(diff) {
		jobValue = tftypes.NewValue(
			schema.AttributeTypes["job"],
			map[string]tftypes.Value{
				*job.ID: out,
			},
		)
	}

	val = tftypes.NewValue(schema, map[string]tftypes.Value{
		"id":  tftypes.NewValue(tftypes.String, *job.ID),
		"job": jobValue,
		"out": out,
	})
	state, err := tfprotov5.NewDynamicValue(schema, val)
	if err != nil {
		return nil, fmt.Errorf("foo: %s", err)
	}

	return &tfprotov5.ReadResourceResponse{
		NewState: &state,
		Private:  req.Private,
	}, nil
}

func hasChanges(diff api.JobDiff) bool {
	if len(diff.Fields)+len(diff.Objects) > 0 {
		return true
	}

	for _, group := range diff.TaskGroups {
		if len(group.Fields)+len(group.Objects) > 0 {
			return true
		}

		for _, task := range group.Tasks {
			if len(task.Fields)+len(task.Objects) > 0 {
				return true
			}
		}
	}

	return false
}

type private struct {
	JobModifyIndex uint64
	Config         []byte
}

func (j nomadJob) PlanResourceChange(_ context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	oldID, oldJob, err := j.unmarshal(req.PriorState)
	if err != nil {
		return nil, err
	}

	_, job, err := j.unmarshal(req.ProposedNewState)
	if err != nil {
		return nil, err
	}

	state := *req.Config
	schema := j.Schema(false)
	val, err := req.Config.Unmarshal(schema)
	if err != nil {
		return nil, err
	}
	var v map[string]tftypes.Value
	if err = val.As(&v); err != nil {
		return nil, err
	}
	configJob := v["job"]

	val, err = req.PriorState.Unmarshal(schema)
	if err != nil {
		return nil, err
	}
	if err = val.As(&v); err != nil {
		return nil, err
	}
	priorJob := v["job"]

	out := tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
	if configJob.Equal(priorJob) {
		val, err = req.ProposedNewState.Unmarshal(schema)
		if err != nil {
			return nil, err
		}
		if err = val.As(&v); err != nil {
			return nil, err
		}
		out = v["out"]
	}

	// When the job is created for the first time the ID will be empty, so we
	// fill it in during the plan. We also mark 'out' as unknow since it will
	// change.
	val = tftypes.NewValue(schema, map[string]tftypes.Value{
		"id":  tftypes.NewValue(tftypes.String, *job.ID),
		"job": configJob,
		"out": out,
	})
	state, err = tfprotov5.NewDynamicValue(schema, val)
	if err != nil {
		return nil, err
	}

	p := private{
		Config: req.Config.MsgPack,
	}

	if oldID != "" {
		_, job, err = j.unmarshal(req.Config)
		if err != nil {
			return nil, err
		}

		plan, _, err := j.s.getClient().Jobs().Plan(job, false, nil)
		if err != nil {
			return nil, fmt.Errorf("foobar: %s", err)
		}

		p.JobModifyIndex = plan.JobModifyIndex
	}

	private, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	return &tfprotov5.PlanResourceChangeResponse{
		PlannedState:    &state,
		PlannedPrivate:  private,
		RequiresReplace: requiresReplace(oldJob, job),
	}, nil
}

func requiresReplace(old, new *api.Job) []*tftypes.AttributePath {
	// This conditions needs to be kept in sync with
	// https://github.com/hashicorp/nomad/blob/402b19c3b0e824e96e8118f66906f7d89d0b6a2c/nomad/job_endpoint.go#L1811-L1846
	// Maybe this is returned during the plan but I did not find it

	res := []*tftypes.AttributePath{}
	path := tftypes.NewAttributePath().WithAttributeName("job").WithElementKeyString(*new.ID)

	// Type transitions are disallowed
	if old.Type != new.Type && old.Type != nil && *old.Type != "" {
		res = append(res, path.WithAttributeName("type"))
	}

	// Transitioning to/from periodic is disallowed
	if old.IsPeriodic() && !new.IsPeriodic() {
		res = append(res, path.WithAttributeName("periodic"))
	}
	if new.IsPeriodic() && !old.IsPeriodic() {
		res = append(res, path.WithAttributeName("periodic"))
	}

	// Transitioning to/from parameterized is disallowed
	if old.IsParameterized() && !new.IsParameterized() {
		res = append(res, path.WithAttributeName("parameterized"))
	}
	if new.IsParameterized() && !old.IsParameterized() {
		res = append(res, path.WithAttributeName("parameterized"))
	}

	if old.ID != new.ID {
		res = append(res, tftypes.NewAttributePath().WithAttributeName("id"))
	}

	return res
}

func (j nomadJob) ApplyResourceChange(_ context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	id, _, err := j.unmarshal(req.PriorState)
	if err != nil {
		return nil, err
	}

	// If there is no planned state, it means we must destroy the resource
	if val, err := req.PlannedState.Unmarshal(tftypes.DynamicPseudoType); err == nil && val.IsNull() {
		err := j.Destroy(id)
		return &tfprotov5.ApplyResourceChangeResponse{
			NewState:    req.PlannedState,
			Diagnostics: wrap(err, "Failed to deregister job"),
		}, nil
	}

	_, job, err := j.unmarshal(req.Config)
	if err != nil {
		return nil, err
	}

	var p private
	if err := json.Unmarshal(req.PlannedPrivate, &p); err != nil {
		return nil, err
	}

	if err = j.Apply(p.JobModifyIndex, job); err != nil {
		return &tfprotov5.ApplyResourceChangeResponse{
			Diagnostics: wrap(err, "Failed to register job"),
		}, nil
	}

	job, err = j.Read(*job.ID)
	if err != nil {
		return nil, err
	}

	// We must set the ID and 'out' in the new state
	schema := j.Schema(false)
	val, err := req.PlannedState.Unmarshal(schema)
	if err != nil {
		return nil, err
	}
	var v map[string]tftypes.Value
	if err = val.As(&v); err != nil {
		return nil, err
	}
	val = tftypes.NewValue(schema, map[string]tftypes.Value{
		"id":  tftypes.NewValue(tftypes.String, *job.ID),
		"job": v["job"],
		"out": j.converter.AsValue(job),
	})
	s, err := tfprotov5.NewDynamicValue(schema, val)
	if err != nil {
		return nil, err
	}

	return &tfprotov5.ApplyResourceChangeResponse{
		NewState: &s,
		Private:  req.PlannedPrivate,
	}, nil
}

func (j nomadJob) Schema(markAttributesAsOptional bool) tftypes.Object {
	return schemaAsObject(j.s.resourceSchemas["nomad_job_v2"], markAttributesAsOptional)
}

func (j nomadJob) ValidateResourceTypeConfig(_ context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	return &tfprotov5.ValidateResourceTypeConfigResponse{}, nil
}

func (j nomadJob) UpgradeResourceState(_ context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	schema := j.Schema(false)
	rawStateObject, err := req.RawState.Unmarshal(schema)
	if err != nil {
		return nil, err
	}

	rawStateValue, err := tfprotov5.NewDynamicValue(schema, rawStateObject)

	return &tfprotov5.UpgradeResourceStateResponse{
		UpgradedState: &rawStateValue,
	}, err
}

func (j nomadJob) ImportResourceState(_ context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	job, err := j.Read(req.ID)
	if err != nil {
		return &tfprotov5.ImportResourceStateResponse{
			Diagnostics: wrap(err, "Failed to import job"),
		}, nil
	}

	schema := j.Schema(false)
	val := j.converter.AsValue(job)
	val = tftypes.NewValue(
		schema,
		map[string]tftypes.Value{
			"id": tftypes.NewValue(tftypes.String, *job.ID),
			"job": tftypes.NewValue(
				schema.AttributeTypes["job"],
				map[string]tftypes.Value{
					*job.ID: val,
				},
			),
			"out": val,
		},
	)
	state, err := tfprotov5.NewDynamicValue(schema, val)
	if err != nil {
		return nil, err
	}

	return &tfprotov5.ImportResourceStateResponse{
		ImportedResources: []*tfprotov5.ImportedResource{
			{
				TypeName: req.TypeName,
				State:    &state,
			},
		},
	}, nil
}
