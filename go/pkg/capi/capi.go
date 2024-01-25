package capi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	capiconfig "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

type ClusterAPI struct {
	log              logr.Logger
	clusterAuth      *k8sclient.ClusterAuthInfo // TODO - why is this * while in other places it is not? (e.g. flux.go)
	runtimeClient    runtimeclient.Client
	clusterctlClient capiclient.Client
	kubeconfigPath   string
}

// NewClusterAPI creates a new instance of the ClusterAPI struct. This function initializes
// the ClusterAPI with the provided logger, authentication information, kubeconfig path,
// and context name. It returns a pointer to the newly created ClusterAPI instance and
// an error if any issues occur during the initialization. Context name has to be provided
// because it is not part of the authentication information stored in the clusterAuth variable
// but clusterApi client works with kubeconfig and context name, rather than REST config or clientset
// Context name is an arbitrary name given to a context inside kubeconfig file. At this stage
// of CAPI cluster the context for a cluster may not even exist yet in the kubeconfig
func NewClusterAPI(log logr.Logger, clusterAuth *k8sclient.ClusterAuthInfo, kubeconfigPath string) (*ClusterAPI, error) {
	runtimeScheme := runtime.NewScheme()
	clusterv1.AddToScheme(runtimeScheme)

	clusterctlConfigPath := utils.RepoRoot() + "/clusters/" + clusterAuth.ClusterName + "/clusterctl.yaml"

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

	// Get the current context name from the rest.Config
	log.Info("Creating Cluster API clients for kube context", "name", clusterAuth.ContextName)
	if err != nil {
		return nil, fmt.Errorf("error getting current context name: %w", err)
	}

	return &ClusterAPI{
		log:              log,
		clusterAuth:      clusterAuth,
		runtimeClient:    runtimeClient,
		clusterctlClient: clusterctlClient,
		kubeconfigPath:   kubeconfigPath,
	}, nil
}

func (c *ClusterAPI) InstallClusterAPI() error {
	// Create a clusterctl client

	initOptions := capiclient.InitOptions{
		Kubeconfig:              capiclient.Kubeconfig{Path: c.kubeconfigPath, Context: c.clusterAuth.ContextName},
		InfrastructureProviders: []string{"aws:v2.3.1"}, // TODO - there is a bug in CAPI init file. infra provider has to be specified explicitely
	}

	// Install Cluster API components on this cluster.
	if _, err := c.clusterctlClient.Init(context.TODO(), initOptions); err != nil {
		return fmt.Errorf("error initializing Cluster API: %w", err)
	}

	return nil
}

func (c *ClusterAPI) WaitForWorkloadClusterFullyRunning(name string) error {

	// TODO - this name extraction happens twice in the cluster bootstrap.
	// maybe a workload cluster should maintain a list of its managed clusters
	workloadClusterName, _, err := utils.GetCAPIClusterNameAndContext(utils.ClusterNameData{Name: name})
	if err != nil {
		return fmt.Errorf("error getting cluster name and context: %v", err)
	}

	c.log.Info("Wating for CAPI cluster to be provisioned and all system components healthy", "cluster", workloadClusterName)

	err = c.waitForCAPIClusterStateProvisioned(workloadClusterName, workloadClusterName)
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

	// TODO - these and other CRDS can be in defaults
	caaphGVRs := []schema.GroupVersionResource{
		{Group: "addons.cluster.x-k8s.io", Version: "v1", Resource: "helmchartproxies"},
		{Group: "addons.cluster.x-k8s.io", Version: "v1", Resource: "helmreleaseproxies"},
	}

	c.log.Info("Wait for CAAPH resources to be Ready")

	namespaces, err := utils.ListAllNamespacesWithPrefix(c.clusterAuth.Clientset, "cluster-")
	if err != nil {
		return fmt.Errorf("error listing all namespaces with prefix 'cluster-': %w", err)
	}

	err = utils.WaitAllResourcesReady(*c.clusterAuth, namespaces, caaphGVRs) // TODO - is this blocking?
	if err != nil {
		return fmt.Errorf("error waiting for CAAPH resources to be ready: %w", err)
	}
	c.log.Info("All CAAPH resources are ready") // TODO - why this line is never printed?

	return nil
}

// waitForCAPIClusterStateProvisioned blocks until the specified cluster reaches the 'Provisioned' state.
// This function specifically checks the status of the Cluster API custom resource named 'clusterName'
// within the given 'namespace'. It's important to note that reaching the 'Provisioned' state does not
// necessarily mean the cluster is fully operational and ready for use. Key components, such as the CNI,
// might still be in the process of becoming ready. Therefore, additional checks should be performed
// after this function returns to ensure that all critical components of the cluster are functional.
func (c *ClusterAPI) waitForCAPIClusterStateProvisioned(clusterName, namespace string) error {
	timeout := 15 * time.Minute

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

			if cluster.Status.Phase == "Provisioned" {
				return nil
			}

			time.Sleep(30 * time.Second)
		}
	}
}

func (c *ClusterAPI) WaitForAllClustersProvisioning() error {
	namespaces, err := utils.ListAllNamespacesWithPrefix(c.clusterAuth.Clientset, "cluster-")
	if err != nil {
		return fmt.Errorf("failed to list all namespaces: %w", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(namespaces))

	for _, ns := range namespaces {
		wg.Add(1)
		go func(namespace string) {
			defer wg.Done()
			// TODO - need to rework the cluster/namespace relashionship later.
			//  One namespace should allow more than 1 cluster
			if err := c.waitForCAPIClusterStateProvisioned(namespace, namespace); err != nil {
				errors <- fmt.Errorf("error in namespace %s: %w", namespace, err)
			}
		}(ns)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			return err // Return on the first error encountered
		}
	}
	return nil
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

// GetClusterAuthInfo returns the clientset and rest.Config for the workload cluster.
// It also updates the kubeconfig with the worklaod cluster config. (TODO - this feels like a side effect, is there a better way to do this?)
func (c *ClusterAPI) GetClusterAuthInfoForWorkloadCluster(authInfo *k8sclient.ClusterAuthInfo, name string) error {
	// translate between this project cluster name (which is more like a role) to the name how CAPI clusters are named
	workloadClusterName, workloadClusterCtxName, err := utils.GetCAPIClusterNameAndContext(utils.ClusterNameData{Name: name})
	if err != nil {
		return fmt.Errorf("error getting cluster name and context: %v", err)
	}

	getKubeconfigOptions := capiclient.GetKubeconfigOptions{
		Namespace:           workloadClusterName,
		WorkloadClusterName: workloadClusterName,
		Kubeconfig: capiclient.Kubeconfig{
			Path: c.kubeconfigPath,
			// This is the context name of "this" cluster (the one which is currently acting as a management cluster for the given workload cluster)
			Context: c.clusterAuth.ContextName,
		},
	}
	c.log.Info("GetClusterAuthInfo for workload cluster", "name", workloadClusterName, "options", getKubeconfigOptions)

	// Get the kubeconfig for the workload cluster
	workloadKubeconfig, err := c.clusterctlClient.GetKubeconfig(context.TODO(), getKubeconfigOptions)
	if err != nil {
		c.log.Error(err, "Failed to get kubeconfig")
		return err
	}

	// Create a rest.Config object from the kubeconfig
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(workloadKubeconfig))
	if err != nil {
		c.log.Error(err, "Failed to create rest.Config from kubeconfig")
		return err
	}

	// Create a Clientset from the rest.Config
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		c.log.Error(err, "Failed to create clientset from rest.Config")
		return err
	}

	authInfo.Clientset = clientset
	authInfo.Config = restConfig
	authInfo.ContextName = workloadClusterCtxName
	authInfo.ClusterName = workloadClusterName

	err = utils.MergeKubeconfigs(workloadKubeconfig, c.kubeconfigPath)
	if err != nil {
		c.log.Error(err, "Error merging kubeconfig files")
		return err
	}
	return nil
}

func (c *ClusterAPI) PivotCluster(permClusterAuth *k8sclient.ClusterAuthInfo) error {
	c.log.Info("Pivoting management cluster", "fromContextName", c.clusterAuth.ContextName, "toContextName", permClusterAuth.ContextName)
	moveOptions := capiclient.MoveOptions{
		FromKubeconfig: capiclient.Kubeconfig{Path: c.kubeconfigPath, Context: c.clusterAuth.ContextName},
		ToKubeconfig:   capiclient.Kubeconfig{Path: c.kubeconfigPath, Context: permClusterAuth.ContextName},
		Namespace:      "cluster-mgmt", // TODO - hardcoded name
	}

	// Perform the move
	if err := c.clusterctlClient.Move(context.TODO(), moveOptions); err != nil {
		c.log.Error(err, "Failed to pivot Cluster API components")
		return err
	}

	ClusterGVR := schema.GroupVersionResource{
		Group:    "cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "clusters",
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for custom resource to be available in the target cluster")
		case <-ticker.C:
			// Check if the custom resource exists in the target cluster
			exists, err := utils.ResourcesExist(permClusterAuth.Config, permClusterAuth.ClusterName, permClusterAuth.ClusterName, ClusterGVR)
			if err != nil {
				c.log.Error(err, "Error checking custom resource in target cluster")
				return err
			}
			if exists {
				c.log.Info("Successfully pivoted Cluster API components and custom resource is available in the target cluster")
				return nil
			}
		}
	}
}
