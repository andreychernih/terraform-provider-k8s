module github.com/banzaicloud/terraform-provider-k8s

go 1.13

require (
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/hashicorp/terraform-plugin-sdk v1.15.0
	github.com/itchyny/gojq v0.12.4
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/pkg/errors v0.9.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kubectl v0.22.1
	sigs.k8s.io/controller-runtime v0.6.2
)
