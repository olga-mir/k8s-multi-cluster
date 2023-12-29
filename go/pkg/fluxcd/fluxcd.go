package fluxcd

import (
	"context"
	"fmt"
	"os"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// InstallFluxCD applies the FluxCD manifests to the cluster
func InstallFluxCD(restConfig *rest.Config, manifestPath string) error {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	files, err := os.ReadDir(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest directory: %w", err)
	}

	for _, file := range files {
		yamlData, err := os.ReadFile(filepath.Join(manifestPath, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file.Name(), err)
		}

		decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		obj := &unstructured.Unstructured{}
		_, _, err = decUnstructured.Decode(yamlData, nil, obj)
		if err != nil {
			return fmt.Errorf("failed to decode %s: %w", file.Name(), err)
		}

		_, err = dynamicClient.Resource(obj.GroupVersionKind().GroupVersion().WithResource(obj.GetKind())).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to apply %s: %w", file.Name(), err)
		}
	}

	return nil
}

func CreateGitRepository(kubeClient client.Client, repoUrl, namespace string) error {
	gitRepo := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-repository", // Change as needed
			Namespace: namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL: repoUrl,
			// Include additional configuration as required...
		},
	}

	// Create or Update the GitRepository
	err := kubeClient.Create(context.TODO(), gitRepo)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}

	return nil
}
