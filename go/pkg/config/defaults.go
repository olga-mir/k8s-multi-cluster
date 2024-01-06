package config

import (
	"os"
)

const (
	KindFluxVersion = "2.2.2"
	FluxNamespace   = "flux-system"

	// https://github.blog/changelog/2022-01-18-githubs-ssh-host-keys-are-now-published-in-the-api/
	// curl -H "Accept: application/vnd.github.v3+json" -s https://api.github.com/meta | jq -r '.ssh_keys'
	// select the one that starts with "ecdsa-sha2-nistp256"
	GithubKnownHosts = "github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg="

	// Cluster names and contexts
	// CURRENT   NAME                              CLUSTER         AUTHINFO             NAMESPACE
	//           cluster-mgmt-admin@cluster-mgmt   cluster-mgmt    cluster-mgmt-admin
	//           kind-tmp-mgmt                     kind-tmp-mgmt   kind-tmp-mgmt
	DefaultKindClusterName    = "tmp-mgmt"
	DefaultCAPIClusterNameTpl = "cluster-{{.Name}}"
	DefaultCAPIContextNameTpl = "cluster-{{.Name}}-admin@{{.Name}}"
)

var ProjectNamespaces = []string{FluxNamespace, "caaph-system"} // TODO - flux namespace can be part of config (dynamic)

// KindClusterConfig provides a default configuration for a kind cluster.
func KindClusterConfig(clusterName string) ClusterConfig {
	fluxcdKey := os.Getenv("FLUXCD_KEY_PATH")
	if fluxcdKey == "" {
		// TODO - for now all clusters will share the same key.
		// TODO - change function so that we don't have to panic here
		panic("FLUXCD_KEY_PATH environment variable is not set")
	}

	return ClusterConfig{
		Name:              clusterName,
		Provider:          "kind",
		ManagementCluster: "",
		Flux: FluxConfig{
			KeyPath:   fluxcdKey,
			Version:   KindFluxVersion,
			Namespace: FluxNamespace,
		},
	}
}
