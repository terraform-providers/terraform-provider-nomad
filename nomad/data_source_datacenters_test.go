package nomad

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceDatacenters_Basic(t *testing.T) {
	dataSourceName := "data.nomad_datacenters.dcs"

	resource.ParallelTest(t, resource.TestCase{
		Providers: testProviders,
		Steps: []resource.TestStep{
			{
				Config: testResourceDataSourceDatacentersConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dataSourceName, "datacenters.#", "1"),
					resource.TestCheckResourceAttr(dataSourceName, "datacenters.0", "dc1"),
				),
			},
		},
	})
}

func TestFilterDatacenters(t *testing.T) {
	cases := []struct {
		name       string
		nodes      []*api.NodeListStub
		prefix     string
		ignoreDown bool
		want       []string
	}{
		{
			name: "single datacenter",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "dc1"},
			},
			want: []string{"dc1"},
		},
		{
			name: "multiple datacenters",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "dc1"},
				&api.NodeListStub{Datacenter: "dc2"},
			},
			want: []string{"dc1", "dc2"},
		},
		{
			name: "duplicate datacenter",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "dc1"},
				&api.NodeListStub{Datacenter: "dc1"},
				&api.NodeListStub{Datacenter: "dc2"},
			},
			want: []string{"dc1", "dc2"},
		},
		{
			name: "filter down nodes",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "dc1"},
				&api.NodeListStub{Datacenter: "dc1"},
				&api.NodeListStub{Datacenter: "dc2", Status: "down"},
			},
			ignoreDown: true,
			want:       []string{"dc1"},
		},
		{
			name: "filter with prefix",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "prod-1"},
				&api.NodeListStub{Datacenter: "prod-2"},
				&api.NodeListStub{Datacenter: "dev"},
			},
			prefix: "prod",
			want:   []string{"prod-1", "prod-2"},
		},
		{
			name: "filter with prefix, empty result",
			nodes: []*api.NodeListStub{
				&api.NodeListStub{Datacenter: "prod-1"},
				&api.NodeListStub{Datacenter: "prod-2"},
				&api.NodeListStub{Datacenter: "dev"},
			},
			prefix: "not-there",
			want:   []string{},
		},
		{
			name:  "empty list",
			nodes: []*api.NodeListStub{},
			want:  []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := filterDatacenters(c.nodes, c.prefix, c.ignoreDown)

			if diff := cmp.Diff(got, c.want); diff != "" {
				t.Fatalf("datacenters mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

var testResourceDataSourceDatacentersConfig = `
data "nomad_datacenters" "dcs" {}
`
