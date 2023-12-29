package capi

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

func InitClusterAPI(k8sClient *kubernetes.Clientset) error {
	capiVersion := os.Getenv("CAPI_VERSION")
	if capiVersion == "" {
		return fmt.Errorf("CAPI_VERSION environment variable is not set")
	}

	coreProvider := fmt.Sprintf("cluster-api:%s", capiVersion)
	bootstrapProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	controlPlaneProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	infraProvider := fmt.Sprintf("aws:%s", capiVersion)

	// Extract the REST config from the Kubernetes client
	restConfig := k8sClient.RESTConfig()

	// Create a clusterctl client
	c, err := client.New("")
	if err != nil {
		return err
	}

	// Define the InitOptions
	initOptions := client.InitOptions{
		Kubeconfig:              client.Kubeconfig{Path: restConfig.Host, Context: restConfig.Context},
		CoreProvider:            coreProvider,
		BootstrapProviders:      []string{bootstrapProvider},
		ControlPlaneProviders:   []string{controlPlaneProvider},
		InfrastructureProviders: []string{infraProvider},
	}

	// Perform the init operation
	if _, err := c.Init(initOptions); err != nil {
		return err
	}

	return nil
}
