package main

import (
	"log"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/capi"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/fluxcd"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/kind"
)

func main() {

	const kubeconfigPath = "/path/to/kubeconfig"
	const kindClusterName = "kind-cluster"

	const kindContext = "kind-kind"
	// const otherClusterContext = "other-cluster-context"
	// otherClientset, err := k8sclient.GetKubernetesClient(kubeconfigPath, otherClusterContext)
	// if err != nil {
	//		log.Fatalf("Failed to create Kubernetes client for other cluster: %v", err)
	//}

	// Create a kind cluster and get its kubeconfig
	_, err := kind.CreateCluster(kindClusterName, kubeconfigPath)
	if err != nil {
		log.Fatalf("Error creating kind cluster: %v", err)
	}

	kindClientset, err := k8sclient.GetKubernetesClient(kubeconfigPath, kindContext)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client for kind cluster: %v", err)
	}

	kubeClients, err := kubeconfig.GetKubernetesClients("/path/to/kubeconfig", []string{"tmp-mgmt", "cluster-mgmt"})
	if err != nil {
		log.Fatalf("Failed to get Kubernetes clients: %v", err)
	}

	/// if err := capi.InitClusterAPI(clientset, []string{"aws"}); err != nil {
	if err := capi.InitClusterAPI(kindClientset); err != nil {
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
