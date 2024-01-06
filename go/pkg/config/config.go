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
	for i := range config.Clusters {
		if config.Clusters[i].Flux.Namespace == "" {
			config.Clusters[i].Flux.Namespace = FluxNamespace
		}
	}

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

	unsafePathPrefixes := []string{"$HOME", "~"}
	for _, p := range unsafePathPrefixes {
		if strings.HasPrefix(config.KubeconfigPath, p) {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			config.KubeconfigPath = strings.Replace(config.KubeconfigPath, p, home, 1)
			break
		} else {
			continue
		}
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
