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
	"reflect"

	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/**
* Goal: Will update the version of resources already found in the given cluster
* If resources are to be ignored, functions involving them (create, patch, delete) will do nothing
* Will:
* - Validate the given desired version
* - Backup the curent state of resources
* - Determine the nodes to target with the labeling & rollout
* - Patch the resources and remove the k8s label last-applied-config and apply the new given settings
* - Label the target nodes with the version of the resources they host
* - Update the config map Streamliner uses as a cache for the versions and node repartition
**/
func UpdateRollout(kubernetesClientManager *utils.K8sClientManager, opts RoosterOptions) (err error) {
	// Manager settings
	m, logger := newManager(kubernetesClientManager)
	controlLabel := opts.CanaryLabel
	resources := opts.Resources
	rollingBatchPercentage := opts.Increment
	clusterID := opts.ClusterID
	projectOptions := opts.ProjectOpts
	dryRun := opts.DryRun
	projectName := projectOptions.Project
	desiredVersion := projectOptions.DesiredVersion
	manifestPath := opts.ManifestPath
	namespace := opts.Namespace
	ignoreResources := opts.IgnoreResources
	action := opts.Action
	targetLabel := opts.TargetLabel
	cmResourcePrj := makeCMName(projectName)
	// get the cm and extract its content
	cmdata, err := m.retrieveConfigMapContent(cmResourcePrj)
	if err != nil {
		return
	}
	// Get the current version
	currentVersion, err := m.getCurrentVersion(projectName, cmdata)
	if err != nil {
		return
	}
	// Validate the desired version
	// The desired version cannot be the current one. Otherwise the operation should be a ROLLOUT
	if currentVersion == desiredVersion {
		return fmt.Errorf("version disparity required. Current: %v - Desired: %v", currentVersion, desiredVersion)
	}
	// When updating:
	// - having previous ACTIVE versions: NOT ALLOWED
	// - having a current version not fully rolled out: NOT ALLOWED
	if err := m.CheckPreviousVersions(cmdata, projectName, action); err != nil {
		return err
	}
	if err := m.CheckCurrentVersion(cmdata, projectName, targetLabel, action); err != nil {
		return err
	}
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = controlLabel
	unstructNodes, err := m.getNodes(customOptions)
	if err != nil {
		return err
	}
	targetNodes := utils.MakeNodeList(unstructNodes)
	if reflect.DeepEqual(targetNodes, core_v1.NodeList{}) {
		// A node is ready for an update if it has been deployed on and it carries the canary/control label
		return errors.New("no nodes ready for an update was found")
	}

	// Define the batchSize
	patchTargets, rollingBatch := m.calBatchSize(targetNodes, rollingBatchPercentage)
	if err = utils.ValidateBatchSize(int(rollingBatch)); err != nil {
		return
	}
	// Back up existing resources
	projectOptions.CurrVersion = currentVersion
	logger.Info("Backing up current version before updating...")
	_, err = backupResources(logger, resources, clusterID, projectOptions, ignoreResources)
	if err != nil {
		return
	}
	// Patch the resources
	err = m.RemoveLastAppliedAnnotation(resources, dryRun)
	if err != nil {
		return
	}
	// apply the new config
	err = m.applyRolloutAction("", manifestPath, namespace, resources, ignoreResources, dryRun)
	if err != nil {
		return
	}
	// Restart pods slowly
	err = m.incrementalNodePatch(patchTargets, controlLabel, dryRun, false)
	if err != nil {
		return
	}
	// Patch the nodes with the version label
	nodeR := convertToStreamlinerResource(patchTargets)
	err = m.applyVersionPatch(nodeR, projectOptions, dryRun)
	if err != nil {
		return
	}
	cmNewData := utils.ComposeConfigMapData(action, projectName, desiredVersion, patchTargets, cmdata)
	// Patch the config map
	_, err = m.patchConfigmap(action, projectOptions, cmNewData, dryRun)
	return err
}
