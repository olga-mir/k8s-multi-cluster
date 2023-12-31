package config

const (
	KindFluxVersion = "2.2.2"

	// https://github.blog/changelog/2022-01-18-githubs-ssh-host-keys-are-now-published-in-the-api/
	// curl -H "Accept: application/vnd.github.v3+json" -s https://api.github.com/meta | jq -r '.ssh_keys'
	// select the one that starts with "ecdsa-sha2-nistp256"
	GithubKnownHosts = "github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg="

	// TODO - make dynamic or env vars
	DefaultGithubUser   = "olga-mir"
	DefaultGithubRepo   = "k8s-multi-cluster"
	DefaultGithubBranch = "develop"
)

// KindClusterConfig provides a default configuration for a kind cluster.
func KindClusterConfig(clusterName string) ClusterConfig {
	return ClusterConfig{
		Name:              clusterName,
		Provider:          "kind",
		ManagementCluster: "",
		Flux: FluxConfig{
			KeyPath: "$HOME/.ssh/flux-github-key-2",
			Version: KindFluxVersion,
			GitHub: GitHubConfig{
				User:   DefaultGithubUser,
				Branch: DefaultGithubBranch,
				Repo:   DefaultGithubRepo,
			},
		},
	}
}
