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
	"testing"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LinearTest struct {
	suite.Suite
}

func (suite *LinearTest) TestTargetDefinition() {
	nodeSet1 := core_v1.NodeList{}
	node := core_v1.Node{}
	for i := 0; i >= 2; i++ {
		node.Name = "my-test-node-" + strconv.Itoa(i)
		nodeSet1.Items = append(nodeSet1.Items, node)
	}
	nodeSet2 := core_v1.NodeList{}
	for i := 0; i >= 4; i++ {
		node.Name = "my-test-node-" + strconv.Itoa(i)
		nodeSet2.Items = append(nodeSet2.Items, node)
	}
	expectedSet := core_v1.NodeList{}
	for i := 3; i == 4; i++ {
		expectedSet.Items = append(expectedSet.Items, node)
	}
	nodeSet3 := extractUncommonNodes(nodeSet1, nodeSet2)
	assert.Equal(suite.T(), nodeSet3.Items, expectedSet.Items)
}

func (suite *LinearTest) TestInvalidIncrement() {
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	options := RolloutOptions{
		Strategy:             "linear",
		NodesWithTargetlabel: core_v1.NodeList{},
		Increment:            0,
		Namespace:            "test-rooster",
		DryRun:               true,
	}
	expectedErr := errors.New("You may want to review the canary/increment.")
	_, err = m.performLinearRollout(options)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
}

func (suite *LinearTest) TestMaxIncrement() {
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	sampleNodes := []core_v1.Node{}
	// populate sampleNodes
	for i := 0; i < 9; i++ {
		name := fmt.Sprintf("test-node-%d", i)
		metadata := meta_v1.ObjectMeta{}
		metadata.Name = name
		node := core_v1.Node{ObjectMeta: metadata}
		sampleNodes = append(sampleNodes, node)
	}
	sampleNodeList := core_v1.NodeList{
		Items: sampleNodes,
	}
	options := RolloutOptions{
		Strategy:             "linear",
		NodesWithTargetlabel: sampleNodeList,
		Increment:            100,
		Namespace:            "test-rooster",
		DryRun:               true,
	}
	expectedErr := errors.New("The batch size cannot be equal to the total number of nodes to consider for the rollout. It must be inferior to the latter.")
	_, err = m.performLinearRollout(options)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
}

func TestLinear(t *testing.T) {
	s := new(LinearTest)
	suite.Run(t, s)
}
