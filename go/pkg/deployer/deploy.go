package deployer

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/kind"
)

// KubernetesClients represents a collection of Kubernetes clients for different clusters.
// ClusterAuthInfo contains REST Config and clientset. Clientset can't be easily used with
// custom resources and clients used by Cluster API and FluxCD
// TODO - review this sructure: maybe using only REST Config and building clientset dynamically when needed.
type KubernetesClients struct {
	TempManagementCluster *k8sclient.ClusterAuthInfo            // Temporary management cluster (kind)
	PermManagementCluster *k8sclient.ClusterAuthInfo            // Permanent management cluster
	WorkloadClusters      map[string]*k8sclient.ClusterAuthInfo // Map of workload clusters
}

func Deploy(log logr.Logger, cfg *config.Config) error {
	// Create a kind cluster and get its kubeconfig
	log.Info("Create `kind` cluster")
	err := kind.CreateCluster(cfg.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating kind cluster: %v", err)
	}

	kindConfig, err := k8sclient.GetKubernetesClient(cfg.KubeconfigPath, config.DefaultKindClusterCtxName, config.DefaultKindClusterName)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients := &KubernetesClients{
		TempManagementCluster: kindConfig,
		PermManagementCluster: nil,                                         // Initialize to nil to indicate that the permanent cluster has not been created yet.
		WorkloadClusters:      make(map[string]*k8sclient.ClusterAuthInfo), // Initialize the map to an empty map
	}

	// Install Cluster API on the kind cluster. kind is a temporary "CAPI management cluster" which will be used to provision
	// a cluster in the cloud which will be used as a permanent "CAPI management cluster" for the workload clusters.
	log.Info("Installing Cluster API on `kind` cluster")
	tmpMgmtCAPI, err := capi.NewClusterAPI(log, kubeClients.TempManagementCluster, cfg.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating Cluster API client: %v", err)
	}

	if err := tmpMgmtCAPI.InstallClusterAPI(); err != nil {
		return fmt.Errorf("error installing Cluster API: %v", err)
	}

	// Install FluxCD on the kind cluster
	log.Info("Installing FluxCD on `kind` cluster")
	kindFluxCD, err := fluxcd.NewFluxCD(log, clusterConfigByName(config.DefaultKindClusterName, cfg).Flux, cfg.Github, kubeClients.TempManagementCluster)
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
		% k8s-multi-cluster % flux get all -A
		NAMESPACE       NAME                            REVISION                SUSPENDED       READY   MESSAGE
		flux-system     gitrepository/flux-system       develop@sha1:5c0b03c8   False           True    stored artifact for revision 'develop@sha1:5c0b03c8'

		NAMESPACE       NAME                            REVISION                SUSPENDED       READY   MESSAGE
		cluster-mgmt    kustomization/flux-remote       develop@sha1:5c0b03c8   False           True    Applied revision: develop@sha1:5c0b03c8
		flux-system     kustomization/caaph             develop@sha1:5c0b03c8   False           True    Applied revision: develop@sha1:5c0b03c8
		flux-system     kustomization/caaph-cni         develop@sha1:5c0b03c8   False           True    Applied revision: develop@sha1:5c0b03c8
		flux-system     kustomization/flux-system       develop@sha1:5c0b03c8   False           True    Applied revision: develop@sha1:5c0b03c8
	*/

	log.Info("Waiting for all Flux resources to become Ready")
	err = kindFluxCD.WaitForFluxResources()
	if err != nil {
		return fmt.Errorf("error waiting for Flux resources: %v", err)
	}

	// Now Flux has applied cluster manifests from the repo and we should wait for the cluster(s) to be ready
	tmpMgmtCAPI.WaitForWorkloadClusterFullyRunning("mgmt")

	// After cluster is ready we need to get its kubeconfig, then suspend flux and pivot management cluster
	kubeClients.PermManagementCluster = &k8sclient.ClusterAuthInfo{}
	err = tmpMgmtCAPI.GetClusterAuthInfoForWorkloadCluster(kubeClients.PermManagementCluster, "mgmt")
	if err != nil {
		return fmt.Errorf("error getting kubeconfig for cluster-mgmt: %v", err)
	}

	err = kindFluxCD.SuspendKustomization("flux-system")
	if err != nil {
		return fmt.Errorf("error suspending kustomization flux-system: %v", err)
	}

	// install Cluster API on permanent management cluster
	mgmtCAPI, err := capi.NewClusterAPI(log, kubeClients.PermManagementCluster, cfg.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating Cluster API client: %v", err)
	}

	log.Info("Installing Cluster API on the permanent management cluster")
	if err := mgmtCAPI.InstallClusterAPI(); err != nil {
		return fmt.Errorf("error installing Cluster API: %v", err)
	}

	// Pivot to the permanent management cluster
	if err := tmpMgmtCAPI.PivotCluster(kubeClients.PermManagementCluster); err != nil {
		return fmt.Errorf("error pivoting to permanent cluster: %v", err)
	}

	// Flux is installed on the permanent management cluster by GitOps magic that runs on temp mgmt cluster
	// But we need to provide the secret for Flux to access the repository.
	log.Info("Creating FluxCD instance for permanent management cluster")
	permMgmtFluxCD, err := fluxcd.NewFluxCD(log, clusterConfigByName("cluster-mgmt", cfg).Flux, cfg.Github, kubeClients.PermManagementCluster)
	if err != nil {
		return fmt.Errorf("error creating FluxCD client: %v", err)
	}

	if err := permMgmtFluxCD.CreateFluxSystemSecret(); err != nil {
		return fmt.Errorf("error creating FluxCD secret: %v", err)
	}

	if err := mgmtCAPI.WaitForAllClustersProvisioning(); err != nil {
		fmt.Printf("Error waiting for clusters to be provisioned: %s\n", err)
	}

	return nil
}

// TODO - this is a temp function. Need to re-think config.yaml
// so that it allows immutable cluster upgrades and what are the cluster names really mean
// cluster-01 and cluster-02 are they peers (e.g. HA design or clusters by function that need to be in multi cluster mesh
// or are they a b/g version during the cluster upgrade flow)
func clusterConfigByName(name string, cfg *config.Config) *config.ClusterConfig {
	for _, cluster := range cfg.Clusters {
		if cluster.Name == name {
			return &cluster
		}
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
		if err := tmpMgmtCAPI.DeleteAllClusters(log); err != nil {
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
