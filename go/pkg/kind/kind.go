package kind

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"
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
	timeout := time.After(3 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for kind cluster to be ready")
		case <-ticker.C:
			if isClusterReady(kubeconfigPath, "kind-tmp-mgmt") { // TODO
				fmt.Println("Kind cluster is ready")
				return nil
			}
		}
	}
}

func isClusterReady(kubeconfigPath, contextName string) bool {
	// Load the kubeconfig file
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Printf("failed to load kubeconfig file: %v\n", err)
		return false
	}

	// Check if the context exists in the kubeconfig
	if _, exists := config.Contexts[contextName]; !exists {
		log.Printf("context '%s' not found in kubeconfig\n", contextName)
		return false
	}

	cmd := exec.Command("kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "jsonpath='{.items[*].status.conditions[?(@.type==\"Ready\")].status}'")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("Error checking cluster status:", err)
		return false
	}
	return strings.Contains(string(out), "True")
}
