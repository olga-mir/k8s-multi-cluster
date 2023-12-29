package kind

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func CreateCluster(kubeconfigPath string) (string, error) {
	clusterName := "tmp-mgmt"
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--kubeconfig", kubeconfigPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Println(stdout.String()) // Print the output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create kind cluster: %s: %w", stderr.String(), err)
	}

	fmt.Println(stdout.String())

	// Wait for the cluster to be ready
	if err := waitForClusterReady(kubeconfigPath); err != nil {
		log.Printf("HHH Failed to wait for cluster to be ready: %v\n", err)
		return "", err
	}

	// Fetch and return the kubeconfig content
	kubeconfig, err := exec.Command("kind", "get", "kubeconfig", "--name", clusterName).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kind cluster kubeconfig: %w", err)
	}

	return string(kubeconfig), nil
}

func waitForClusterReady(kubeconfigPath string) error {
	for i := 0; i < 60; i++ { // Wait up to 5 minutes
		log.Printf("Waiting for kind cluster to be ready %d\n", i)
		cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "--no-headers")
		var out bytes.Buffer
		cmd.Stdout = &out

		if err := cmd.Run(); err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		if strings.Contains(out.String(), "Ready") {
			return nil
		}

		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for kind cluster to be ready")
}
