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
	DefaultKindClusterCtxName = "kind-tmp-mgmt"
	DefaultCAPIClusterNameTpl = "cluster-{{.Name}}"
	DefaultCAPIContextNameTpl = "cluster-{{.Name}}-admin@cluster-{{.Name}}"
)

var ProjectNamespaces = []string{FluxNamespace, "caaph-system"} // TODO - flux namespace can be part of config (dynamic)

// KindClusterConfig provides a default configuration for a kind cluster.
func GetKindClusterConfig(clusterName string) ClusterConfig {
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

// TODO - this is a terrible terrible name. "ClusterConfig" here comes from the
// config of this app, but this misleading because CAPI client also has a config file
// TODO - also we might want to store the clusterName and context name inside this config
// instead of keeping it in stray variables and passing around.
func GetCapiClusterConfig(clusterName string) ClusterConfig {
	return ClusterConfig{
		Name:              clusterName,
		Provider:          "capi", // iirc this is to distinguish if we are using CAPI or CrossPlane
		ManagementCluster: "",
		Flux: FluxConfig{
			Version:   KindFluxVersion, // TODO - don't care now, can be the same version
			Namespace: FluxNamespace,
		},
	}
}
