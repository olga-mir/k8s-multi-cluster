package k8sclient

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterClient is a struct that contains a Kubernetes clientset and a Kubernetes config.
// in somecases we can't use clientset and have to pass REST Config, e.g. in InitClusterAPI
type ClusterClient struct {
	Clientset *kubernetes.Clientset
	Config    *rest.Config
}

type KubernetesClients struct {
	TempManagementCluster *ClusterClient            // Temporary management cluster (kind)
	PermManagementCluster *ClusterClient            // Permanent management cluster
	WorkloadClusters      map[string]*ClusterClient // Map of workload clusters
}

// GetKubernetesClients creates KubernetesClients struct with clients for each workload cluster.
func GetKubernetesClients(kubeconfigPath string, workloadClusterContexts []string) (*KubernetesClients, error) {
	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clients := &KubernetesClients{
		WorkloadClusters: make(map[string]*ClusterClient),
	}

	for _, context := range workloadClusterContexts {
		clientConfig := clientcmd.NewNonInteractiveClientConfig(*config, context, &clientcmd.ConfigOverrides{}, nil)
		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, err
		}

		clients.WorkloadClusters[context] = &ClusterClient{Clientset: clientset, Config: restConfig}
	}

	return clients, nil
}
