package deployer

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
// CluserAuthInfo contains REST Config and clientset. Clientset can't be easily used with
// custom resources and clients used by Cluster API and FluxCD
// TODO - review this sructure: maybe using only REST Config and building clientset dynamically when needed.
type KubernetesClients struct {
	TempManagementCluster *k8sclient.CluserAuthInfo            // Temporary management cluster (kind)
	PermManagementCluster *k8sclient.CluserAuthInfo            // Permanent management cluster
	WorkloadClusters      map[string]*k8sclient.CluserAuthInfo // Map of workload clusters
}

func Deploy(log logr.Logger, cfg *config.Config) error {
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

	// TODO - naming. kindClusterConfig is the user config found in config.yaml + defaults
	// it contains info about flux install for example.
	kindClusterConfig := config.KindClusterConfig("tmp-mgmt")

	kindConfig, err := k8sclient.GetKubernetesClient(kubeconfigPath, kindContext)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients := &KubernetesClients{
		TempManagementCluster: kindConfig,
		PermManagementCluster: nil,                                        // Initialize to nil to indicate that the permanent cluster has not been created yet.
		WorkloadClusters:      make(map[string]*k8sclient.CluserAuthInfo), // Initialize the map to an empty map
	}

	// Install Cluster API on the kind cluster. kind is a temporary "CAPI management cluster" which will be used to provision
	// a cluster in the cloud which will be used as a permanent "CAPI management cluster" for the workload clusters.
	log.Info("Installing Cluster API on `kind` cluster")
	capi, err := capi.NewClusterAPI(log, kubeClients.TempManagementCluster, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating Cluster API client: %v", err)
	}

	if err := capi.InstallClusterAPI(); err != nil {
		return fmt.Errorf("error installing Cluster API: %v", err)
	}

	// Install FluxCD on the kind cluster
	log.Info("Installing FluxCD on `kind` cluster")
	kindFluxCD, err := fluxcd.NewFluxCD(log, kindClusterConfig.Flux, kubeClients.TempManagementCluster)
	if err != nil {
		return fmt.Errorf("error creating FluxCD client: %v", err)
	}

	if err := kindFluxCD.InstallFluxCD(); err != nil {
		return fmt.Errorf("error installing FluxCD: %v", err)
	}

	// Waiting for "all" Flux resources to be ready is tricky, because it is multi-step process
	// Once GitRepo and Kustomization are applied, flux will apply other manifests from the repo
	// Need to wait for all these:
	/*
		% flux get all
		NAME                            REVISION                SUSPENDED       READY   MESSAGE
		gitrepository/flux-system       develop@sha1:d23c95cc   False           True    stored artifact for revision 'develop@sha1:d23c95cc'

		NAME                            REVISION                SUSPENDED       READY   MESSAGE
		kustomization/caaph             develop@sha1:d23c95cc   False           True    Applied revision: develop@sha1:d23c95cc
		kustomization/caaph-cni         develop@sha1:d23c95cc   False           True    Applied revision: develop@sha1:d23c95cc
		kustomization/flux-system       develop@sha1:d23c95cc   False           True    Applied revision: develop@sha1:d23c95cc
	*/
	log.Info("Waiting for all Flux resources to become Ready")
	err = kindFluxCD.WaitForFluxResources()
	if err != nil {
		return fmt.Errorf("error waiting for Flux resources: %v", err)
	}

	// Now FLux has applied cluster manifests from the repo and we should wait for the cluster(s) to be ready
	capi.WaitForClusterFullyRunning("cluster-mgmt", "cluster-mgmt")

	// Pivot to the permanent management cluster
	if err := capi.PivotCluster(kubeClients.PermManagementCluster); err != nil {
		return fmt.Errorf("error pivoting to permanent cluster: %v", err)
	}

	return nil
}

func Uninstall(log logr.Logger, cfg *config.Config) error {
	log.Info("Suspending all FluxCD Kustomizations and HelmReleases")
	/*
		if err := fluxcdClientTODO.SuspendAll(); err != nil {
			return fmt.Errorf("error suspending all FluxCD resources: %v", err)
		}

		log.Info("Deleting All Cluster API Clusters")
		if err := capi.DeleteAllClusters(log); err != nil {
			return fmt.Errorf("error deleting all Cluster API clusters: %v", err)
		}

		log.Info("Deleting kind cluster")
		if err := kind.DeleteCluster(); err != nil {
			return fmt.Errorf("error deleting kind cluster: %v", err)
		}

		log.Info("Uninstalling complete")
	*/
	return nil
}
