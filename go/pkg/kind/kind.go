package kind

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func CreateCluster(kubeconfigPath string) error {
	clusterName := "tmp-mgmt"
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--kubeconfig", kubeconfigPath)

	// Stream the output to os.Stdout and os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create kind cluster: %w", err)
	}

	// Wait for the cluster to be ready
	if err := waitForClusterReady(kubeconfigPath); err != nil {
		return err
	}

	return nil
}

func waitForClusterReady(kubeconfigPath string) error {
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for kind cluster to be ready")
		case <-ticker.C:
			if isClusterReady(kubeconfigPath) {
				fmt.Println("Kind cluster is ready")
				return nil
			}
		}
	}
}

func isClusterReady(kubeconfigPath string) bool {
	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Ready\")].status}'")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("Error checking cluster status:", err)
		return false
	}
	return strings.Contains(string(out), "True")
}
