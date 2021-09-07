terraform {
  required_version = ">= 0.12"

  required_providers {
    k8s = {
      source  = "banzaicloud/k8s"
      version = ">= 0.0.1"
    }
  }
}
