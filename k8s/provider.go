package k8s

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mitchellh/go-homedir"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // to import all provider auths
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func Provider() *schema.Provider {
	p := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_HOST", ""),
				Description: "The hostname (in form of URI) of Kubernetes master.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_USER", ""),
				Description: "The username to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_PASSWORD", ""),
				Description: "The password to use for HTTP basic authentication when accessing the Kubernetes master endpoint.",
			},
			"insecure": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_INSECURE", false),
				Description: "Whether server should be accessed without verifying the TLS certificate.",
			},
			"client_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLIENT_CERT_DATA", ""),
				Description: "PEM-encoded client certificate for TLS authentication.",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLIENT_KEY_DATA", ""),
				Description: "PEM-encoded client certificate key for TLS authentication.",
			},
			"cluster_ca_certificate": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CLUSTER_CA_CERT_DATA", ""),
				Description: "PEM-encoded root certificates bundle for TLS authentication.",
			},
			"config_path": {
				Type:     schema.TypeString,
				Optional: true,
				DefaultFunc: schema.MultiEnvDefaultFunc(
					[]string{
						"KUBE_CONFIG",
						"KUBECONFIG",
					},
					"~/.kube/config"),
				Description: "Path to the kube config file, defaults to ~/.kube/config",
			},
			"config_context": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX", ""),
			},
			"config_context_auth_info": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX_AUTH_INFO", ""),
				Description: "",
			},
			"config_context_cluster": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_CTX_CLUSTER", ""),
				Description: "",
			},
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_TOKEN", ""),
				Description: "Token to authenticate an service account",
			},
			"load_config_file": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KUBE_LOAD_CONFIG_FILE", true),
				Description: "Load local kubeconfig.",
			},
			"exec": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_version": {
							Type:     schema.TypeString,
							Required: true,
						},
						"command": {
							Type:     schema.TypeString,
							Required: true,
						},
						"env": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"args": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
				Description: "",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"k8s_manifest": resourceK8sManifest(),
		},
	}

	p.ConfigureFunc = func(d *schema.ResourceData) (interface{}, error) {
		terraformVersion := p.TerraformVersion
		if terraformVersion == "" {
			// Terraform 0.12 introduced this field to the protocol
			// We can therefore assume that if it's missing it's 0.10 or 0.11
			terraformVersion = "0.11+compatible"
		}
		return providerConfigure(d, terraformVersion)
	}

	return p
}

type ProviderConfig struct {
	RuntimeClient client.Client
}

func providerConfigure(d *schema.ResourceData, terraformVersion string) (interface{}, error) {
	cfg := &restclient.Config{}
	var err error
	_, configSet := d.GetOk("host")
	if d.Get("load_config_file").(bool) {
		// Config file loading
		cfg, err = tryLoadingConfigFile(d)
		if err != nil {
			return nil, err
		}
	} else if !configSet {
		// Attempt to load in-cluster config
		cfg, err = restclient.InClusterConfig()
		if err != nil {
			// Fallback to standard config if we are not running inside a cluster
			if err == restclient.ErrNotInCluster {
				cfg = &restclient.Config{}
			} else {
				return nil, fmt.Errorf("Failed to configure: %s", err)
			}
		}
	}

	// Overriding with static configuration
	cfg.UserAgent = fmt.Sprintf("HashiCorp/1.0 Terraform/%s", terraformVersion)

	if v, ok := d.GetOk("host"); ok {
		cfg.Host = v.(string)
	}
	if v, ok := d.GetOk("username"); ok {
		cfg.Username = v.(string)
	}
	if v, ok := d.GetOk("password"); ok {
		cfg.Password = v.(string)
	}
	if v, ok := d.GetOk("insecure"); ok {
		cfg.Insecure = v.(bool)
	}
	if v, ok := d.GetOk("cluster_ca_certificate"); ok {
		cfg.CAData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("client_certificate"); ok {
		cfg.CertData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("client_key"); ok {
		cfg.KeyData = bytes.NewBufferString(v.(string)).Bytes()
	}
	if v, ok := d.GetOk("token"); ok {
		cfg.BearerToken = v.(string)
	}

	if v, ok := d.GetOk("exec"); ok {
		exec := &clientcmdapi.ExecConfig{}
		if spec, ok := v.([]interface{})[0].(map[string]interface{}); ok {
			exec.APIVersion = spec["api_version"].(string)
			exec.Command = spec["command"].(string)
			exec.Args = expandStringSlice(spec["args"].([]interface{}))
			for kk, vv := range spec["env"].(map[string]interface{}) {
				exec.Env = append(exec.Env, clientcmdapi.ExecEnvVar{Name: kk, Value: vv.(string)})
			}
		} else {
			return nil, fmt.Errorf("Failed to parse exec")
		}
		cfg.ExecProvider = exec
	}

	if logging.IsDebugOrHigher() {
		log.Printf("[DEBUG] Enabling HTTP requests/responses tracing")
		cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
			return logging.NewTransport("Kubernetes", rt)
		}
	}

	mapper, err := apiutil.NewDynamicRESTMapper(cfg, apiutil.WithLazyDiscovery)
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{
		Mapper: mapper,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to configure: %s", err)
	}

	return &ProviderConfig{
		RuntimeClient: c,
	}, nil
}

func tryLoadingConfigFile(d *schema.ResourceData) (*restclient.Config, error) {
	path, err := homedir.Expand(d.Get("config_path").(string))
	if err != nil {
		return nil, err
	}

	loader := &clientcmd.ClientConfigLoadingRules{
		ExplicitPath: path,
	}

	overrides := &clientcmd.ConfigOverrides{}
	ctxSuffix := "; default context"

	ctx, ctxOk := d.GetOk("config_context")
	authInfo, authInfoOk := d.GetOk("config_context_auth_info")
	cluster, clusterOk := d.GetOk("config_context_cluster")
	if ctxOk || authInfoOk || clusterOk {
		ctxSuffix = "; overriden context"
		if ctxOk {
			overrides.CurrentContext = ctx.(string)
			ctxSuffix += fmt.Sprintf("; config ctx: %s", overrides.CurrentContext)
			log.Printf("[DEBUG] Using custom current context: %q", overrides.CurrentContext)
		}

		overrides.Context = clientcmdapi.Context{}
		if authInfoOk {
			overrides.Context.AuthInfo = authInfo.(string)
			ctxSuffix += fmt.Sprintf("; auth_info: %s", overrides.Context.AuthInfo)
		}
		if clusterOk {
			overrides.Context.Cluster = cluster.(string)
			ctxSuffix += fmt.Sprintf("; cluster: %s", overrides.Context.Cluster)
		}
		log.Printf("[DEBUG] Using overidden context: %#v", overrides.Context)
	}

	cc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, overrides)
	cfg, err := cc.ClientConfig()
	if err != nil {
		if pathErr, ok := err.(*os.PathError); ok && os.IsNotExist(pathErr.Err) {
			log.Printf("[INFO] Unable to load config file as it doesn't exist at %q", path)
			return nil, nil
		}
		log.Printf("[WARN] Failed to load config (%s%s): %s", path, ctxSuffix, err)
	}

	log.Printf("[INFO] Successfully loaded config file (%s%s)", path, ctxSuffix)
	return cfg, nil
}
