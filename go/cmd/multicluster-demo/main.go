package main

import (
	"multicluster-demo/pkg/builder"
	"multicluster-demo/pkg/config"
	"multicluster-demo/pkg/runner"

	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/k8sclient"
	"github.com/spf13/cobra"
)

type KubernetesClients struct {
	TempManagementCluster *k8sclient.ClusterClient            // Temporary management cluster (kind)
	PermManagementCluster *k8sclient.ClusterClient            // Permanent management cluster
	WorkloadClusters      map[string]*k8sclient.ClusterClient // Map of workload clusters
}

// GetKubernetesClients creates KubernetesClients struct with clients for each workload cluster.
func main() {
	var rootCmd = &cobra.Command{Use: "multicluster-demo"}

	var cfgFile string
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")

	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Build all clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return err
			}
			return builder.BuildClusters(cfg)
		},
	}

	var runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run scenarios",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(cfgFile)
			if err != nil {
				return err
			}
			return runner.RunScenarios(cfg)
		},
	}

	rootCmd.AddCommand(buildCmd, runCmd)
	rootCmd.Execute()

	/*
		logger.SetLogger(zap.New(zap.UseDevMode(true)))

		const kindContext = "kind-tmp-mgmt" // TODO

		kubeconfigPath := os.Getenv("K8S_MULTI_KUBECONFIG")
		if kubeconfigPath == "" {
			log.Fatalf("K8S_MULTI_KUBECONFIG environment variable is not set")
		}

		// Create a kind cluster and get its kubeconfig
		log.Printf("Create `kind` cluster")
		err := kind.CreateCluster(kubeconfigPath)
		if err != nil {
			log.Fatalf("Error creating kind cluster: %v", err)
		}

		kindConfig, err := k8sclient.GetKubernetesClient(kubeconfigPath, kindContext)
		if err != nil {
			log.Fatalf("Failed to create Kubernetes client for kind cluster: %v", err)
		}

		kubeClients := &KubernetesClients{
			TempManagementCluster: kindConfig,
			PermManagementCluster: nil,                                       // Initialize to nil to indicate that the permanent cluster has not been created yet.
			WorkloadClusters:      make(map[string]*k8sclient.ClusterClient), // Initialize the map to an empty map
		}

		if err := capi.InitClusterAPI(kubeClients.TempManagementCluster.Config, kubeconfigPath); err != nil {
			log.Fatalf("Failed to initialize Cluster API: %v", err)
		}

		// Install FluxCD on the kind cluster
		if err := fluxcd.InstallFluxCD(kubeClients.TempManagementCluster.Config, kubeClients.TempManagementCluster.Clientset); err != nil {
			log.Fatalf("Error installing FluxCD: %v", err)
		}

		// Pivot to the permanent management cluster
		// if err := capi.PivotCluster("path/to/temp/kubeconfig", "path/to/permanent/kubeconfig"); err != nil {
		//	log.Fatalf("Error pivoting to permanent cluster: %v", err)
		//}
	*/
}
