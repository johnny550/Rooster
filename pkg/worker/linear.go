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

	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *Manager) performLinearRollout(opts RolloutOptions) (backupDirectory string, err error) {
	// Get params
	logger := m.kcm.Logger
	targetNodes := opts.NodesWithTargetlabel
	incr := opts.Increment
	// Define batch size
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = opts.CanaryLabel
	nodesWithControlLabel, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	// defineNewTargetNodes
	// get nodes that aren't common to the 2 slices
	newTargets := extractUncommonNodes(targetNodes, nodesWithControlLabel)
	if len(newTargets.Items) == 0 {
		logger.Info("All nodes already carry the control label")
		newTargets.Items = append(newTargets.Items, targetNodes.Items...)
	}
	// based off those nodes, determine the batch size
	logger.Sugar().Infof("Potential target nodes: %d", len(newTargets.Items))
	logger.Sugar().Infof("increment: %d", incr)
	rolloutNodes, batchSize := m.defineBatchSize(newTargets, incr)
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
	opts.BatchSize = batchSize
	opts.RolloutNodes = rolloutNodes
	backupDirectory, err = m.performRollout(opts)
	if err != nil {
		return backupDirectory, err
	}
	logger.Info("The linear realease is now complete.")
	return
}

func extractUncommonNodes(targetNodes, canaryNodes core_v1.NodeList) (nodesWithoutCanaryLabel core_v1.NodeList) {
	markedNodes := make(map[string]core_v1.Node)
	for _, n := range canaryNodes.Items {
		markedNodes[n.Name] = n
	}
	for _, n := range targetNodes.Items {
		if node := markedNodes[n.Name]; node.Name == "" {
			nodesWithoutCanaryLabel.Items = append(nodesWithoutCanaryLabel.Items, n)
		}
	}
	return
}
