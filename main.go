package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	tf5server "github.com/hashicorp/terraform-plugin-go/tfprotov5/server"
	tfmux "github.com/hashicorp/terraform-plugin-mux"
	protocol "github.com/hashicorp/terraform-provider-nomad/internal/protocolprovider"
	"github.com/hashicorp/terraform-provider-nomad/nomad"
)

func main() {
	ctx := context.Background()

	provider := nomad.Provider()
	muxed, err := tfmux.NewSchemaServerFactory(ctx, provider.GRPCProvider, protocol.Server(provider))
	if err != nil {
		panic(err)
	}

	err = tf5server.Serve("registry.terraform.io/hashicorp/nomad", func() tfprotov5.ProviderServer {
		return muxed.Server()
	})
	if err != nil {
		panic(err)
	}
}
