package main

import (
	"log"

	"github.com/olga-mir/k8s-multi-cluster/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/pkg/kind"
)

func main() {

	const kubeconfigPath = "/path/to/kubeconfig"
	const kindClusterName = "kind-cluster"

	// Create a kind cluster and get its kubeconfig
	_, err := kind.CreateCluster(kindClusterName, kubeconfigPath)
	if err != nil {
		log.Fatalf("Error creating kind cluster: %v", err)
	}

	// Initialize a Kubernetes client with the kind cluster kubeconfig
	clientset, err := k8sclient.GetKubernetesClient(kubeconfigPath)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	if err := capi.InitClusterAPI(clientset, []string{"aws"}); err != nil {
		log.Fatalf("Failed to initialize Cluster API: %v", err)
	}

	// Install FluxCD on the kind cluster
	if err := fluxcd.InstallFlux("path/to/kind/kubeconfig", "git-repo-url", "branch"); err != nil {
		log.Fatalf("Error installing FluxCD: %v", err)
	}

	// Pivot to the permanent management cluster
	if err := capi.PivotCluster("path/to/temp/kubeconfig", "path/to/permanent/kubeconfig"); err != nil {
		log.Fatalf("Error pivoting to permanent cluster: %v", err)
	}
}
