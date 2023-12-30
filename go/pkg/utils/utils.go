package utils

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func RepoRoot() string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}

	repoRoot := strings.TrimSpace(out.String())

	return repoRoot
}

func GetCurrentContextName(config *rest.Config, kubeconfigPath string) (string, error) {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig file: %w", err)
	}

	currentContext := kubeconfig.CurrentContext
	if currentContext == "" {
		return "", fmt.Errorf("current context is not set in kubeconfig")
	}

	return currentContext, nil
}
