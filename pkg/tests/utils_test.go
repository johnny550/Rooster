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

package tests

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	core_v1 "k8s.io/api/core/v1"
)

type StreamlinerUtilsTest struct {
	suite.Suite
}

type plural struct {
	Group    string
	Version  string
	Resource string
}

func (suite *StreamlinerUtilsTest) SetupSuite() {
	fmt.Println(" SetupSuite")
	customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	customListOptions.Limit = 1
	fmt.Printf("customDeleteOptions: %v\n", customDeleteOptions)
}

func (suite *StreamlinerUtilsTest) TestGroupVersionGuess() {
	apiVersion := "v1"
	kind := "pod"
	expectedResult := plural{}
	expectedResult.Group = ""
	expectedResult.Resource = kind + "s"
	expectedResult.Version = apiVersion
	val, err := utils.UnsafeGuessGroupVersionResource(apiVersion, kind)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), expectedResult.Group, val.Group)
	assert.Equal(suite.T(), expectedResult.Version, val.Version)
	assert.Equal(suite.T(), expectedResult.Resource, val.Resource)
}

func (suite *StreamlinerUtilsTest) TestShellScript() {
	cmd := "pwd"
	result, err := utils.Shell(cmd)
	assert.Nil(suite.T(), err)
	assert.NotEmpty(suite.T(), result)
	assert.Contains(suite.T(), result, "pkg/tests")
}

func (suite *StreamlinerUtilsTest) TestAssessmentOptions() {
	testPackage := "my-test-package"
	testBinary := "my-test-binary"
	skip, err := utils.ValidateTestOptions(testPackage, testBinary)
	assert.Nil(suite.T(), err)
	assert.False(suite.T(), skip)
}

func (suite *StreamlinerUtilsTest) TestAssessmentOptionsAbsence() {
	testPackage := ""
	testBinary := ""
	skip, err := utils.ValidateTestOptions(testPackage, testBinary)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), skip)
}

func (suite *StreamlinerUtilsTest) TestAssessmentPackageFailure() {
	testPackage := ""
	testBinary := "my-test-binary"
	skip, err := utils.ValidateTestOptions(testPackage, testBinary)
	expectedErr := errors.New("test package not defined")
	assert.EqualError(suite.T(), err, expectedErr.Error())
	assert.False(suite.T(), skip)
}

func (suite *StreamlinerUtilsTest) TestAssessmentBinaryFailure() {
	testPackage := "my-test-package"
	testBinary := ""
	skip, err := utils.ValidateTestOptions(testPackage, testBinary)
	expectedErr := errors.New("test binary not defined")
	assert.EqualError(suite.T(), err, expectedErr.Error())
	assert.False(suite.T(), skip)
}

func (suite *StreamlinerUtilsTest) TestMatchBatchFailure() {
	rolloutNodes := []core_v1.Node{}
	testNodes := []core_v1.Node{}
	node := core_v1.Node{}
	for i := 0; i <= 2; i++ {
		node.Name = "my-test-node-" + strconv.Itoa(i)
		testNodes = append(testNodes, node)
	}
	rolloutNodes = append(rolloutNodes, testNodes...)
	err := utils.MatchBatch(testNodes, rolloutNodes)
	assert.NotNil(suite.T(), err)
}

func (suite *StreamlinerUtilsTest) TestMatchBatchSuccess() {
	rolloutNodes := []core_v1.Node{}
	testNodes := []core_v1.Node{}
	node := core_v1.Node{}
	for i := 0; i <= 2; i++ {
		node.Name = "my-test-node-" + strconv.Itoa(i)
		testNodes = append(testNodes, node)
	}
	node.Name = "my-target-node"
	rolloutNodes = append(rolloutNodes, node)
	err := utils.MatchBatch(testNodes, rolloutNodes)
	assert.Nil(suite.T(), err)
}

func TestUtils(t *testing.T) {
	s := new(StreamlinerUtilsTest)
	suite.Run(t, s)
}
