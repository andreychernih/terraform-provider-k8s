package k8s

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/mitchellh/mapstructure"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestAccK8sManifest_basic tests the basic functionality of the k8s_manifest resource.
func TestAccK8sManifest_basic(t *testing.T) {
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckK8sManifestDestroy,
		Steps: []resource.TestStep{
			{ // First, create the resource.
				Config: testAccK8sManifest_basic(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckK8sManifestReady("k8s_manifest.test"),
				),
			},
			{ // Then modify it.
				Config: testAccK8sManifest_modified(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckK8sManifestReady("k8s_manifest.test"),
				),
			},
		},
	})
}

// testAccK8sManifest_basic provides the terraform config
// used to test basic functionality of the k8s_manifest resource.
func testAccK8sManifest_basic(name string) string {
	return fmt.Sprintf(`resource "k8s_manifest" "test" {
  ignore_fields = [".nothing"] # Temporary workaround, see https://github.com/banzaicloud/terraform-provider-k8s/issues/84
  content = <<EOT
kind: ConfigMap
apiVersion: v1
metadata:
    name: %s
data:
    foo: bar

EOT
}
`, name)
}

// testAccK8sManifest_modified provides the terraform config
// used to test updating a k8s_manifest resource.
func testAccK8sManifest_modified(name string) string {
	return fmt.Sprintf(`resource "k8s_manifest" "test" {
  ignore_fields = [".nothing"] # Temporary workaround, see https://github.com/banzaicloud/terraform-provider-k8s/issues/84
  content = <<-EOT
kind: ConfigMap
apiVersion: v1
metadata:
    name: %s
data:
    foo: bar
    baz: bat
EOT
}
`, name)
}

func testAccCheckK8sManifestReady(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		namespace, gv, kind, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}

		groupVersion, err := k8sschema.ParseGroupVersion(gv)
		if err != nil {
			return err
		}

		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(groupVersion.WithKind(kind))
		object.SetNamespace(namespace)
		object.SetName(name)

		objectKey := client.ObjectKeyFromObject(object)

		c := testAccProvider.Meta().(*ProviderConfig).RuntimeClient

		err = c.Get(context.Background(), objectKey, object)
		if err != nil {
			return err
		}

		if s, ok := object.Object["status"]; ok {
			if statusViewer, err := polymorphichelpers.StatusViewerFor(object.GetObjectKind().GroupVersionKind().GroupKind()); err == nil {
				_, ready, err := statusViewer.Status(object, 0)
				if err != nil {
					return err
				}
				if !ready {
					return errors.New("object is not ready according to its status")
				}
			}

			var status status
			err = mapstructure.Decode(s, &status)
			if err != nil {
				return err
			}

			if status.ReadyReplicas != nil && *status.ReadyReplicas == 0 {
				return errors.New("object is not ready: no replicas are available")
			}

			if status.Phase != nil && (*status.Phase != "Active" &&
				*status.Phase != "Bound" &&
				*status.Phase != "Running" &&
				*status.Phase != "Ready" &&
				*status.Phase != "Online" &&
				*status.Phase != "Healthy") {
				return errors.New("object is not ready according to its phase")
			}

			if status.LoadBalancer != nil {
				// LoadBalancer status may be for an Ingress or a Service having type=LoadBalancer
				checkLoadBalancer := true

				if object.GetAPIVersion() == "v1" && object.GetKind() == "Service" {
					spec, ok := object.Object["spec"].(map[string]interface{})
					if !ok {
						return errors.New("invalid service object")
					}

					checkLoadBalancer = spec["type"] == "LoadBalancer"
				}

				if checkLoadBalancer && len(*status.LoadBalancer) == 0 {
					return errors.New("object is not ready: loadbalancer is not ready")
				}
			}
		}

		return nil
	}
}

func testAccCheckK8sManifestDestroy(s *terraform.State) error {
	c := testAccProvider.Meta().(*ProviderConfig).RuntimeClient

	ctx := context.Background()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "k8s_manifest" {
			continue
		}

		namespace, gv, kind, name, err := idParts(rs.Primary.ID)
		if err != nil {
			return err
		}

		groupVersion, err := k8sschema.ParseGroupVersion(gv)
		if err != nil {
			return err
		}

		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(groupVersion.WithKind(kind))
		object.SetNamespace(namespace)
		object.SetName(name)

		err = c.Delete(ctx, object)
		if err != nil && !k8sapierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
