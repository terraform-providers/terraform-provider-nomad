package protocol

import (
	"context"
	"reflect"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-nomad/nomad"
)

type server struct {
	err      error
	provider *schema.Provider

	resourceSchemas map[string]*tfprotov5.Schema

	resourceRouter
	dataSourceRouter
}

func (s server) GetProviderSchema(ctx context.Context, req *tfprotov5.GetProviderSchemaRequest) (*tfprotov5.GetProviderSchemaResponse, error) {
	return &tfprotov5.GetProviderSchemaResponse{
		ResourceSchemas: s.resourceSchemas,
	}, s.err
}

func (s server) PrepareProviderConfig(ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	return nil, nil
}

func (s server) ConfigureProvider(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	var diags []*tfprotov5.Diagnostic
	return &tfprotov5.ConfigureProviderResponse{
		Diagnostics: diags,
	}, nil
}

func (s server) StopProvider(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return &tfprotov5.StopProviderResponse{}, nil
}

func (s server) getClient() *api.Client {
	return s.provider.Meta().(nomad.ProviderConfig).Client
}

func Server(provider *schema.Provider) func() tfprotov5.ProviderServer {
	return func() tfprotov5.ProviderServer {
		c, err := NewConverter(reflect.TypeOf(api.Job{}))
		s := server{
			// We keep a reference to the other provider so that we can use the
			// same client
			provider: provider,

			// If we get an error now we store it in the server so that we can
			// return it in GetProviderSchema()
			err: err,
			resourceSchemas: map[string]*tfprotov5.Schema{
				"nomad_job_v2": {
					Version: 1,
					Block: &tfprotov5.SchemaBlock{
						Version: 1,
						Attributes: []*tfprotov5.SchemaAttribute{
							{
								Name:     "id",
								Type:     tftypes.String,
								Computed: true,
							},
							{
								Name:     "out",
								Type:     tftypes.DynamicPseudoType,
								Computed: true,
							},
						},
						BlockTypes: []*tfprotov5.SchemaNestedBlock{
							{
								TypeName: "job",
								Nesting:  tfprotov5.SchemaNestedBlockNestingModeMap,
								Block:    c.Schema,
							},
						},
					},
				},
			},
			resourceRouter: resourceRouter{},
		}

		s.resourceRouter["nomad_job_v2"] = nomadJob{
			s:         &s,
			converter: c,
		}

		return s
	}
}
