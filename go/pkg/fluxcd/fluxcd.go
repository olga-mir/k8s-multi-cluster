package fluxcd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	// kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

// InstallFluxCD applies the FluxCD manifests to the cluster
func InstallFluxCD(restConfig *rest.Config, client *kubernetes.Clientset) error {

	fluxcdVersion := os.Getenv("FLUXCD_VERSION")
	if fluxcdVersion == "" {
		log.Fatalf("FLUXCD_VERSION environment variable is not set")
	}

	fluxKeyPath := os.Getenv("FLUX_KEY_PATH")
	if fluxcdVersion == "" {
		log.Fatalf(" environment variable is not set")
	}

	githubKnownHosts := os.Getenv("GITHUB_KNOWN_HOSTS")
	if fluxcdVersion == "" {
		log.Fatalf("GITHUB_KNOWN_HOSTS environment variable is not set")
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	createFluxSystemSecret(client, fluxKeyPath, fluxKeyPath+".pub", githubKnownHosts)

	manifestPath := utils.RepoRoot() + "/k8s-platform/flux/" + fluxcdVersion + "/gotk-components.yaml"
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

	// TODO - move this out of "install" to another function
	repoUrl := "ssh://git@github.com/olga-mir/k8s-multi-cluster"
	namespace := "flux-system"

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("error getting kubeconfig: %s", err)
	}

	// Create a new client to interact with cluster and host specific information
	kubeClient, err := runtimeClient.New(cfg, runtimeClient.Options{})
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}

	if err := CreateGitRepository(kubeClient, repoUrl, namespace); err != nil {
		log.Fatalf("Error creating GitRepository: %s", err)
	}

	if err := CreateKustomization(kubeClient, namespace); err != nil {
		log.Fatalf("Error creating Kustomization: %s", err)
	}

	return nil
}

func CreateGitRepository(kubeClient runtimeClient.Client, repoUrl, namespace string) error {
	gitRepo := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			Interval: metav1.Duration{Duration: 2 * time.Minute},
			URL:      repoUrl,
			Reference: &sourcev1.GitRepositoryRef{
				Branch: "develop",
			},
			SecretRef: &meta.LocalObjectReference{
				Name: "flux-system",
			},
		},
	}

	if err := kubeClient.Create(context.TODO(), gitRepo); err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}
	return nil
}

func CreateKustomization(kubeClient runtimeClient.Client, namespace string) error {
	kustomization := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: namespace,
		},
		Spec: kustomizev1.KustomizationSpec{
			Interval: metav1.Duration{Duration: 2 * time.Minute},
			Path:     "./clusters/cluster-mgmt",
			Prune:    true,
			SourceRef: kustomizev1.CrossNamespaceSourceReference{
				Kind: "GitRepository",
				Name: "flux-system",
			},
		},
	}

	if err := kubeClient.Create(context.TODO(), kustomization); err != nil {
		return fmt.Errorf("failed to create Kustomization: %w", err)
	}
	return nil
}

func createFluxSystemSecret(clientset *kubernetes.Clientset, keyPath, keyPubPath, knownHosts string) {
	secretData := make(map[string][]byte)

	key, err := os.ReadFile(keyPath)
	if err != nil {
		log.Fatalf("Error reading key file: %s", err.Error())
	}
	secretData["identity"] = key

	keyPub, err := os.ReadFile(keyPubPath)
	if err != nil {
		log.Fatalf("Error reading key pub file: %s", err.Error())
	}
	secretData["identity.pub"] = keyPub

	secretData["known_hosts"] = []byte(knownHosts)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: "flux-system",
		},
		Data: secretData,
	}

	_, err = clientset.CoreV1().Secrets("flux-system").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating secret: %s", err.Error())
	}

	log.Println("Secret created successfully")
}
