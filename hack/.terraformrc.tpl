provider_installation {
  filesystem_mirror {
    path    = "PATH"
    include = ["registry.terraform.io/banzaicloud/k8s"]
  }
  direct {
    exclude = ["registry.terraform.io/banzaicloud/k8s"]
  }
}
