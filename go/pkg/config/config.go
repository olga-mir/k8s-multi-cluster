package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Clusters []ClusterConfig
}

type GitHubConfig struct {
	User   string `mapstructure:"user"`
	Branch string `mapstructure:"branch"`
	Repo   string `mapstructure:"repo"`
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
	GitHub    GitHubConfig `mapstructure:"github"`
	KeyPath   string       `mapstructure:"keyPath"`
	Version   string       `mapstructure:"version"`
	Namespace string       `mapstructure:"namespace"`
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

	return &config, nil
}
