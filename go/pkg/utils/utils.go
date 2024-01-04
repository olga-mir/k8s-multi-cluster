package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// WaitAllResourcesReady waits for all specified resources to be ready in the given namespaces.
func WaitAllResourcesReady(clusterAuth k8sclient.CluserAuthInfo, namespaces []string, gvr []schema.GroupVersionResource) error {
	if len(namespaces) == 0 {
		// If no namespaces are provided, use a function to list all namespaces
		var err error
		namespaces, err = ListAllNamespacesWithPrefix(clusterAuth.Clientset, "")
		if err != nil {
			return fmt.Errorf("failed to list all namespaces: %w", err)
		}
	}

	var wg sync.WaitGroup
	resultChan := make(chan error, len(namespaces)*len(gvr))

	for _, ns := range namespaces {
		for _, resource := range gvr {
			wg.Add(1)
			go func(ns string, resource schema.GroupVersionResource) {
				defer wg.Done()
				err := waitForResourceReady(clusterAuth.Config, ns, resource)
				resultChan <- err
			}(ns, resource)
		}
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(resultChan)

	// Collect results
	for err := range resultChan {
		if err != nil {
			return err // Return on the first error encountered
		}
	}
	return nil
}

// ListAllNamespaces lists all namespaces in the cluster
// if prefix is provided only namespace with this prefix are returned
func ListAllNamespacesWithPrefix(k8sClient *kubernetes.Clientset, prefix string) ([]string, error) {
	namespaceList, err := k8sClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	var namespaces []string
	for _, ns := range namespaceList.Items {
		if strings.HasPrefix(ns.Name, prefix) {
			namespaces = append(namespaces, ns.Name)
		}
	}

	return namespaces, nil
}

func waitForResourceReady(restConfig *rest.Config, namespace string, gvr schema.GroupVersionResource) error {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Use the provided gvr for listing resources
	resources, err := dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list resources for %s: %w", gvr.Resource, err)
	}

	for _, resource := range resources.Items {
		conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
		if err != nil || !found {
			continue // Skip resources without status conditions
		}

		ready := false
		for _, cond := range conditions {
			condition, ok := cond.(map[string]interface{})
			if !ok {
				continue
			}
			if condition["type"] == "Ready" && condition["status"] == "True" {
				ready = true
				break
			}
		}

		if !ready {
			return fmt.Errorf("resource %s/%s is not ready", gvr.Resource, resource.GetName())
		}
	}

	return nil
}

// ApplyManifestsFile applies all manifests in a provided file
func ApplyManifestsFile(dynamicClient dynamic.Interface, manifestFile string) error {
	fileData, err := os.ReadFile(manifestFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", manifestFile, err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(fileData), 4096)
	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break // End of file, exit the loop
			}
			return fmt.Errorf("failed to decode YAML document: %w", err)
		}

		if obj.Object == nil {
			continue // Skip empty objects
		}

		gvr := schema.GroupVersionResource{
			Group:    obj.GetObjectKind().GroupVersionKind().Group,
			Version:  obj.GetObjectKind().GroupVersionKind().Version,
			Resource: getResourceName(obj.GetKind()),
		}

		_, err := dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Create(context.TODO(), &obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to apply resource (Kind: %s, Name: %s): %w", obj.GetKind(), obj.GetName(), err)
		}
	}

	return nil
}

func getResourceName(kind string) string {
	// Handle special cases
	specialCases := map[string]string{
		"Endpoints":                "endpoints",
		"PodSecurityPolicy":        "podsecuritypolicies",
		"NetworkPolicy":            "networkpolicies",
		"CustomResourceDefinition": "customresourcedefinitions",
	}

	if resourceName, ok := specialCases[kind]; ok {
		return resourceName
	}

	// Default case: add an 's' to the kind
	return strings.ToLower(kind) + "s"
}

func WaitForCRDs(config *rest.Config, crds []string) error {
	apiExtClient, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating API extensions client: %w", err)
	}
	for _, crd := range crds {
		if err := waitUntilCRDEstablished(apiExtClient, crd); err != nil {
			return err
		}
	}
	return nil
}

func waitUntilCRDEstablished(clientSet apiextensionsclientset.Interface, crdName string) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for CRD %s to be established", crdName)
		case <-ticker.C:
			crd, err := clientSet.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crdName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting CRD %s: %w", crdName, err)
			}

			for _, cond := range crd.Status.Conditions {
				if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
					return nil
				}
			}
		}
	}
}

func RepoRoot() string {
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

func GetCurrentContextName(config *rest.Config, kubeconfigPath string) (string, error) {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	currentContext := kubeconfig.CurrentContext
	if currentContext == "" {
		return "", fmt.Errorf("current context is not set in kubeconfig")
	}

	return currentContext, nil
}
