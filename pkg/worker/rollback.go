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

	"rooster/pkg/config"
	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/**
* Goal: Based off a given set of options, prepare a rollback of resources in the cluster
* Will:
* - Roll resources back to a specific version, if it is indicated
* - Clean the cluster of indicated resources, if no version is specified
**/
func RevertDeployment(kubernetesClientManager *utils.K8sClientManager, opts RoosterOptions) (err error) {
	// Manager settings
	m, logger := newManager(kubernetesClientManager)
	targetVersion := opts.ProjectOpts.DesiredVersion
	ignoreResources := opts.IgnoreResources
	resources := opts.Resources
	switch targetVersion != "" {
	case true:
		// GOAL: rollback to specific version
		// Re-label nodes
		// Update the config map
		err = m.revertToVersion(opts)
	case false:
		// GOAL: Rollback.
		// Un-label all nodes
		// Update the config map
		err = m.cleanResources(opts)
		if ignoreResources || err != nil {
			return
		}
		// delete the resources that are deployed in the cluster
		deleteOpts := utils.MakeDeleteOptions(opts.DryRun)
		dynamicOpts := utils.DynamicQueryOptions{DeleteOptions: deleteOpts}
		_, err = m.queryResources(utils.Delete, resources, dynamicOpts)
		if err != nil {
			return
		}
	}
	if err != nil {
		return
	}
	logger.Info("Rollback complete.")
	return
}

/**
* Update the project's config map
* Unlabel the nodes
**/
func (m *Manager) cleanResources(opts RoosterOptions) (err error) {
	action := opts.Action
	controlLabel := opts.CanaryLabel
	projectOpts := opts.ProjectOpts
	dryRun := opts.DryRun
	// the labels
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = controlLabel
	project := projectOpts.Project
	desiredVersion := projectOpts.DesiredVersion
	cmResourcePrj := makeCMName(project)
	versionKey, _ := utils.MakeVersionLabel(STREAMLINER_LBL_PREFIX, project, "")
	// get the cm and extract its content
	cmdata, err := m.retrieveConfigMapContent(cmResourcePrj)
	if err != nil {
		return
	}
	// get nodes tagged with the control label
	targetNodes, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	if len(targetNodes) == 0 {
		return errors.New("no node carrying the control/canary label was found")
	}
	nodeResources := utils.MakeNodeList(targetNodes)
	finalNodes := nodeResources.Items
	// TODO: reverse the list of final nodes, in order to unlabel from the oldest to the most recent node targeted by the rollout
	opts.RolloutNodes = finalNodes
	// Split the canary label
	data := utils.SplitLabel([]string{controlLabel, versionKey + "="})
	// Patch the other nodes, to remove the control and version labels
	op := "remove"
	_, err = m.patchNodes(opts, op, data)
	if err != nil {
		return
	}
	cmNewData := utils.ComposeConfigMapData(action, project, desiredVersion, finalNodes, cmdata)
	// patch the config map
	_, err = m.patchConfigmap(action, projectOpts, cmNewData, dryRun)
	return
}

/**
* Reverts resources to the indicated version if they are not to be ignored, and label nodes where the daemonset's pods are running, with the proper version.
* If resources are to be ignored, functions involving them (create, patch, delete) will do nothing.
**/
func (m *Manager) revertToVersion(opts RoosterOptions) (err error) {
	ns := opts.Namespace
	clusterID := opts.ClusterID
	dryRun := opts.DryRun
	projectOpts := opts.ProjectOpts
	projectName := projectOpts.Project
	desiredVrs := projectOpts.DesiredVersion
	controlLabel := opts.CanaryLabel
	logger := m.kcm.Logger
	cmResourcePrj := makeCMName(projectName)
	ignoreResources := opts.IgnoreResources
	action := opts.Action
	// get the cm and extract its content
	cmdata, err := m.retrieveConfigMapContent(cmResourcePrj)
	if err != nil {
		return
	}
	// get current resource version
	currentVersion, err := m.getCurrentVersion(projectName, cmdata)
	if err != nil {
		return
	}
	if currentVersion == desiredVrs {
		return errors.New("cannot rollback to a version that is current")
	}
	// When rolling back to a version:
	// - having previous ACTIVE versions: ALLOWED
	// - having a current version not fully rolled out: ALLOWED

	projectOpts.CurrVersion = currentVersion
	// find the backup with the desired version
	dirName, err := getVersionBackupPath(projectOpts, clusterID)
	if err != nil {
		return
	}
	// get nodes tagged with the current version
	nodes, err := m.getMarkedNodes(projectName, currentVersion)
	targetNodes := utils.ConvertToNodeList(nodes)
	if err != nil {
		return
	}
	if len(targetNodes.Items) == 0 {
		return errors.New("no node carrying the control/canary label was found")
	}
	// roll all nodes back to the indicated version
	rollbackTargetNodes := targetNodes.Items
	// Get the resources
	resources, err := ReadManifestFiles(logger, dirName, ns)
	if err != nil {
		return
	}
	// in case the ns is empty because the manifest path is not indicated. (no need for rollback to version ops)
	ns = resources[0].Namespace
	// only used when rolling back all instances.
	// resources will be deleted and re-created with the indicated configuration
	err = m.applyRolloutAction("apply-all", dirName, ns, resources, false, dryRun)
	if err != nil {
		return
	}

	// Restart pods slowly
	err = m.incrementalNodePatch(rollbackTargetNodes, controlLabel, dryRun, false)
	if err != nil {
		return
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(ignoreResources, resources)
	if err != nil {
		return
	}
	nodeR := convertToStreamlinerResource(rollbackTargetNodes)
	err = m.applyVersionPatch(nodeR, projectOpts, dryRun)
	if err != nil {
		return
	}
	// get nodes with version v
	d := cmdata.Data
	for _, pii := range d.Info {
		if pii.Version != desiredVrs {
			continue
		}
		for _, n := range pii.Nodes {
			if n == "" {
				continue
			}
			un := core_v1.Node{}
			un.SetName(n)
			rollbackTargetNodes = append(rollbackTargetNodes, un)
		}
	}
	cmNewData := utils.ComposeConfigMapData(action, projectName, desiredVrs, rollbackTargetNodes, cmdata)
	// patch the config map
	_, err = m.patchConfigmap(action, projectOpts, cmNewData, dryRun)
	return
}

func getVersionBackupPath(prjOpts ProjectOptions, clusterName string) (dirName string, err error) {
	projectName := prjOpts.Project
	targetVers := prjOpts.DesiredVersion
	backupDir := config.Env.BackupDirectory
	// Find the backup folder
	nameComponents := []string{backupDir, clusterName, projectName, targetVers}
	dirName = strings.Join(nameComponents, "/")
	if found := CheckDirectoryExistence(dirName); !found {
		err = errors.New("Could not find repository " + dirName)
		return
	}
	return
}
