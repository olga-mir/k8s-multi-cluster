package k8sclient

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// CluserAuthInfo is a struct that contains a Kubernetes clientset and a Kubernetes config.
// in somecases we can't use clientset and have to pass REST Config, e.g. in InitClusterAPI
type CluserAuthInfo struct {
	Clientset *kubernetes.Clientset
	Config    *rest.Config
}

func GetKubernetesClient(kubeconfigPath string, context string) (*CluserAuthInfo, error) {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientConfig := clientcmd.NewNonInteractiveClientConfig(*config, context, &clientcmd.ConfigOverrides{}, nil)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &CluserAuthInfo{Clientset: clientset, Config: restConfig}, nil
}
