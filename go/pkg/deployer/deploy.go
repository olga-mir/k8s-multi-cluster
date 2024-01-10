package deployer

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/kind"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
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
	// Create a kind cluster and get its kubeconfig
	log.Info("Create `kind` cluster")
	err := kind.CreateCluster(cfg.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("error creating kind cluster: %v", err)
	}

	// TODO - naming. kindClusterConfig is the user config found in config.yaml + defaults
	// it contains info about flux install for example.
	kindClusterConfig := config.GetKindClusterConfig(config.DefaultKindClusterName)

	kindConfig, err := k8sclient.GetKubernetesClient(cfg.KubeconfigPath, config.DefaultKindClusterCtxName)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients := &KubernetesClients{
		TempManagementCluster: kindConfig,
		PermManagementCluster: nil,                                        // Initialize to nil to indicate that the permanent cluster has not been created yet.
		WorkloadClusters:      make(map[string]*k8sclient.CluserAuthInfo), // Initialize the map to an empty map
	}

	permMgmtClusterName, permMgmtClusterCtxName, err := utils.GetCAPIClusterNameAndContext(utils.ClusterNameData{Name: "mgmt"})
	if err != nil {
		return fmt.Errorf("error getting cluster name and context: %v", err)
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
	kindFluxCD, err := fluxcd.NewFluxCD(log, kindClusterConfig.Flux, cfg.Github, kubeClients.TempManagementCluster)
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

	// Now FLux has applied cluster manifests from the repo and we should wait for the cluster(s) to be ready
	tmpMgmtCAPI.WaitForClusterFullyRunning(permMgmtClusterName, permMgmtClusterName)

	// After cluster is ready we need to get its kubeconfig, then suspend flux and pivot management cluster
	// flux --kubeconfig $KUBECONFIG --context kind-kind suspend kustomization flux-system
	kubeClients.PermManagementCluster = &k8sclient.CluserAuthInfo{}
	err = tmpMgmtCAPI.GetClusterAuthInfo(permMgmtClusterName, permMgmtClusterCtxName, kubeClients.PermManagementCluster)
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
	permMgmtClusterConfig := config.GetKindClusterConfig(permMgmtClusterName)
	permMgmtFluxCD, err := fluxcd.NewFluxCD(log, permMgmtClusterConfig.Flux, cfg.Github, kubeClients.PermManagementCluster)
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
