/*
Copyright 2023 The Rooster Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package worker

import (
	"errors"
	"strings"

	"rooster/pkg/utils"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
)

func newManager(kubernetesClientManager *utils.K8sClientManager) (m Manager, logger *zap.Logger) {
	m = Manager{}
	m.kcm = *kubernetesClientManager
	logger = kubernetesClientManager.Logger
	return
}

func deployResources(logger *zap.Logger, manifestPath, targetNamespace string, dryRun bool) (err error) {
	if manifestPath == "" {
		err = errors.New("missing manifest path")
		return
	}
	if exists := CheckDirectoryExistence(manifestPath); !exists {
		err = errors.New(manifestPath + ": No such file or directory")
		return
	}
	logger.Info("Deploying resources...")
	logger.Info("Resource path: " + manifestPath)
	dryRunStrategy := "none"
	if dryRun {
		dryRunStrategy = "client"
	}
	// Follow the given path. Deploy the yaml files in there
	cmd, err := utils.KubectlEmulator(targetNamespace, "apply", "-f", manifestPath, "--dry-run="+dryRunStrategy)
	if err != nil {
		logger.Info(cmd)
		return err
	}
	logger.Info("Resources were deployed")
	return
}

func makeCMName(project string) (newCmResource Resource) {
	// adjust the name of the confimap
	newCmName := strings.Join([]string{cmName, strings.ToLower(project)}, "-")
	newCmResource = *&cmResource
	newCmResource.Name = newCmName
	return
}

func convertToStreamlinerResource(nodes []core_v1.Node) (nodeResources []Resource) {
	r := Resource{}
	nodeResources = []Resource{}
	for _, n := range nodes {
		r.Name = n.Name
		r.Kind = nodeKind
		r.ApiVersion = apiVersionCoreV1
		nodeResources = append(nodeResources, r)
	}
	return
}
