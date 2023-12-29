package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/kind"
)

type KubernetesClients struct {
	TempManagementCluster *k8sclient.ClusterClient            // Temporary management cluster (kind)
	PermManagementCluster *k8sclient.ClusterClient            // Permanent management cluster
	WorkloadClusters      map[string]*k8sclient.ClusterClient // Map of workload clusters
}

// GetKubernetesClients creates KubernetesClients struct with clients for each workload cluster.
func main() {

	const kindContext = "kind-kind"

	kubeconfigPath := os.Getenv("K8S_MULTI_KUBECONFIG")
	if kubeconfigPath == "" {
		log.Fatalf("K8S_MULTI_KUBECONFIG environment variable is not set")
	}

	// Create a kind cluster and get its kubeconfig
	log.Printf("Create `kind` cluster")
	_, err := kind.CreateCluster(kubeconfigPath)
	if err != nil {
		log.Fatalf("Error creating kind cluster: %v", err)
	}

	kindConfig, err := k8sclient.GetKubernetesClient(kubeconfigPath, kindContext)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients := &KubernetesClients{
		TempManagementCluster: kindConfig,
		PermManagementCluster: nil,                                       // Initialize to nil to indicate that the permanent cluster has not been created yet.
		WorkloadClusters:      make(map[string]*k8sclient.ClusterClient), // Initialize the map to an empty map
	}

	if err := capi.InitClusterAPI(kubeClients.TempManagementCluster.Config, kubeconfigPath); err != nil {
		log.Fatalf("Failed to initialize Cluster API: %v", err)
	}

	// Install FluxCD on the kind cluster
	// $REPO_ROOT/k8s-platform/flux/v$FLUXCD_VERSION/gotk-components.yaml

	path := utilsRepoRoot() + "/k8s-platform/flux/v1.22.1/gotk-components.yaml"
	if err := fluxcd.InstallFluxCD(kubeClients.TempManagementCluster.Config, path); err != nil {
		log.Fatalf("Error installing FluxCD: %v", err)
	}

	// Pivot to the permanent management cluster
	// if err := capi.PivotCluster("path/to/temp/kubeconfig", "path/to/permanent/kubeconfig"); err != nil {
	//	log.Fatalf("Error pivoting to permanent cluster: %v", err)
	//}
}

// TODO - move this away
func utilsRepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}

	repoRoot := strings.TrimSpace(out.String())

	return repoRoot
}
