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
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	appconfig "github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

// FluxCD handles the installation of FluxCD
type FluxCD struct {
	log           logr.Logger
	fluxConfig    appconfig.FluxConfig
	clusterAuth   k8sclient.CluserAuthInfo
	runtimeClient runtimeClient.Client
}

// NewFluxCD creates a new FluxCD with the provided configurations
func NewFluxCD(log logr.Logger, fluxConfig appconfig.FluxConfig, clusterAuth *k8sclient.CluserAuthInfo) (*FluxCD, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting kubeconfig: %s", err)
	}

	// Add Flux resource to scheme to the runtime scheme. Fixes this runtime error:
	// `failed to create GitRepository: no kind is registered for the type v1beta1.GitRepository in scheme "pkg/runtime/scheme.go:100"`
	runtimeScheme := runtime.NewScheme()
	sourcev1.AddToScheme(runtimeScheme)
	kustomizev1.AddToScheme(runtimeScheme)

	// Create a new client to interact with cluster and host specific information
	runtimeClient, err := runtimeClient.New(cfg, runtimeClient.Options{Scheme: runtimeScheme})
	if err != nil {
		return nil, fmt.Errorf("error creating client: %s", err)
	}
	return &FluxCD{
		log:           log,
		fluxConfig:    fluxConfig,
		clusterAuth:   *clusterAuth,
		runtimeClient: runtimeClient,
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

	f.createFluxSystemSecret()

	if err := f.createGitRepository(); err != nil {
		log.Fatalf("Error creating GitRepository: %s", err)
	}

	if err := f.createKustomization(); err != nil {
		log.Fatalf("Error creating Kustomization: %s", err)
	}

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
			URL:      f.fluxConfig.GitHub.Repo,
			Reference: &sourcev1.GitRepositoryRef{
				Branch: f.fluxConfig.GitHub.Branch,
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

func (f *FluxCD) createFluxSystemSecret() {
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
		f.log.Error(err, "Error creating secret")
	}

	f.log.Info("Successfully created FluxCD secret")
}

func (f *FluxCD) WaitForFluxResources() error {
	// Define the GVRs for Flux resources
	fluxGVRs := []schema.GroupVersionResource{
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		{Group: "helm.toolkit.fluxcd.io", Version: "v2beta1", Resource: "helmreleases"}, // deprecated
		{Group: "helm.toolkit.fluxcd.io", Version: "v2beta2", Resource: "helmreleases"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Resource: "helmrepositories"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1beta2", Resource: "helmcharts"},
	}

	// Call the WaitAllResourcesReady function
	namespaces := []string{} // empty list means all namespaces. TODO change funciton signature to make it clearer
	err := utils.WaitAllResourcesReady(f.clusterAuth, namespaces, fluxGVRs)
	if err != nil {
		return err
	}
	return nil
}
