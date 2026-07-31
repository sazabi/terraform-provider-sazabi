package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/sazabi/terraform-provider-sazabi/internal/provider"
)

// version is set by goreleaser at release time via ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/sazabi/sazabi",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
