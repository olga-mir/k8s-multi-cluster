package capi

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	capiconfig "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ClusterAPI struct {
	log              logr.Logger
	clusterAuth      *k8sclient.CluserAuthInfo
	kubeconfigPath   string
	runtimeClient    runtimeclient.Client
	clusterctlClient capiclient.Client
}

func NewClusterAPI(log logr.Logger, clusterAuth *k8sclient.CluserAuthInfo, kubeconfigPath string) (*ClusterAPI, error) {
	runtimeScheme := runtime.NewScheme()
	clusterv1.AddToScheme(runtimeScheme)

	clusterctlConfigPath := utils.RepoRoot() + "/clusters/tmp-mgmt/clusterctl.yaml"

	clusterctlConfig, err := capiconfig.New(context.TODO(), clusterctlConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error creating clusterctl config: %w", err)
	}

	clusterctlClient, err := capiclient.New(context.TODO(), "", capiclient.InjectConfig(clusterctlConfig))
	if err != nil {
		return nil, fmt.Errorf("error creating clusterctl client: %w", err)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %s", err)
	}

	runtimeClient, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: runtimeScheme})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %s", err)
	}

	return &ClusterAPI{
		log:              log,
		clusterAuth:      clusterAuth,
		kubeconfigPath:   kubeconfigPath,
		runtimeClient:    runtimeClient,
		clusterctlClient: clusterctlClient,
	}, nil
}

func (c *ClusterAPI) InstallClusterAPI() error {
	// Create a clusterctl client
	// Get the current context name from the rest.Config
	contextName, err := utils.GetCurrentContextName(c.clusterAuth.Config, c.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error getting current context name: %w", err)
	}

	initOptions := capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: c.kubeconfigPath, Context: contextName},
		InfrastructureProviders: []string{"aws:v2.3.1"},
	}

	// Install Cluster API components on this cluster.
	if _, err := c.clusterctlClient.Init(context.TODO(), initOptions); err != nil {
		return fmt.Errorf("error initializing Cluster API: %w", err)
	}

	return nil
}

func (c *ClusterAPI) WaitForClusterFullyRunning(clusterName, namespace string) error {
	err := c.waitForClusterProvisioning(clusterName, namespace)
	if err != nil {
		return fmt.Errorf("error waiting for cluster provisioning: %w", err)
	}

	// After cluster is provisioned from Cluster API standpoint, we still need to wait for the CNI and Flux
	// being ready on the "workload" cluster, which will be permanent management cluster.
	/*
			% k get hrp -A
		NAMESPACE      NAME                        CLUSTER        READY   REASON   STATUS     REVISION
		cluster-mgmt   cilium-cluster-mgmt-9w44z   cluster-mgmt   True             deployed   1
		%
		% k get hcp -A
		NAMESPACE      NAME             READY   REASON
		cluster-mgmt   cilium           True
		cluster-mgmt   cilium-no-mesh   True
	*/

	caaphGVRs := []schema.GroupVersionResource{
		{Group: "addons.cluster.x-k8s.io", Version: "v1", Resource: "helmchartproxies"},
		{Group: "addons.cluster.x-k8s.io", Version: "v1", Resource: "helmreleaseproxies"},
	}
	namespaces := []string{}
	err = utils.WaitAllResourcesReady(*c.clusterAuth, namespaces, caaphGVRs)
	if err != nil {
		return fmt.Errorf("error waiting for CAAPH resources to be ready: %w", err)
	}

	return nil
}

func (c *ClusterAPI) waitForClusterProvisioning(clusterName, namespace string) error {
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

			// Check if the cluster is provisioned
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

func (c *ClusterAPI) PivotCluster(permClusterAuth *k8sclient.CluserAuthInfo) error {

	// TODO - context name can be part of ClusterAuth or `ClusterAPI` receiver
	contextName, err := utils.GetCurrentContextName(c.clusterAuth.Config, c.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error getting current context name: %w", err)
	}

	moveOptions := capiclient.MoveOptions{
		FromKubeconfig: capiclient.Kubeconfig{Path: c.kubeconfigPath, Context: contextName},
		ToKubeconfig:   capiclient.Kubeconfig{Path: c.kubeconfigPath}, //, Context: "cluster-mgmt-admin@cluster-mgmt"}, // TODO - context name
	}

	// Perform the move
	if err := c.clusterctlClient.Move(context.TODO(), moveOptions); err != nil {
		c.log.Error(err, "Failed to pivot Cluster API components")
		return err
	}

	c.log.Info("Successfully pivoted Cluster API components")
	return nil
}
