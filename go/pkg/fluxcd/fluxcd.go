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
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	appconfig "github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

// FluxCD handles the installation of FluxCD
type FluxCD struct {
	log           logr.Logger
	fluxConfig    appconfig.FluxConfig
	githubConfig  appconfig.GithubConfig
	clusterAuth   k8sclient.ClusterAuthInfo
	runtimeClient runtimeclient.Client
}

// NewFluxCD creates a new FluxCD with the provided configurations
func NewFluxCD(log logr.Logger, fluxConfig appconfig.FluxConfig, githubConfig appconfig.GithubConfig, clusterAuth *k8sclient.ClusterAuthInfo) (*FluxCD, error) {
	// Add Flux resource to scheme to the runtime scheme. Fixes this runtime error:
	// `failed to create GitRepository: no kind is registered for the type v1beta1.GitRepository in scheme "pkg/runtime/scheme.go:100"`
	runtimeScheme := runtime.NewScheme()
	sourcev1.AddToScheme(runtimeScheme)
	kustomizev1.AddToScheme(runtimeScheme)

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %s", err)
	}

	// Create a new client to interact with cluster and host specific information
	runtimeClient, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: runtimeScheme})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %s", err)
	}

	return &FluxCD{
		log:           log,
		fluxConfig:    fluxConfig,
		clusterAuth:   *clusterAuth,
		runtimeClient: runtimeClient,
		githubConfig:  githubConfig,
	}, nil
}

func (f *FluxCD) InstallFluxCD() error {
	dynamicClient, err := dynamic.NewForConfig(f.clusterAuth.Config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	manifestPath := utils.RepoRoot() + "/k8s-platform/flux/" + "v" + f.fluxConfig.Version

	// Apply gotk-components.yaml first
	f.log.Info("Applying gotk-components")
	if err := utils.ApplyManifestsFile(dynamicClient, filepath.Join(manifestPath, "gotk-components.yaml")); err != nil {
		return err
	}

	// Wait for CRDs to be established
	fluxCRDs := []string{"kustomizations.kustomize.toolkit.fluxcd.io", "gitrepositories.source.toolkit.fluxcd.io"}
	f.log.Info("Waiting for Flux CRDs to become established")
	if err := utils.WaitForCRDs(f.clusterAuth.Config, fluxCRDs); err != nil {
		return err
	}

	f.CreateFluxSystemSecret()

	if err := f.createGitRepository(); err != nil {
		log.Fatalf("Error creating GitRepository: %s", err)
	}

	if err := f.createKustomization(); err != nil {
		log.Fatalf("Error creating Kustomization: %s", err)
	}

	// TODO. We need to add a wait here because next step in `builder` will be calling to wait for all
	// FluxCD resources. I think it is being called too early when there are still no resources. Then
	// it also needs to wait for the Flux resources which are applied from the repo.
	f.log.Info("Sleeping for 3 min to allow Flux to apply resources from the repository")
	time.Sleep(3 * time.Minute)

	return nil
}

func (f *FluxCD) createGitRepository() error {
	gitRepo := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: f.fluxConfig.Namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			Interval: metav1.Duration{Duration: 2 * time.Minute},
			URL:      f.githubConfig.URL,
			Reference: &sourcev1.GitRepositoryRef{
				Branch: f.githubConfig.Branch,
			},
			SecretRef: &meta.LocalObjectReference{
				Name: "flux-system",
			},
		},
	}

	if err := f.runtimeClient.Create(context.TODO(), gitRepo); err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}
	return nil
}

func (f *FluxCD) createKustomization() error {
	kustomization := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: f.fluxConfig.Namespace,
		},
		Spec: kustomizev1.KustomizationSpec{
			Interval: metav1.Duration{Duration: 2 * time.Minute},
			Path:     "./clusters/tmp-mgmt", // TODO - defaults?
			Prune:    true,
			SourceRef: kustomizev1.CrossNamespaceSourceReference{
				Kind: "GitRepository",
				Name: "flux-system",
			},
		},
	}

	if err := f.runtimeClient.Create(context.TODO(), kustomization); err != nil {
		return fmt.Errorf("failed to create Kustomization: %w", err)
	}
	return nil
}

func (f *FluxCD) CreateFluxSystemSecret() error {
	f.log.Info("Creating secret for Flux")

	secretData := make(map[string][]byte)

	key, err := os.ReadFile(f.fluxConfig.KeyPath)
	if err != nil {
		log.Fatalf("Error reading key file: %s", err.Error())
	}
	secretData["identity"] = key

	keyPub, err := os.ReadFile(f.fluxConfig.KeyPath + ".pub")
	if err != nil {
		log.Fatalf("Error reading key pub file: %s", err.Error())
	}
	secretData["identity.pub"] = keyPub

	secretData["known_hosts"] = []byte(appconfig.GithubKnownHosts)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: f.fluxConfig.Namespace,
		},
		Data: secretData,
	}

	_, err = f.clusterAuth.Clientset.CoreV1().Secrets(f.fluxConfig.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating secret: %s", err)
	}

	return nil
}

func (f *FluxCD) WaitForFluxResources() error {
	// Define the GVRs for Flux resources
	fluxGVRs := []schema.GroupVersionResource{
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2beta2", Resource: "helmreleases"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Resource: "helmrepositories"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Resource: "helmcharts"},
	}

	clusterNamespaces, err := utils.ListAllNamespacesWithPrefix(f.clusterAuth.Clientset, "cluster")
	if err != nil {
		return fmt.Errorf("failed to list all namespaces: %w", err)
	}
	namespaces := append(clusterNamespaces, appconfig.ProjectNamespaces...)

	err = utils.WaitAllResourcesReady(f.clusterAuth, namespaces, fluxGVRs)
	if err != nil {
		return err
	}
	return nil
}

func (f *FluxCD) SuspendAll() error {
	f.log.Info("TODO - implementation. Suspending Flux resources")
	return nil
}

func (f *FluxCD) SuspendKustomization(name string) error {
	// There is no suspend method in the Kustomization API, so we need to suspend the Kustomization
	// https://pkg.go.dev/github.com/fluxcd/kustomize-controller/api@v1.2.1/v1#pkg-functions
	// TODO - verify that there is no method

	// Fetch the Kustomization
	kustomization := &kustomizev1.Kustomization{}
	if err := f.runtimeClient.Get(context.TODO(), runtimeclient.ObjectKey{
		Name:      name,
		Namespace: f.fluxConfig.Namespace,
	}, kustomization); err != nil {
		return fmt.Errorf("failed to get kustomization: %w", err)
	}

	// Suspend the Kustomization
	kustomization.Spec.Suspend = true
	if err := f.runtimeClient.Update(context.TODO(), kustomization); err != nil {
		return fmt.Errorf("failed to suspend kustomization: %w", err)
	}

	f.log.Info("Suspended kustomization", "name", name, "namespace", f.fluxConfig.Namespace)

	return nil
}
