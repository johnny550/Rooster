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
	"strings"

	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
)

/**
* Goal: Perform a canary rollout
* Ref: https://confluence.rakuten-it.com/confluence/display/CCOG/v1.1.0+-+Streamliner#v1.1.0Streamliner-Canaryrollout
* Will:
* - Determine the nodes to target first (canary)
* - Perform a rollout
* - Determine the rest of the nodes to target (all others relevant to the combination of target + canary label)
* - Perform a rollout on the remaining nodes
* - Label all the target nodes with the version of the resources they host
**/
func (m *Manager) performCanaryRollout(opts RoosterOptions) (backupDirectory string, err error) {
	// Get params
	logger := m.kcm.Logger
	canary := opts.Canary
	dryRun := opts.DryRun
	canaryLabel := opts.CanaryLabel
	projectOptions := opts.ProjectOpts
	// new targets may include nodes that have don't have the target resources (pods) running on them
	newTargets, err := m.DefineTargetNodes(opts)
	if err != nil {
		return
	}
	if len(newTargets.Items) == 0 {
		err = utils.MakeRollloutLimitErr()
		return
	}
	// Define batch size
	rolloutNodes, batchSize := m.calBatchSize(newTargets, canary)
	if err = utils.ValidateBatchSize(int(batchSize)); err != nil {
		return
	}
	// Verify the batch nodes
	err = utils.MatchBatch(newTargets.Items, rolloutNodes)
	if err != nil {
		return
	}
	// BATCH ROLLOUT
	opts.RolloutNodes = rolloutNodes
	opts.BatchSize = batchSize
	backupDirectory, err = m.performRollout(opts)
	if err != nil {
		return backupDirectory, err
	}
	// double verification, even though this whole func is only exectued if the strategy is canary.
	if !strings.EqualFold(opts.Strategy, "canary") || opts.DryRun {
		return backupDirectory, err
	}
	// Update the list of rollout nodes
	otherNodes := defineRestOfNodes(newTargets, len(rolloutNodes))
	opts.RolloutNodes = otherNodes
	// Update the batch size
	updatedBatchSize := len(newTargets.Items) - int(batchSize)
	opts.BatchSize = float64(updatedBatchSize)
	// Complete the rollout
	logger.Info("Patching remaining nodes...")
	err = m.incrementalNodePatch(otherNodes, canaryLabel, dryRun, true)
	if err != nil {
		return backupDirectory, err
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(opts.IgnoreResources, opts.Resources)
	if err != nil {
		return backupDirectory, err
	}
	// Apply the version-related patch, on the rollout nodes
	allNodes := []core_v1.Node{}
	allNodes = append(allNodes, rolloutNodes...)
	allNodes = append(allNodes, otherNodes...)
	nodeResources := convertToStreamlinerResource(allNodes)
	err = m.applyVersionPatch(nodeResources, projectOptions, dryRun)
	if err != nil {
		return
	}
	logger.Info("The canary realease is now complete.")
	return backupDirectory, err
}

func defineRestOfNodes(nodeList core_v1.NodeList, NumberOfCanaryNodes int) (otherNodes []core_v1.Node) {
	otherNodes = nodeList.Items[NumberOfCanaryNodes:]
	return
}
