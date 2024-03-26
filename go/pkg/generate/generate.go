package generate

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/config"
	"github.com/olga-mir/k8s-multi-cluster/go/pkg/utils"
)

func Generate(log logr.Logger, cfg *config.Config) error {

	templateDir := utils.RepoRoot() + "/templates-go"

	for _, cluster := range cfg.Clusters {
		clusterPath := utils.RepoRoot() + "/clusters/cluster-mgmt/" + cluster.Name
		err := os.MkdirAll(clusterPath, os.ModePerm)
		if err != nil {
			return err
		}

		// generate platform.yaml - this file speicifies what platform components should be installed
		// and what version of them
		utils.RenderTemplateToFile(templateDir+"/platform.gotmpl", clusterPath+"platform.yaml", cluster)
	}
	return nil
}
