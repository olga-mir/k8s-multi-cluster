package fluxcd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	manifestPath := utils.RepoRoot() + "/k8s-platform/flux/" + "v" + fluxcdVersion

	// Apply gotk-components.yaml first
	if err := applyManifest(dynamicClient, filepath.Join(manifestPath, "gotk-components.yaml")); err != nil {
		return err
	}

	// Wait for CRDs to be established
	if err := waitForCRDs(dynamicClient, []string{"kustomizations.kustomize.toolkit.fluxcd.io", "gitrepositories.source.toolkit.fluxcd.io"}); err != nil {
		return err
	}

	// Then apply kustomization.yaml
	if err := applyManifest(dynamicClient, filepath.Join(manifestPath, "kustomization.yaml")); err != nil {
		return err
	}

	createFluxSystemSecret(client, fluxKeyPath, fluxKeyPath+".pub", githubKnownHosts)

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

	if err := createGitRepository(kubeClient, repoUrl, namespace); err != nil {
		log.Fatalf("Error creating GitRepository: %s", err)
	}

	if err := createKustomization(kubeClient, namespace); err != nil {
		log.Fatalf("Error creating Kustomization: %s", err)
	}

	return nil
}

func createGitRepository(kubeClient runtimeClient.Client, repoUrl, namespace string) error {
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

func createKustomization(kubeClient runtimeClient.Client, namespace string) error {
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

func applyManifest(dynamicClient dynamic.Interface, manifestFile string) error {
	yamlData, err := os.ReadFile(manifestFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", manifestFile, err)
	}

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err = decUnstructured.Decode(yamlData, nil, obj)
	if err != nil {
		return fmt.Errorf("failed to decode %s: %w", manifestFile, err)
	}

	_, err = dynamicClient.Resource(obj.GroupVersionKind().GroupVersion().WithResource(obj.GetKind())).Namespace(obj.GetNamespace()).Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to apply %s: %w", manifestFile, err)
	}
	return nil
}

func waitForCRDs(dynamicClient dynamic.Interface, crds []string) error {
	for _, crd := range crds {
		if err := waitUntilCRDEstablished(dynamicClient, crd); err != nil {
			return err
		}
	}
	return nil
}

func waitUntilCRDEstablished(dynamicClient dynamic.Interface, crdName string) error {
	w, err := dynamicClient.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: "metadata.name=" + crdName,
	})
	if err != nil {
		return fmt.Errorf("failed to watch CRD %s: %w", crdName, err)
	}
	defer w.Stop()

	timeout := time.After(5 * time.Minute)
	for {
		select {
		case event := <-w.ResultChan():
			if event.Type == watch.Modified {
				crd := event.Object.(*apiextensionsv1.CustomResourceDefinition)
				for _, cond := range crd.Status.Conditions {
					if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
						return nil
					}
				}
			}
		case <-timeout:
			return fmt.Errorf("timeout waiting for CRD %s to be established", crdName)
		}
	}
}
