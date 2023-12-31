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
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
