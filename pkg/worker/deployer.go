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
	"fmt"
	"strconv"
	"strings"

	"rooster/pkg/utils"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	kcm utils.K8sClientManager
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type basicK8sConfiguration struct {
	ApiVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   basicK8sMetadata `json:"metadata"`
}

type basicK8sMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type RolloutOptions struct {
	Strategy             string           //Indicated rollout strategy
	ClusterName          string           // Current cluster name
	BatchSize            float64          // Number of nodes onto which to rollout
	Namespace            string           // Targeted namespace
	ManifestPath         string           // Path to the manifests to perform a canary release for
	Resources            []Resource       // Resources to rollout
	CanaryLabel          string           // Label to put on nodes to control the canary process
	TargetLabel          string           // Label identifying the nodes in the cluster
	NodesWithTargetlabel core_v1.NodeList // Nodes carrying the indicated target label
	RolloutNodes         []core_v1.Node   // Nodes onto which to rollout
	Increment            int              // Rollout increment over time. In percentage
	Canary               int              // Canary batch size. In percentage
	TestPackage          string           // Test package name
	TestBinary           string           // Test binary name
	UpdateIfExists       bool             // Update existing resources
	DryRun               bool
}

func ProceedToDeployment(kubernetesClientManager *utils.K8sClientManager, rolloutOpts RolloutOptions) (backupDirectory string, err error) {
	// Client settings
	m, logger := newManager(kubernetesClientManager)
	strategy := rolloutOpts.Strategy
	canaryLabel := rolloutOpts.CanaryLabel
	targetLabel := rolloutOpts.TargetLabel
	// Verify the canary & target label
	canaryLabelConfig := make(map[string]string)
	targetLabelConfig := make(map[string]string)
	targetLabelConfig["label"] = targetLabel
	targetLabelConfig["expectation"] = strconv.FormatBool(true)
	canaryLabelConfig["label"] = canaryLabel
	canaryLabelConfig["expectation"] = strconv.FormatBool(false)
	labelsConfig := [](map[string]string){targetLabelConfig, canaryLabelConfig}
	for _, labelConfig := range labelsConfig {
		logger.Info("Verifying label " + labelConfig["label"])
		if valid, err := m.verifyLabelValidity(labelConfig["label"], labelConfig["expectation"]); !valid || err != nil {
			return backupDirectory, err
		}
	}
	// Where to deploy it
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	targetNodes, err := m.getNodes(customOptions)
	if err != nil {
		return backupDirectory, err
	}
	if len(targetNodes.Items) == 0 {
		err = errors.New("No node carrying the target label was found.")
		return backupDirectory, err
	}
	// To add more options, let's create a copy of the given options and add them there.
	// Avoid modifying a given parameter
	opts := rolloutOpts
	opts.NodesWithTargetlabel = targetNodes
	switch strings.ToLower(strategy) {
	case "linear":
		backupDirectory, err = m.performLinearRollout(opts)
		if err != nil {
			return
		}
	case "canary":
		backupDirectory, err = m.performCanaryRollout(opts)
		if err != nil {
			return
		}
	default:
		err = errors.New("Unsupported rollout strategy")
		return backupDirectory, err
	}
	return backupDirectory, err
}

func newManager(kubernetesClientManager *utils.K8sClientManager) (m Manager, logger *zap.Logger) {
	m = Manager{}
	m.kcm = *kubernetesClientManager
	logger = kubernetesClientManager.Logger
	return
}

func RevertDeployment(kubernetesClientManager *utils.K8sClientManager, manifestPath string, targetLabel string, canaryLabel string, targetNamespace string) (output bool, err error) {
	dryRun := false
	m, logger := newManager(kubernetesClientManager)
	// the labels
	canaryLabelElements := strings.Split(canaryLabel, "=")
	canaryLabelKey := canaryLabelElements[0]
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	targetNodes, err := m.getNodes(customOptions)
	if err != nil {
		return output, err
	}
	if len(targetNodes.Items) == 0 {
		err = errors.New("No node carrying the target label was found.")
		return output, err
	}
	for _, targetNode := range targetNodes.Items {
		// Remove the canary label from all target nodes. Freeze the resources to 0 replica
		_, err := m.removeLabelFromNode(targetNode, canaryLabelKey, dryRun)
		if err != nil {
			return output, err
		}
	}
	// Get the new resources
	targetResources, err := ReadManifestFiles(logger, manifestPath, targetNamespace)
	if err != nil {
		return output, err
	}
	if len(targetResources) < 1 {
		err = errors.New("Target resources could not be determined. Aborting...")
		return output, err
	}
	// delete new resources & redeploy the old ones
	output, err = m.rollbackToPreviousSettings(targetResources, manifestPath, targetNamespace, dryRun)
	if err != nil {
		return output, err
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(targetResources)
	if err != nil {
		output = false
		return output, err
	}
	output = true
	logger.Info("The rollout failed. All resources were reverted")
	return output, err
}

func indicateNextAction() bool {
	var response string
	fmt.Println("At least one node was found carrying the indicated canary label.")
	fmt.Println("Would you like to continue? (y/n)")
	fmt.Scanln(&response)
	return strings.EqualFold(response, "Y")
}

func checkDaemonSetStatus(dsStatus map[string]interface{}) (ready bool, err error) {
	if dsStatus == nil {
		return false, errors.New("daemonSet status was not retrieved")
	}
	desiredNumberScheduled := dsStatus["desiredNumberScheduled"]
	numberReady := dsStatus["numberReady"]
	return desiredNumberScheduled == numberReady, nil
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

func DetermineNamespace(manifestIndicatedNamespace, optionIndicatedNamespace string) (finalNamespace string, err error) {
	if manifestIndicatedNamespace == "" && optionIndicatedNamespace == "" {
		finalNamespace = "default"
	} else if manifestIndicatedNamespace == "" && optionIndicatedNamespace != "" {
		finalNamespace = optionIndicatedNamespace
	} else if manifestIndicatedNamespace != "" && optionIndicatedNamespace == "" {
		finalNamespace = manifestIndicatedNamespace
	}
	switch finalNamespace != "" {
	case false:
		if optionIndicatedNamespace != manifestIndicatedNamespace {
			err = errors.New("!!! Namespace conflict detected !!!" + manifestIndicatedNamespace + " vs " + optionIndicatedNamespace)
		} else {
			finalNamespace = manifestIndicatedNamespace
		}
	}
	return
}

func defineLinearNodeScope(logger *zap.Logger, nodesToExclude core_v1.NodeList, batchSize int, nodesWithTargetLabel core_v1.NodeList) (nodeScope []core_v1.Node) {
	if len(nodesToExclude.Items) > batchSize && len(nodesToExclude.Items) < len(nodesWithTargetLabel.Items) {
		logger.Info("Increment control label's scope")
		logger.Info("Nodes with the control label will not be reverted")
		return
	}
	nodeScope = append(nodeScope, nodesToExclude.Items...)
	return
}

func defineCanaryNodeScope(nodes core_v1.NodeList, key string) (nodeScope []core_v1.Node) {
	// Ensure no node already has the canary label
	// If anyone does, return it
	for _, targetNode := range nodes.Items {
		labels := targetNode.Labels
		if labels[key] != "" {
			nodeScope = append(nodeScope, targetNode)
		}
	}
	return
}
