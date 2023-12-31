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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	appconfig "github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

// FluxCDInstaller handles the installation of FluxCD
type FluxCDInstaller struct {
	log        logr.Logger
	fluxConfig appconfig.FluxConfig
	k8sClient  *kubernetes.Clientset
	restConfig *rest.Config
}

// NewFluxCDInstaller creates a new FluxCDInstaller with the provided configurations
func NewFluxCDInstaller(log logr.Logger, fluxConfig appconfig.FluxConfig, restConfig *rest.Config, client *kubernetes.Clientset) *FluxCDInstaller {
	return &FluxCDInstaller{
		log:        log,
		fluxConfig: fluxConfig,
		k8sClient:  client,
		restConfig: restConfig,
	}
}

func (f *FluxCDInstaller) InstallFluxCD() error {
	dynamicClient, err := dynamic.NewForConfig(f.restConfig)
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
	if err := utils.WaitForCRDs(f.restConfig, fluxCRDs); err != nil {
		return err
	}

	// Then apply kustomization.yaml
	f.log.Info("Applying kustomization")
	if err := utils.ApplyManifestsFile(dynamicClient, filepath.Join(manifestPath, "kustomization.yaml")); err != nil {
		return err
	}

	f.createFluxSystemSecret()

	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("error getting kubeconfig: %s", err)
	}

	// Create a new client to interact with cluster and host specific information
	kubeClient, err := runtimeClient.New(cfg, runtimeClient.Options{})
	if err != nil {
		return fmt.Errorf("error creating client: %s", err)
	}

	if err := f.createGitRepository(kubeClient); err != nil {
		log.Fatalf("Error creating GitRepository: %s", err)
	}

	if err := f.createKustomization(kubeClient); err != nil {
		log.Fatalf("Error creating Kustomization: %s", err)
	}

	return nil
}

func (f *FluxCDInstaller) createGitRepository(kubeClient runtimeClient.Client) error {
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

	if err := kubeClient.Create(context.TODO(), gitRepo); err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}
	return nil
}

func (f *FluxCDInstaller) createKustomization(kubeClient runtimeClient.Client) error {
	kustomization := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-system",
			Namespace: f.fluxConfig.Namespace,
		},
		Spec: kustomizev1.KustomizationSpec{
			Interval: metav1.Duration{Duration: 2 * time.Minute},
			Path:     "./clusters/cluster-mgmt", // TODO - defaults?
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

func (f *FluxCDInstaller) createFluxSystemSecret() {
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

	_, err = f.k8sClient.CoreV1().Secrets(f.fluxConfig.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating secret: %s", err.Error())
	}

	log.Println("Secret created successfully")
}
