package main

import (
	"github.com/banzaicloud/terraform-provider-k8s/k8s"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: k8s.Provider,
	})
}
