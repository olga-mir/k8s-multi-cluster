package builder

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/kind"
)

// KubernetesClients represents a collection of Kubernetes clients for different clusters.
// ClusterClient contains REST Config and clientset. Clientset can't be easily used with
// custom resources and clients used by Cluster API and FluxCD
// TODO - review this sructure: maybe using only REST Config and building clientset dynamically when needed.
type KubernetesClients struct {
	TempManagementCluster *k8sclient.ClusterClient            // Temporary management cluster (kind)
	PermManagementCluster *k8sclient.ClusterClient            // Permanent management cluster
	WorkloadClusters      map[string]*k8sclient.ClusterClient // Map of workload clusters
}

func BuildClusters(log logr.Logger, cfg *config.Config) error {

	const kindContext = "kind-tmp-mgmt" // TODO

	kubeconfigPath := os.Getenv("K8S_MULTI_KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("K8S_MULTI_KUBECONFIG environment variable is not set")
	}

	// Create a kind cluster and get its kubeconfig
	log.Info("Create `kind` cluster")
	err := kind.CreateCluster(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating kind cluster: %v", err)
	}

	kindClusterConfig := config.KindClusterConfig("tmp-mgmt")

	kindConfig, err := k8sclient.GetKubernetesClient(kubeconfigPath, kindContext)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients := &KubernetesClients{
		TempManagementCluster: kindConfig,
		PermManagementCluster: nil,                                       // Initialize to nil to indicate that the permanent cluster has not been created yet.
		WorkloadClusters:      make(map[string]*k8sclient.ClusterClient), // Initialize the map to an empty map
	}

	// Install Cluster API on the kind cluster. kind is a temporary "CAPI management cluster" which will be used to provision
	// a cluster in the cloud which will be used as a permanent "CAPI management cluster" for the workload clusters.
	log.Info("Installing Cluster API on `kind` cluster")
	if err := capi.InitClusterAPI(kubeClients.TempManagementCluster.Config, kubeconfigPath); err != nil {
		return fmt.Errorf("error installing Cluster API: %w", err)
	}

	// Install FluxCD on the kind cluster
	log.Info("Installing FluxCD on `kind` cluster")
	fluxInstaller := fluxcd.NewFluxCDInstaller(log, kindClusterConfig.Flux, kubeClients.TempManagementCluster.Config, kubeClients.TempManagementCluster.Clientset)
	if err := fluxInstaller.InstallFluxCD(); err != nil {
		return fmt.Errorf("error installing FluxCD: %v", err)
	}

	// Pivot to the permanent management cluster
	// if err := capi.PivotCluster("path/to/temp/kubeconfig", "path/to/permanent/kubeconfig"); err != nil {
	//	log.Fatalf("Error pivoting to permanent cluster: %v", err)
	//}
	return nil
}
