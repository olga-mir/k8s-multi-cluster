package fluxcd

import (
	"context"
	"fmt"

	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"golang.org/x/vuln/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createGitRepository(kubeClient client.Client, repoUrl, namespace string) error {
	gitRepo := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-repository",
			Namespace: namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL: repoUrl,
			// Additional configuration...
		},
	}

	err := kubeClient.Create(context.TODO(), gitRepo)
	if err != nil {
		return fmt.Errorf("failed to create GitRepository: %w", err)
	}

	return nil
}
