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

	core_v1 "k8s.io/api/core/v1"
)

func (m *Manager) performCanaryRollout(opts RolloutOptions) (backupDirectory string, err error) {
	// Get params
	logger := m.kcm.Logger
	targetNodes := opts.NodesWithTargetlabel
	canary := opts.Canary
	// Define batch size
	rolloutNodes, batchSize := m.defineBatchSize(targetNodes, canary)
	if batchSize == 0 {
		err = errors.New("You may want to review the canary/increment.")
		return
	}
	// Verify the batch nodes
	err = utils.MatchBatch(targetNodes.Items, rolloutNodes)
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
	otherNodes := defineRestOfNodes(targetNodes, len(rolloutNodes))
	opts.RolloutNodes = otherNodes
	// Update the batch size
	updatedBatchSize := len(targetNodes.Items) - int(batchSize)
	opts.BatchSize = float64(updatedBatchSize)
	// Complete the rollout
	logger.Info("Patching remaining nodes...")
	_, err = m.patchNodes(opts)
	if err != nil {
		return backupDirectory, err
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(opts.Resources)
	if err != nil {
		return backupDirectory, err
	}
	logger.Info("The canary realease is now complete.")
	return backupDirectory, err
}

func defineRestOfNodes(nodeList core_v1.NodeList, NumberOfCanaryNodes int) (otherNodes []core_v1.Node) {
	otherNodes = nodeList.Items[NumberOfCanaryNodes:]
	return
}
