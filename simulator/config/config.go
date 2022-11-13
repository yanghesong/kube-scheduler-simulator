package config

import (
	"golang.org/x/xerrors"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	v1beta2config "k8s.io/kube-scheduler/config/v1beta2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config/scheme"
	"net/url"
	"os"
	"strconv"

	"sigs.k8s.io/kube-scheduler-simulator/simulator/scheduler/config"
)

// configYaml represents the value from the config file.
var configYaml *ConfigYaml = &ConfigYaml{}

// YamlFile is the config file path.
// TODO: Config this file path by cli in main function.
const YamlFile = "./config.yml"

// Config is configuration for simulator.
type Config struct {
	Port                  int
	KubeAPIServerURL      string
	EtcdURL               string
	CorsAllowedOriginList []string
	// ExternalImportEnabled indicates whether the simulator will import resources from an existing cluster or not.
	ExternalImportEnabled bool
	// ExternalKubeClientCfg is KubeConfig to get resources from external cluster.
	// This field is non-empty only when ExternalImportEnabled == true.
	ExternalKubeClientCfg *rest.Config
	InitialSchedulerCfg   *v1beta2config.KubeSchedulerConfiguration
	// ExternalSchedulerEnabled indicates whether an external scheduler is enabled.
	ExternalSchedulerEnabled bool
}

// ConfigYaml is the Go representation of a module configuration in the yaml
// config file.
type ConfigYaml struct {
	Port                     int      `yaml:"Port"`
	EtcdURL                  string   `yaml:"EtcdURL"`
	CorsAllowedOriginList    []string `yaml:"CorsAllowedOriginList"`
	KubeConfig               string   `yaml:"KubeConfig"`
	KubeApiHost              string   `yaml:"KubeApiHost"`
	KubeApiPort              int      `yaml:"KubeApiPort"`
	KubeSchedulerConfigPath  string   `yaml:"KubeSchedulerConfigPath"`
	ExternalImportEnabled    bool     `yaml:"ExternalImportEnabled"`
	ExternalSchedulerEnabled bool     `yaml:"ExternalSchedulerEnabled"`
}

// NewConfig gets some settings from environment variables.
func NewConfig() (*Config, error) {
	readConfigYaml()

	port, err := getPort()
	if err != nil {
		return nil, xerrors.Errorf("get port: %w", err)
	}

	etcdurl, err := getEtcdURL()
	if err != nil {
		return nil, xerrors.Errorf("get etcd URL: %w", err)
	}

	corsAllowedOriginList, err := getCorsAllowedOriginList()
	if err != nil {
		return nil, xerrors.Errorf("get frontend URL: %w", err)
	}

	apiurl := getKubeAPIServerURL()

	externalimportenabled := getExternalImportEnabled()
	externalKubeClientCfg := &rest.Config{}
	if externalimportenabled {
		externalKubeClientCfg, err = GetKubeClientConfig()
		if err != nil {
			return nil, xerrors.Errorf("get kube clientconfig: %w", err)
		}
	}

	initialschedulerCfg, err := getSchedulerCfg()
	if err != nil {
		return nil, xerrors.Errorf("get SchedulerCfg: %w", err)
	}

	externalSchedEnabled, err := getExternalSchedulerEnabled()
	if err != nil {
		return nil, xerrors.Errorf("get externalSchedulerEnabled: %w", err)
	}

	return &Config{
		Port:                     port,
		KubeAPIServerURL:         apiurl,
		EtcdURL:                  etcdurl,
		CorsAllowedOriginList:    corsAllowedOriginList,
		InitialSchedulerCfg:      initialschedulerCfg,
		ExternalImportEnabled:    externalimportenabled,
		ExternalKubeClientCfg:    externalKubeClientCfg,
		ExternalSchedulerEnabled: externalSchedEnabled,
	}, nil
}

// ReadConfigYaml read the yaml file and set configYaml
func readConfigYaml() {
	var configByte []byte
	var err error

	configByte, err = ioutil.ReadFile(YamlFile)
	if err != nil {
		//level.Error(logger).Log("msg", "Error reading config file", "error", err)
		return
	}

	if err = yaml.Unmarshal(configByte, configYaml); err != nil {
		return
	}
}

// getPort gets Port from the environment variable named PORT.
func getPort() (int, error) {
	port := configYaml.Port

	return port, nil
}

func getKubeAPIServerURL() string {
	port := configYaml.KubeApiPort

	host := configYaml.KubeApiHost
	if host == "" {
		host = "127.0.0.1"
	}
	return host + ":" + strconv.Itoa(port)
}

func getExternalSchedulerEnabled() (bool, error) {
	isExternalSchedulerEnabled := configYaml.ExternalSchedulerEnabled

	return isExternalSchedulerEnabled, nil
}

func getEtcdURL() (string, error) {
	etcdURL := configYaml.EtcdURL

	return etcdURL, nil
}

// getCorsAllowedOriginList fetches CorsAllowedOriginList from the env named CORS_ALLOWED_ORIGIN_LIST.
// This allowed list is applied to kube-apiserver and the simulator server.
//
// Let's say CORS_ALLOWED_ORIGIN_LIST="http://localhost:3000, http://localhost:3001, http://localhost:3002" are given.
// Then, getCorsAllowedOriginList returns []string{"http://localhost:3000", "http://localhost:3001", "http://localhost:3002"}
func getCorsAllowedOriginList() ([]string, error) {
	corsAllowedOriginList := configYaml.CorsAllowedOriginList

	if err := validateURLs(corsAllowedOriginList); err != nil {
		return nil, xerrors.Errorf("validate origins in CORS_ALLOWED_ORIGIN_LIST: %w", err)
	}

	return corsAllowedOriginList, nil
}

// validateURLs checks if all URLs in slice is valid or not.
func validateURLs(urls []string) error {
	for _, u := range urls {
		_, err := url.ParseRequestURI(u)
		if err != nil {
			return xerrors.Errorf("parse request uri: %w", err)
		}
	}
	return nil
}

// getSchedulerCfg reads KUBE_SCHEDULER_CONFIG_PATH which means initial kube-scheduler configuration
// and converts it into *v1beta2config.KubeSchedulerConfiguration.
// KUBE_SCHEDULER_CONFIG_PATH is not required.
// If KUBE_SCHEDULER_CONFIG_PATH is not set, the default configuration of kube-scheduler will be used.
func getSchedulerCfg() (*v1beta2config.KubeSchedulerConfiguration, error) {
	kubeSchedulerConfigPath := configYaml.KubeSchedulerConfigPath
	if kubeSchedulerConfigPath == "" {
		dsc, err := config.DefaultSchedulerConfig()
		if err != nil {
			return nil, xerrors.Errorf("create default scheduler config: %w", err)
		}
		return dsc, nil
	}

	data, err := os.ReadFile(kubeSchedulerConfigPath)
	if err != nil {
		return nil, xerrors.Errorf("read scheduler config file: %w", err)
	}

	sc, err := decodeSchedulerCfg(data)
	if err != nil {
		return nil, xerrors.Errorf("decode scheduler config file: %w", err)
	}

	return sc, nil
}

// getExternalImportEnabled reads EXTERNAL_IMPORT_ENABLED and convert it to bool.
// This function will return `true` if `EXTERNAL_IMPORT_ENABLED` is "1".
func getExternalImportEnabled() bool {
	isExternalImportEnabled := configYaml.ExternalImportEnabled
	return isExternalImportEnabled == true
}

func decodeSchedulerCfg(buf []byte) (*v1beta2config.KubeSchedulerConfiguration, error) {
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(buf, nil, nil)
	if err != nil {
		return nil, xerrors.Errorf("load an k8s object from buffer: %w", err)
	}

	sc, ok := obj.(*v1beta2config.KubeSchedulerConfiguration)
	if !ok {
		return nil, xerrors.Errorf("convert to *v1beta2config.KubeSchedulerConfiguration, but got unexpected type: %T", obj)
	}

	if err = sc.DecodeNestedObjects(decoder); err != nil {
		return nil, xerrors.Errorf("decode nested plugin args: %w", err)
	}
	return sc, nil
}

func GetKubeClientConfig() (*rest.Config, error) {
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, xerrors.Errorf("get client config: %w", err)
	}
	return config, nil
}
