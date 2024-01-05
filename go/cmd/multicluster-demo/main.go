package main

import (
	"fmt"
	"os"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/builder"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/runner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var requiredVars = []string{
	"K8S_MULTI_KUBECONFIG",
	"AWS_B64ENCODED_CREDENTIALS",
	"FLUXCD_KEY_PATH",
}

var logger = log.Log

var cfgFile string

// Following cmd variables could be defined inside main function, but setting them as global variables have some advantages:
// - Organises command setup separately from the main application logic.
// - Allows for modular command definitions, where each command's setup is contained within its own init function.
// - Useful in larger applications where commands might be spread across multiple files.
// This approach is endorsed by Cobra creators as can be seen in the user guide: https://github.com/spf13/cobra/blob/main/site/content/user_guide.md
// TODO - this is however can have implications on running tests and logger usage, e.g. https://github.com/spf13/cobra/issues/1599
var rootCmd = &cobra.Command{
	Use:   "multicluster-demo",
	Short: "Multi Cluster Demo app build a multi cluster setup in a cloud provider of choice by using Cluster API or CrossPlane and runs scenarios such as immutable cluster upgrade with no downtime or cluster failover",
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a multi cluster setup in a cloud provider of choice by using Cluster API or CrossPlane according to specification provided in config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return err
		}
		return builder.Deploy(logger, cfg)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall all project resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return err
		}
		return builder.Uninstall(logger, cfg)
	},
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run scenarios",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return err
		}
		return runner.RunScenarios(cfg) // TODO - logger dep injection
	},
}

func main() {
	// Validate required environment variables
	if err := validateEnvVars(requiredVars); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Initialize the logger
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.myapp.yaml)")
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(runCmd)
}

func initConfig() {

	// Load the configuration file if specified
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".myapp")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// validateEnvVars checks if all required environment variables are set.
// It returns an error if any variable is missing.
func validateEnvVars(vars []string) error {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			return fmt.Errorf("required environment variable not set: %s", v)
		}
	}
	return nil
}
