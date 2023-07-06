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
	"rooster/pkg/utils"
)

/**
* Goal: Perform a linear rollout
* Ref: https://confluence.rakuten-it.com/confluence/display/CCOG/v1.1.0+-+Streamliner#v1.1.0Streamliner-Linearrollout
* Will:
* - Determine the nodes to target first
* - Perform a rollout
* - Label the target nodes with the version of the resources they host
**/
func (m *Manager) performLinearRollout(opts RoosterOptions) (backupDirectory string, err error) {
	// Get params
	logger := m.kcm.Logger
	incr := opts.Increment
	projectOptions := opts.ProjectOpts
	dryRun := opts.DryRun
	// new targets may include nodes that have don't have the target resources (pods) running on them
	newTargets, err := m.DefineTargetNodes(opts)
	if err != nil {
		return
	}
	if len(newTargets.Items) == 0 {
		err = utils.MakeRollloutLimitErr()
		return
	}
	// based off those nodes, determine the batch size
	logger.Sugar().Infof("Potential target nodes: %d", len(newTargets.Items))
	// Define batch size
	rolloutNodes, batchSize := m.calBatchSize(newTargets, incr)
	if err = utils.ValidateBatchSize(int(batchSize)); err != nil {
		return
	}
	// Verify the batch nodes
	// err = utils.MatchBatch(targetNodes.Items, rolloutNodes)
	err = utils.MatchBatch(newTargets.Items, rolloutNodes)
	if err != nil {
		return
	}
	// BATCH ROLLOUT
	opts.BatchSize = batchSize
	opts.RolloutNodes = rolloutNodes
	backupDirectory, err = m.performRollout(opts)
	if err != nil {
		return backupDirectory, err
	}
	// Apply the version-related patch, on the rollout nodes
	nodeResources := convertToStreamlinerResource(rolloutNodes)
	err = m.applyVersionPatch(nodeResources, projectOptions, dryRun)
	logger.Info("The linear realease is now complete.")
	return
}
