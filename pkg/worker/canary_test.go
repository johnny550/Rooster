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
	"testing"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CanaryTest struct {
	suite.Suite
}

func (suite *CanaryTest) TestInvalidCanary() {
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	options := RolloutOptions{
		Strategy:             "canary",
		NodesWithTargetlabel: core_v1.NodeList{},
		Canary:               0,
		Namespace:            "test-rooster",
	}
	expectedErr := errors.New("You may want to review the canary/increment.")
	_, err = m.performCanaryRollout(options)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
}

func (suite *CanaryTest) TestCanaryWrongNode() {
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
		Strategy:             "canary",
		NodesWithTargetlabel: sampleNodeList,
		Canary:               10,
		Namespace:            "test-rooster",
		DryRun:               true,
		CanaryLabel:          "my-canary=label",
	}
	_, err = m.performCanaryRollout(options)
	// The sample nodes do not exist in the clusters. Patching them will be impossible.
	expectedErr := k8s_errors.IsNotFound(err)
	assert.True(suite.T(), expectedErr)
}

func TestCanary(t *testing.T) {
	s := new(CanaryTest)
	suite.Run(t, s)
}
