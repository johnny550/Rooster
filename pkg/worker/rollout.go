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

	"rooster/pkg/config"
	"rooster/pkg/utils"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/**
* Goal: Based off a given strategy, prepare to rollout resources onto a cluster
* Will:
* - Validate the given desired version
* - Get the nodes with the target label
* - Depending on the given strategy (canary || linear), deploy resources
* - Add the name of the nodes that have a pod of the daemonset running. Nodes are sperated by version of the resources the host
* - Create/Modify the config map Streamliner uses as a cache for the versions and node repartition
**/
func ProceedToDeployment(kubernetesClientManager *utils.K8sClientManager, rolloutOpts RoosterOptions) (err error) {
	// Manager settings
	m, logger := newManager(kubernetesClientManager)
	defaultNs := config.Env.DefaultNamespace
	action := rolloutOpts.Action
	strategy := rolloutOpts.Strategy
	canaryLabel := rolloutOpts.CanaryLabel
	targetLabel := rolloutOpts.TargetLabel
	projectOpts := rolloutOpts.ProjectOpts
	project := projectOpts.Project
	version := projectOpts.DesiredVersion
	dryRun := rolloutOpts.DryRun
	// adjust the name of the connfimap
	cmResourcePrj := makeCMName(project)
	newCmName := cmResourcePrj.Name
	_, prjVersionLabel := utils.MakeVersionLabel(STREAMLINER_LBL_PREFIX, project, version)
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
			return err
		}
	}
	// get the cm and extract its content
	cmdata, err := m.retrieveConfigMapContent(cmResourcePrj)
	cmIsNotFound := k8s_errors.IsNotFound(err)
	if err != nil && !cmIsNotFound {
		return
	}
	// Validate the desired version
	// The desired version must be the current one. Otherwise the operation should be an UPDATE
	currentVersion, err := m.getCurrentVersion(project, cmdata)
	if err != nil {
		return
	}
	if currentVersion != version && currentVersion != "" {
		return fmt.Errorf("version disparity detected. Current: %v - Desired: %v", currentVersion, version)
	}
	// Where to deploy resources
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	targetNodes, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	if len(targetNodes) == 0 {
		return errors.New("no node carrying the target label was found")
	}
	nodes := utils.MakeNodeList(targetNodes)
	// populate params
	rolloutOpts.NodesWithTargetlabel = nodes
	rolloutOpts.ProjectOpts.CurrVersion = currentVersion
	switch strings.ToLower(strategy) {
	case "linear":
		_, err = m.performLinearRollout(rolloutOpts)
		if err != nil {
			return
		}
	case "canary":
		_, err = m.performCanaryRollout(rolloutOpts)
		if err != nil {
			return
		}
	default:
		return errors.New("unsupported rollout strategy")
	}
	// Get the nodes that have been deployed onto. They are marked with the version label, by performRollout()
	customOptions.LabelSelector = prjVersionLabel
	assignedNodes, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	nodes = utils.MakeNodeList(assignedNodes)
	// Create the config map
	cmLabels := utils.ComposeConfigMapLabels()
	data := utils.ComposeConfigMapData(action, project, version, nodes.Items, cmdata)
	if cmIsNotFound {
		cm := utils.ComposeConfigMap(defaultNs, newCmName, cmLabels, data)
		_, err = m.createConfigMap(defaultNs, *cm, dryRun)
		return
	}
	_, err = m.patchConfigmap(action, projectOpts, data, dryRun)
	return
}
