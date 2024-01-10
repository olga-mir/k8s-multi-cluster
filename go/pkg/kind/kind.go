package kind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateCluster(kubeconfigPath string) error {
	clusterName := "tmp-mgmt"
	// Create a temporary file for the Kind configuration
	kindConfig, err := os.CreateTemp("", "kind-bootstrap-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file for kind config: %w", err)
	}
	defer os.Remove(kindConfig.Name())

	// Write the Kind configuration to the temp file
	kindConfigContent := `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
`
	if _, err := kindConfig.WriteString(kindConfigContent); err != nil {
		return fmt.Errorf("failed to write kind config: %w", err)
	}

	// Ensure the file is written before using it
	if err := kindConfig.Sync(); err != nil {
		return fmt.Errorf("failed to sync kind config file: %w", err)
	}

	// Run the Kind command with the config file
	cmd := exec.Command("kind", "create", "cluster", "--name", clusterName, "--config", kindConfig.Name(), "--kubeconfig", kubeconfigPath)

	// Stream the output to os.Stdout and os.Stderr
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

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
	log := log.FromContext(context.Background())
	timeout := time.After(3 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for kind cluster to be ready")
		case <-ticker.C:
			if isClusterReady(kubeconfigPath, "kind-tmp-mgmt") { // TODO
				log.Info("Kind cluster is ready")
				return nil
			}
		}
	}
}

func isClusterReady(kubeconfigPath, contextName string) bool {
	log := log.FromContext(context.Background())
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Info("failed to load kubeconfig file: %v\n", err)
		return false
	}

	// Check if the context exists in the kubeconfig
	if _, exists := config.Contexts[contextName]; !exists {
		log.Info("context '%s' not found in kubeconfig\n", contextName)
		return false
	}

	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Ready\")].status}'")
	out, err := cmd.Output()
	if err != nil {
		log.Info("Error checking cluster status:", err)
		return false
	}
	return strings.Contains(string(out), "True")
}
