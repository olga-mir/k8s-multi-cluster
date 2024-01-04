package capi

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ClusterAPI struct {
	log            logr.Logger
	clusterAuth    *k8sclient.CluserAuthInfo
	kubeconfigPath string
	runtimeClient  runtimeclient.Client
}

func NewClusterAPI(log logr.Logger, clusterAuth *k8sclient.CluserAuthInfo, kubeconfigPath string) (*ClusterAPI, error) {
	runtimeScheme := runtime.NewScheme()
	clusterv1.AddToScheme(runtimeScheme)

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %s", err)
	}

	runtimeClient, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: runtimeScheme})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %s", err)
	}

	return &ClusterAPI{
		log:            log,
		clusterAuth:    clusterAuth,
		kubeconfigPath: kubeconfigPath,
		runtimeClient:  runtimeClient,
	}, nil
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

func (c *ClusterAPI) WaitForClusterProvisioning(clusterName, namespace string) error {
	timeout := 15 * time.Minute
	c.log.Info("Waiting for cluster to be provisioned", "cluster", clusterName, "namespace", namespace)

	// Define the timeout for the wait operation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for cluster '%s' to be provisioned", clusterName)
		default:
			cluster := &clusterv1.Cluster{}
			err := c.runtimeClient.Get(ctx, runtimeclient.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
			if err != nil {
				return err
			}

			// Check if the cluster is provisioned (example condition)
			if cluster.Status.Phase == "Provisioned" {
				return nil
			}

			// Sleep before the next check
			time.Sleep(15 * time.Second)
		}
	}
}

func (c *ClusterAPI) WaitForAllClustersProvisioning() {
	// TODO

	/*
		clusterNamespaces, err := utils.ListAllNamespacesWithPrefix(kubeClients.TempManagementCluster.Clientset, "cluster")
		if err != nil {
			return fmt.Errorf("error listing namespaces with prefix 'cluster': %v", err)
		}
		log.Info("Cluster namespaces: ", "namespaces", clusterNamespaces)
	*/
}

func (c *ClusterAPI) WaitForClusterDeletion(clusterName, namespace string) error {
	timeout := 15 * time.Minute
	c.log.Info("Waiting for cluster to be deleted", "cluster", clusterName, "namespace", namespace)

	// Define the timeout for the wait operation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for cluster '%s' to be deleted", clusterName)
		default:
			cluster := &clusterv1.Cluster{}
			err := c.runtimeClient.Get(ctx, runtimeclient.ObjectKey{Name: clusterName, Namespace: namespace}, cluster)
			if err != nil {
				return nil
			}

			// Sleep before the next check
			time.Sleep(15 * time.Second)
		}
	}
}
