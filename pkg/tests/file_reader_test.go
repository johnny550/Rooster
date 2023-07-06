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
	"testing"

	"rooster/pkg/worker"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type Readertest struct {
	suite.Suite
}

func (suite *Readertest) TestDirectoryAssessmentFailure() {
	path := "complete/gibberish/in/here"
	found := worker.CheckDirectoryExistence(path)
	assert.False(suite.T(), found)
}
func (suite *Readertest) TestDirectoryAssessmentSuccess() {
	path := "./testdata/non_empty_file/file-2.yaml"
	found := worker.CheckDirectoryExistence(path)
	assert.True(suite.T(), found)
}

func (suite *Readertest) TestReadFilePathFailure() {
	path := "./testdata/empty_file"
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// Handle panic generated by ReadManifestFiles
	defer func() {
		if err := recover(); err != nil {
			expectedErr := errors.New("stat ./testdata/empty_filefile-1.yaml: no such file or directory")
			assert.Equal(suite.T(), expectedErr.Error(), err)
		}
	}()
	worker.ReadManifestFiles(logger, path, ns)
}

func (suite *Readertest) TestReadFileNamespaceFailure() {
	path := "./testdata/non_empty_file/"
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// Handle panic generated by ReadManifestFiles
	defer func() {
		if err := recover(); err != nil {
			assert.Contains(suite.T(), err.(string), "Namespace conflict detected ")
		}
	}()
	worker.ReadManifestFiles(logger, path, ns)
}

func (suite *Readertest) TestReadEmptyFile() {
	path := "./testdata/empty_file/"
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	ref, err := worker.ReadManifestFiles(logger, path, ns)
	assert.Nil(suite.T(), err)
	assert.Empty(suite.T(), ref)
}

func (suite *Readertest) TestReadFileSuccess() {
	path := "./testdata/non_empty_file/"
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	ref, err := worker.ReadManifestFiles(logger, path, ns)
	expectedResource := worker.Resource{
		ApiVersion: "v1",
		Name:       "nginx",
		Kind:       "Pod",
		Manifest:   path + "file-2.yaml",
		Namespace:  ns,
		Ready:      false,
	}
	assert.Nil(suite.T(), err)
	assert.NotEmpty(suite.T(), ref)
	assert.Equal(suite.T(), []worker.Resource{expectedResource}, ref)
}

func (suite *Readertest) TestReadFileContainingMultipleResources() {
	path := "./testdata/combined_resources/"
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	ref, err := worker.ReadManifestFiles(logger, path, ns)
	expectedResource1 := worker.Resource{
		ApiVersion: "v1",
		Name:       "busybox",
		Kind:       "Pod",
		Manifest:   path + "file-3.yaml",
		Namespace:  ns,
		Ready:      false,
	}
	expectedResource2 := worker.Resource{
		ApiVersion: "v1",
		Name:       "istio-ca-root-cert",
		Kind:       "ConfigMap",
		Manifest:   path + "file-3.yaml",
		Namespace:  ns,
		Ready:      false,
	}
	assert.Nil(suite.T(), err)
	assert.NotEmpty(suite.T(), ref)
	assert.Equal(suite.T(), []worker.Resource{expectedResource1, expectedResource2}, ref)
}

func TestReader(t *testing.T) {
	s := new(Readertest)
	suite.Run(t, s)
}
