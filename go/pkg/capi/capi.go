package capi

import (
	"context"
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

func InitClusterAPI(config *rest.Config) error {
	capiVersion := os.Getenv("CAPI_VERSION")
	if capiVersion == "" {
		return fmt.Errorf("CAPI_VERSION environment variable is not set")
	}
	j
	coreProvider := fmt.Sprintf("cluster-api:%s", capiVersion)
	bootstrapProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	controlPlaneProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	infraProvider := fmt.Sprintf("aws:%s", capiVersion)

	// Create a clusterctl client
	c, err := client.New(context.TODO(), "path/to/kubeconfig")
	if err != nil {
		return fmt.Errorf("error creating clusterctl client: %w", err)
	}

	// Define the InitOption
	initOptions := client.InitOptions{
		Kubeconfig:              client.Kubeconfig{},
		CoreProvider:            coreProvider,
		BootstrapProviders:      []string{bootstrapProvider},
		ControlPlaneProviders:   []string{controlPlaneProvider},
		InfrastructureProviders: []string{infraProvider},
	}

	// Perform the init operation
	if _, err := c.Init(context.TODO(), initOptions); err != nil {
		return err
	}

	return nil
}
