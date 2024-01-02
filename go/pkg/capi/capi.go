package capi

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

type ClusterAPI struct {
	log            logr.Logger
	clusterAuth    *k8sclient.CluserAuthInfo
	kubeconfigPath string
}

func NewClusterAPI(log logr.Logger, clusterAuth *k8sclient.CluserAuthInfo, kubeconfigPath string) *ClusterAPI {
	return &ClusterAPI{
		log:            log,
		clusterAuth:    clusterAuth,
		kubeconfigPath: kubeconfigPath,
	}
}

func (c *ClusterAPI) InstallClusterAPI() error {
	// TODO - hardcoded while I am figuring out why
	// `clusterctl init --config clusters/tmp-mgmt/clusterctl.yaml` doesn't work
	capiVersion := "v1.6.0"
	capaVersion := "v2.3.1"

	// Correct providers based on the CAPI version
	coreProvider := fmt.Sprintf("cluster-api:%s", capiVersion)
	bootstrapProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	controlPlaneProvider := fmt.Sprintf("kubeadm:%s", capiVersion)
	infraProvider := fmt.Sprintf("aws:%s", capaVersion)

	// Create a clusterctl client
	// Get the current context name from the rest.Config
	contextName, err := utils.GetCurrentContextName(c.clusterAuth.Config, c.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error getting current context name: %w", err)
	}

	// Initialize clusterctl client with the existing kubeconfig and context
	clusterctlClient, err := client.New(context.TODO(), "")
	if err != nil {
		return fmt.Errorf("error creating clusterctl client: %w", err)
	}

	initOptions := client.InitOptions{
		Kubeconfig:              client.Kubeconfig{Path: c.kubeconfigPath, Context: contextName},
		CoreProvider:            coreProvider,
		BootstrapProviders:      []string{bootstrapProvider},
		ControlPlaneProviders:   []string{controlPlaneProvider},
		InfrastructureProviders: []string{infraProvider},
	}

	// Install Cluster API components on this cluster.
	if _, err := clusterctlClient.Init(context.TODO(), initOptions); err != nil {
		return fmt.Errorf("error initializing Cluster API: %w", err)
	}

	return nil
}
