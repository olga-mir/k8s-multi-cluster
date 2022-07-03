# Config

This folder contains all required config for all clusters and components. The config is stored as a shell script that exports settings as envirionment variables. Deploy script will source relevant file before installing components on a given cluster.

Some tools, like cluster-api, accept config as an yaml [init file](https://cluster-api.sigs.k8s.io/clusterctl/configuration.html) (in addition to env var option). This is far better than env vars, but the drawbacks compared to unified env var config is that they are separate files and some settings could be re-used across different components. This may feel as abuse of a variable name, but it helps to keep settings consistent for each cluster, e.g. both CAPI and Cilium need pod CIDR value.
Another issue with init files is that it is hard to find the schema for them. CAPI documentation doesn't provide easy way to understand the file schema: https://cluster-api.sigs.k8s.io/clusterctl/configuration.html nor it is very clear from source code or the https://pkg.go.dev. Finding an env variable name on the other hand is much easier.

Previously this project was setup using CAPI config file: https://github.com/olga-mir/k8s-multi-cluster/blob/91ea9747b55833970fecd70c44d33ed938a5084a/mgmt-cluster/init-config-mgmt.yaml#L1-L7
However deploy script was polluted with the config data and cluster templates were hardcoded without option to re-generate them.
