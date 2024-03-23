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
	"text/template"
	"time"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TODO - rework utils to receiver methods

// WaitAllResourcesReady waits for all specified resources to be ready in the given namespaces.
// If namespaces array is empty the function returns immediatelly
func WaitAllResourcesReady(clusterAuth k8sclient.ClusterAuthInfo, namespaces []string, gvr []schema.GroupVersionResource) error {
	if len(namespaces) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	resultChan := make(chan error, len(namespaces)*len(gvr))

	for _, ns := range namespaces {
		for _, resource := range gvr {
			wg.Add(1)
			go func(ns string, resource schema.GroupVersionResource) {
				defer wg.Done()
				err := waitForResourceReady(clusterAuth.Config, ns, resource, 10*time.Minute)
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
		// TODO - is it better to just panic here to minimise a little the if-err hell?
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

func waitForResourceReady(restConfig *rest.Config, namespace string, resource schema.GroupVersionResource, timeout time.Duration) error {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Setup a ticker for periodic checks
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Setup a deadline for timeout
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-time.After(time.Until(deadline)): // TODO - "case <-timeout"?
			return fmt.Errorf("timeout waiting for resource %s in namespace %s to be ready", resource.Resource, namespace)

		case <-ticker.C:
			// Check resource status
			ready, err := isResourceReady(dynamicClient, namespace, resource)
			if err != nil {
				return err
			}
			if ready {
				return nil
			}
		}
	}
}

func isResourceReady(dynamicClient dynamic.Interface, namespace string, gvr schema.GroupVersionResource) (bool, error) {
	resources, err := dynamicClient.Resource(gvr).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list resources for %s: %w", gvr.Resource, err)
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
			return false, nil
		}
	}
	return true, nil
}

func ResourcesExist(restConfig *rest.Config, namespace string, resourceName string, gvr schema.GroupVersionResource) (bool, error) {
	// TODO - signature inconsistent with above function, but this can be solved later with creating a reciver object for utils.
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return false, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	_, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil // Resource does not exist
		}
		// For other errors, return them
		return false, fmt.Errorf("error: %w", err)
	}
	return true, nil

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

type ClusterNameData struct {
	Name string
}

// GetCAPIClusterCtxName generates a CAPI cluster context name using a template and data.
func GetCAPIClusterNameAndContext(data ClusterNameData) (string, string, error) {
	t, err := template.New("clustername").Parse(config.DefaultCAPIClusterNameTpl)
	if err != nil {
		return "", "", err
	}

	var tplClusterName bytes.Buffer
	if err := t.Execute(&tplClusterName, data); err != nil {
		return "", "", err
	}
	clusterName := tplClusterName.String()

	tCtx, err := template.New("clusterctx").Parse(config.DefaultCAPIContextNameTpl)
	if err != nil {
		return "", "", err
	}

	var tplClusterCtx bytes.Buffer
	if err := tCtx.Execute(&tplClusterCtx, data); err != nil {
		return "", "", err
	}
	clusterCtx := tplClusterCtx.String()

	return clusterName, clusterCtx, nil
}

// mergeKubeconfigs merges the content of srcKubeconfig into dstKubeconfigPath.
// srcKubeconfig is a kubeconfig file in a string form
// dstKubeconfigPath is the path to the destination kubeconfig file, which already contains other content.
// TODO - this should not be a ClusterAPI method
func MergeKubeconfigs(srcKubeconfig, dstKubeconfigPath string) error {
	// Load the destination kubeconfig
	dstConfig, err := clientcmd.LoadFromFile(dstKubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load destination kubeconfig: %w", err)
	}

	// Parse the source kubeconfig from the string
	srcConfig, err := clientcmd.Load([]byte(srcKubeconfig))
	if err != nil {
		return fmt.Errorf("failed to parse source kubeconfig: %w", err)
	}

	// Merge srcConfig into dstConfig
	for key, value := range srcConfig.Clusters {
		dstConfig.Clusters[key] = value
	}
	for key, value := range srcConfig.Contexts {
		dstConfig.Contexts[key] = value
	}
	for key, value := range srcConfig.AuthInfos {
		dstConfig.AuthInfos[key] = value
	}

	// Write the merged configuration back to the destination kubeconfig file
	if err = clientcmd.WriteToFile(*dstConfig, dstKubeconfigPath); err != nil {
		return fmt.Errorf("failed to write merged kubeconfig: %w", err)
	}

	return nil
}

func RenderTemplateToFile(inputFilePath, outputFilePath string, data interface{}) error {
	// Parse the template from the input file
	tmpl, err := template.ParseFiles(inputFilePath)
	if err != nil {
		return err
	}

	// Create the output file
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Execute the template, substituting the data
	return tmpl.Execute(outputFile, data)
}
