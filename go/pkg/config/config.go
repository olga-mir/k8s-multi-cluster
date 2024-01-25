package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Clusters       []ClusterConfig
	Github         GithubConfig `mapstructure:"github"`
	KubeconfigPath string       `mapstructure:"kubeconfigPath"`
}

// TODO - URL and GithubKnownHosts are not expected to be provided by the user
// Is there a pattern to manage a data type where fields exposed to the user
// while others are used by other packages in the app, but preset with calculated
// const or default values
type GithubConfig struct {
	User             string `mapstructure:"user"`
	Branch           string `mapstructure:"branch"`
	RepoName         string `mapstructure:"repoName"`
	URL              string
	GithubKnownHosts string
}

type ClusterConfig struct {
	Name              string     `mapstructure:"name"`
	Provider          string     `mapstructure:"provider"`
	KubernetesVersion string     `mapstructure:"kubernetesVersion"`
	PodCIDR           string     `mapstructure:"podCIDR"`
	ManagementCluster string     `mapstructure:"managementCluster"`
	Flux              FluxConfig `mapstructure:"flux"`
	CNI               CNIConfig  `mapstructure:"cni"`
	AWS               AWSConfig  `mapstructure:"aws"`
}

type FluxConfig struct {
	KeyPath   string `mapstructure:"keyPath"`
	Version   string `mapstructure:"version"`
	Namespace string `mapstructure:"namespace"`
}

type CNIConfig struct {
	Type   string `mapstructure:"type"`
	Config string `mapstructure:"config"`
}

type AWSConfig struct {
	SSHKeyName string `mapstructure:"sshKeyName"`
	Region     string `mapstructure:"region"`
}

func LoadConfig(path string) (*Config, error) {
	var config Config

	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	setDefaults(&config)

	return &config, nil
}

func setDefaults(config *Config) error {
	// has to be accessed by index, so that paths are replaced by passing
	// to the relevant fields
	for i := range config.Clusters {
		if config.Clusters[i].Flux.Namespace == "" {
			config.Clusters[i].Flux.Namespace = FluxNamespace
		}

		err := ensureSafePath(&config.Clusters[i].Flux.KeyPath)
		if err != nil {
			return err
		}
	}
	config.Clusters = append(config.Clusters, kindClusterConfig(DefaultKindClusterName))

	// if kubeconfigPath is not set, use K8S_MULTI_KUBECONFIG environment variable.
	// kubeconfig path MUST be provided by the user explicitely in one of these two ways
	// because the app will be modifying this file and user must be aware of this fact
	if config.KubeconfigPath == "" {
		envKubeconfigPath := os.Getenv("K8S_MULTI_KUBECONFIG")
		if envKubeconfigPath != "" {
			config.KubeconfigPath = envKubeconfigPath
		} else {
			return fmt.Errorf("K8S_MULTI_KUBECONFIG environment variable is not set")
		}
	}

	err := ensureSafePath(&config.KubeconfigPath)
	if err != nil {
		return err
	}
	// Validate Github config
	if config.Github.User == "" || config.Github.RepoName == "" {
		return fmt.Errorf("github user and repo are not set")
	}

	if config.Github.Branch == "" {
		config.Github.Branch = "main"
	}

	config.Github.URL = "ssh://git@github.com/" + config.Github.User + "/" + config.Github.RepoName
	config.Github.GithubKnownHosts = GithubKnownHosts

	return nil
}

func ensureSafePath(pathPtr *string) error {

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	unsafePathPrefixes := []string{"$HOME", "~"}
	for _, p := range unsafePathPrefixes {
		if strings.HasPrefix(*pathPtr, p) {
			safePath := strings.Replace(*pathPtr, p, home, 1)
			*pathPtr = safePath
			return nil
		}
	}
	return nil
}

func kindClusterConfig(clusterName string) ClusterConfig {
	// TODO - re-think implicit kind config. We shouldn't require flux key here.
	fluxcdKey := os.Getenv("FLUXCD_KEY_PATH")
	if fluxcdKey == "" {
		// TODO - for now all clusters will share the same key.
		// TODO - change function so that we don't have to panic here
		panic("FLUXCD_KEY_PATH environment variable is not set")
	}
	return ClusterConfig{
		Name:              clusterName,
		Provider:          "kind",
		ManagementCluster: "",
		Flux: FluxConfig{
			KeyPath:   fluxcdKey,
			Version:   KindFluxVersion,
			Namespace: FluxNamespace,
		},
	}
}
