package capi

import (
	"context"
	"fmt"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

func InitClusterAPI(restConfig *rest.Config, kubeconfigPath string) error {
	clusterctlConfigPath := utils.RepoRoot() + "/clusters/tmp-mgmt/clusterctl.yaml"
	myConfig, err := config.New(context.TODO(), clusterctlConfigPath)
	if err != nil {
		panic(err)
	}

	// Create the clusterctl client with the custom configuration
	c, err := client.New(context.TODO(), clusterctlConfigPath, client.InjectConfig(myConfig))
	if err != nil {
		panic(err)
	}

	// Create a clusterctl client
	// Get the current context name from the rest.Config
	contextName, err := utils.GetCurrentContextName(restConfig, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error getting current context name: %w", err)
	}

	initOptions := client.InitOptions{
		Kubeconfig: client.Kubeconfig{Path: kubeconfigPath, Context: contextName},
	}

	// Install Cluster API components on this cluster.
	if _, err := c.Init(context.TODO(), initOptions); err != nil {
		return fmt.Errorf("error initializing Cluster API: %w", err)
	}

	return nil
}
