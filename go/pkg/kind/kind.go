package kind

import (
	"bytes"
	"fmt"
	"os/exec"
)

// CreateCluster creates a kind cluster and returns the kubeconfig as a string
func CreateCluster(kubeconfigPath string) (string, error) {
	clusterName := "tmp-mgmt"
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--kubeconfig", kubeconfigPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create kind cluster: %s: %w", stderr.String(), err)
	}

	// Fetch and return the kubeconfig content
	kubeconfig, err := exec.Command("kind", "get", "kubeconfig", "--name", "tmp-mgmt").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kind cluster kubeconfig: %w", err)
	}

	return string(kubeconfig), nil
}
