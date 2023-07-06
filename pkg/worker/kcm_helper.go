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
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (m *Manager) patchNodes(opts RolloutOptions) (bool, error) {
	logger := m.kcm.Logger
	ctx := context.TODO()
	strategy := opts.Strategy
	rolloutNodes := opts.RolloutNodes
	nodesWithTargetLabel := opts.NodesWithTargetlabel
	controlLabel := opts.CanaryLabel
	batchSize := opts.BatchSize
	dryRun := opts.DryRun
	// Split the canary label
	cL := strings.Split(controlLabel, "=")
	controlLabelKey := cL[0]
	controlLabelVal := cL[1]
	// get the nodes with the control/canary label
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = controlLabel
	nodes, err := m.getNodes(customOptions)
	if err != nil {
		return false, err
	}
	logger.Sugar().Infof("Batch size---: %g", batchSize)
	nodesToRevert := []core_v1.Node{}
	switch strings.ToLower(strategy) {
	case "linear":
		nodesToRevert = defineLinearNodeScope(logger, nodes, int(batchSize), nodesWithTargetLabel)
	case "canary":
		nodesToRevert = defineCanaryNodeScope(nodes, controlLabelKey)
	default:
		return false, errors.New("Unsupported rollout strategy")
	}
	// Case 1: More nodes than specified by the canary batch size have the canary label already. This may be a resource update situation
	// E.g: batch size=3. nodes with the canary label in the cluster at this point: 4 or more
	if len(nodesToRevert) > int(batchSize) {
		// iterate in reverse to start with the last nodes
		for i := len(nodesToRevert) - 1; i >= 0; i-- {
			// Remove the canary label from the nodes part of the 2nd batch
			if i == int(batchSize)-1 {
				logger.Info("Reached the batch size limit")
				break
			}
			logger.Info("Removing canary label from " + nodesToRevert[i].Name)
			_, err := m.removeLabelFromNode(nodesToRevert[i], controlLabelKey, dryRun)
			if err != nil {
				return false, err
			}
		}
		waitForResources(10 * time.Second)
		return true, nil
	}
	// Case 2: Either no node has the canary label yet, or less nodes specified by the canary batch size do
	// E.g: batch size=3. nodes with the canary label in the cluster at this point: 1, or 0
	customPatchOptions := meta_v1.PatchOptions{}
	if dryRun {
		customPatchOptions.DryRun = append(customPatchOptions.DryRun, "All")
	}
	p := types.JSONPatchType
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/metadata/labels/" + controlLabelKey,
		Value: controlLabelVal,
	}}
	data, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	for _, rolloutNode := range rolloutNodes {
		// Label the nodes with the control/canary label
		logger.Info("Node to patch: " + rolloutNode.Name)
		_, err := m.kcm.Client.CoreV1().Nodes().Patch(ctx, rolloutNode.Name, p, data, customPatchOptions)
		if err != nil {
			return false, err
		}
	}
	logger.Info("Patching complete")
	return true, nil
}

func (m *Manager) determineRolloutAction(opts RolloutOptions, missingResources []Resource) (rolloutAction string) {
	updateIfExists := opts.UpdateIfExists
	if updateIfExists {
		rolloutAction = "apply-all"
	} else if len(missingResources) != 0 && !updateIfExists {
		rolloutAction = "apply-selective"
	}
	return
}

func (m *Manager) applyRolloutAction(action string, manifestPath string, resources []Resource, namespace string, dryRun bool) (err error) {
	logger := m.kcm.Logger
	logger.Info("ACTION: " + action)
	if strings.EqualFold(action, "apply-all") {
		// make sure the latest version will be deployed by removing the old ones first
		// 3 for the verb DELETE
		_, err = m.queryResources(3, resources, dryRun)
		if err != nil {
			return err
		}
		logger.Info("Resources deletion is now complete.")
	}
	err = deployResources(logger, manifestPath, namespace, dryRun)
	if err != nil {
		return err
	}
	return
}

func (m *Manager) getMissingResources(targetResources []Resource) (missingResources []Resource, err error) {
	missingResources = []Resource{}
	for _, currRes := range targetResources {
		_, err := m.getResource(currRes.Kind, currRes.Name, currRes.Namespace)
		if k8s_errors.IsNotFound(err) {
			rs := Resource{
				Kind:      currRes.Kind,
				Name:      currRes.Name,
				Namespace: currRes.Namespace,
				Manifest:  currRes.Manifest,
			}
			missingResources = append(missingResources, rs)
			continue
		}
		if err != nil {
			return missingResources, err
		}
	}
	return
}

func waitForResources(duration time.Duration) {
	time.Sleep(duration)
}

func (m *Manager) verifyResourcesStatus(targetResources []Resource) (err error) {
	resourceReport, err := m.areResourcesReady(targetResources)
	if err != nil {
		return
	}
	if len(resourceReport) == 0 {
		// TODO_1b the next line won't be needed anymore once areResourcesReady is improved.
		// A more explicit error should be returned
		err = errors.New("Resources readiness could not be defined")
		return
	}
	for _, rs := range resourceReport {
		if !rs.Ready {
			err = errors.New("Issues encountered with the " + rs.Kind + " " + rs.Name)
			return err
		}
	}
	return
}

func (m *Manager) removeLabelFromNode(targetNode core_v1.Node, labelKey string, dryRun bool) (done bool, err error) {
	dryRunStrategy := "none"
	if dryRun {
		dryRunStrategy = "client"
	}
	// Get all the nodes matching the target label
	cmd, err := utils.KubectlEmulator("", "label node "+targetNode.Name+" "+labelKey+"-"+" --dry-run="+dryRunStrategy)
	if err != nil {
		m.kcm.Logger.Info(cmd)
		return false, err
	}
	return true, nil
}

func (m *Manager) rollbackToPreviousSettings(targetResources []Resource, pathToBackupDirectory string, targetNamespace string, dryRun bool) (bool, error) {
	logger := m.kcm.Logger
	logger.Info("----Rolling back to the previous settings------")
	// delete the resources that are deployed in the cluster
	_, err := m.queryResources(3, targetResources, false)
	if err != nil {
		return false, err
	}
	logger.Info("Resources deletion is now complete.")
	// deploy the resources that had their config backed up before
	err = deployResources(logger, pathToBackupDirectory, targetNamespace, dryRun)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (m *Manager) areResourcesReady(targetResources []Resource) (resourcesStatus []Resource, err error) {
	logger := m.kcm.Logger
	logger.Info("Waiting for resources to be ready...")
	waitForResources(20 * time.Second)
	resourcesStatus = []Resource{}
	rs := Resource{}
	// 0 for the verb GET
	resources, err := m.queryResources(0, targetResources, false)
	if err != nil {
		return
	}
	for _, kubernetesResource := range resources {
		k8sObject := kubernetesResource.Object
		kind := k8sObject["kind"].(string)
		name := k8sObject["metadata"].(map[string]interface{})["name"].(string)
		namespace := k8sObject["metadata"].(map[string]interface{})["namespace"].(string)
		status := make(map[string]interface{})
		logger.Info("Found " + kind + " " + name)
		if kind == "DaemonSet" {
			status = k8sObject["status"].(map[string]interface{})
		}
		rs.Name = name
		rs.Kind = kind
		rs.Namespace = namespace
		ready, err := m.checkResourceStatus(kind, status, rs)
		if err != nil {
			return resourcesStatus, err
		}
		rs.Ready = ready
		resourcesStatus = append(resourcesStatus, rs)
	}
	return resourcesStatus, err
}

func (m *Manager) checkResourceStatus(kind string, status map[string]interface{}, rs Resource) (result bool, err error) {
	switch kind {
	case "DaemonSet":
		ready, err := checkDaemonSetStatus(status)
		if err != nil {
			return ready, err
		}
	default:
		_, err := m.getResource(rs.Kind, rs.Name, rs.Namespace)
		if err != nil {
			return false, err
		}
	}
	return true, err
}

func (m *Manager) getNodes(customOptions meta_v1.ListOptions) (targetNodes core_v1.NodeList, err error) {
	ctx := context.TODO()
	// Get all the nodes with the indicated label.
	nodeList, err := m.kcm.Client.CoreV1().Nodes().List(ctx, customOptions)
	targetNodes = *nodeList
	return
}

func (m *Manager) verifyLabelValidity(label string, shouldExist string) (output bool, err error) {
	/*
	* Nodes with the target label must exist
	* If no node is found, bail
	* Nodes with the canary labels should not exist
	* If some are found, the choice to continue the rollout is given to the user
	*
	 */
	// Review nodes labels
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = label
	nodes, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	expectedStatus, _ := strconv.ParseBool(shouldExist)
	// Nodes with the canary label were found, but it may be okay to carry on
	if !expectedStatus && len(nodes.Items) > 0 {
		decision := indicateNextAction()
		if !decision {
			err = errors.New("User cancelled action")
		}
		return decision, err
	}
	// No nodes carrying the target label were found. Abort!
	if expectedStatus && len(nodes.Items) == 0 {
		err = errors.New("No node carrying the target label was found.")
		return
	}
	output = true
	return
}

func (m *Manager) performRollout(rolloutOpts RolloutOptions) (backupDirectory string, err error) {
	dryRun := rolloutOpts.DryRun
	manifestPath := rolloutOpts.ManifestPath
	namespace := rolloutOpts.Namespace
	resourcesToDeploy := rolloutOpts.Resources
	logger := m.kcm.Logger
	logger.Info("Patching nodes...")
	_, err = m.patchNodes(rolloutOpts)
	if err != nil {
		return backupDirectory, err
	}
	logger.Info("Backing up resources...")
	// Back up existing resources
	backupDirectory, err = backupResources(logger, rolloutOpts.Resources, rolloutOpts.ClusterName)
	if err != nil {
		return backupDirectory, err
	}
	// Check all the resources. See if they are in the cluster
	missingResources, err := m.getMissingResources(rolloutOpts.Resources)
	if err != nil {
		return backupDirectory, err
	}
	logger.Sugar().Infof("Missing resource: %t", len(missingResources) > 0)
	rolloutAction := m.determineRolloutAction(rolloutOpts, missingResources)
	switch rolloutAction {
	case "apply-all":
		err = m.applyRolloutAction(rolloutAction, manifestPath, resourcesToDeploy, namespace, dryRun)
	case "apply-selective":
		// Get manifest of missing resource
		// Only create missing resources
		for _, rs := range missingResources {
			myRs := []Resource{rs}
			logger.Info("Creating missing " + rs.Kind + " " + rs.Name + ", in namespace: " + rs.Namespace)
			err = m.applyRolloutAction(rolloutAction, rs.Manifest, myRs, rs.Namespace, dryRun)
		}
	}
	if err != nil {
		return backupDirectory, err
	}
	if rolloutOpts.DryRun {
		logger.Info("Dry run operation. No errors encountered")
		return
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(rolloutOpts.Resources)
	if err != nil {
		return backupDirectory, err
	}
	// Run the tests
	err = runTests(logger, rolloutOpts.TestPackage, rolloutOpts.TestBinary)
	if err != nil {
		logger.Warn("Tests have failed.")
		return backupDirectory, err
	}
	return backupDirectory, err
}

func (m *Manager) defineBatchSize(nodeList core_v1.NodeList, canaryOrIncrement int) (nodes []core_v1.Node, batchSize float64) {
	logger := m.kcm.Logger
	// Set the batch size
	logger.Info("Defining batch size...")
	if canaryOrIncrement > 100 {
		logger.Warn("Batch size cannot be defined. Invalid canary/increment")
		return
	}
	switch len(nodeList.Items) {
	case 0:
		batchSize = 0
		nodes = nil
	case 1:
		batchSize = 1
		nodes = nodeList.Items
	default:
		batchSize = math.Round(float64(len(nodeList.Items)*canaryOrIncrement) / 100)
		nodes = nodeList.Items[:int(batchSize)]
		logger.Sugar().Infof("Targeted nodes count: %g/%d", batchSize, len(nodeList.Items))
	}
	return
}
