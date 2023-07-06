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
	"fmt"
	"log"
	"os/exec"
	"testing"
	"time"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CrudRoosterTest struct {
	suite.Suite
}

const (
	ns        = "../tests/testdata/test_ns/test_ns.yaml"
	daemonset = "../tests/testdata/others/ds.yaml"
	service   = "../tests/testdata/others/svc.yaml"
	namespace = "test-rooster"
	dryRun    = true
)

func (suite *CrudRoosterTest) SetupSuite() {
	cmd := fmt.Sprintf("kubectl apply -f %v", ns)
	output, err := shell(context.Background(), cmd)
	assert.NotNil(suite.T(), output)
	assert.Nil(suite.T(), err)
	ready := isNamespaceSet(namespace)
	assert.True(suite.T(), ready)
	// Create the other resources in the namespace
	resources := []string{daemonset, service}
	for _, r := range resources {
		cmd = fmt.Sprintf("kubectl apply -f %v", r)
		output, err = shell(context.Background(), cmd)
		assert.NotNil(suite.T(), output)
		assert.Nil(suite.T(), err)
	}
}

func (suite *CrudRoosterTest) TestService() {
	name := "my-service"
	svc := &unstructured.Unstructured{}
	done := false
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	tests := []string{"GetService", "DeleteService"}
	for _, t := range tests {
		suite.Run(t, func() {
			switch t {
			case "GetService":
				svc, err = m.getResource("Service", name, namespace)
				assert.NotNil(suite.T(), svc)
				assert.Equal(suite.T(), svc.GetName(), name)
			case "DeleteService":
				done, err = m.deleteResource("Service", name, namespace, dryRun)
				assert.True(suite.T(), done)
			}
			assert.Nil(suite.T(), err)
		})
	}
}

func (suite *CrudRoosterTest) TestServiceAccount() {
	name := "default"
	sa := &unstructured.Unstructured{}
	done := false
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	tests := []string{"GetServiceAccount", "DeleteServiceAccount"}
	for _, t := range tests {
		suite.Run(t, func() {
			switch t {
			case "GetServiceAccount":
				sa, err = m.getResource("ServiceAccount", name, namespace)
				assert.NotNil(suite.T(), sa)
				assert.Equal(suite.T(), sa.GetName(), name)
			case "DeleteServiceAccount":
				done, err = m.deleteResource("ServiceAccount", name, namespace, dryRun)
				assert.True(suite.T(), done)
			}
			assert.Nil(suite.T(), err)
		})
	}
}

func (suite *CrudRoosterTest) TestConfigMap() {
	name := "kube-root-ca.crt"
	cm := &unstructured.Unstructured{}
	done := false
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	tests := []string{"GetConfigMap", "DeleteConfigMap"}
	for _, t := range tests {
		suite.Run(t, func() {
			switch t {
			case "GetServiceAccount":
				cm, err = m.getResource("ConfigMap", name, namespace)
				assert.NotNil(suite.T(), cm)
				assert.Equal(suite.T(), cm.GetName(), name)
			case "DeleteServiceAccount":
				done, err = m.deleteResource("ConfigMap", name, namespace, dryRun)
				assert.True(suite.T(), done)
			}
			assert.Nil(suite.T(), err)
		})
	}
}

func (suite *CrudRoosterTest) TestDaemonSet() {
	name := "fluentd-elasticsearch"
	ds := &unstructured.Unstructured{}
	done := false
	manager, err := utils.New("")
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), manager)
	m := Manager{
		kcm: *manager,
	}
	tests := []string{"GetDaemonSet", "DeleteDaemonSet"}
	for _, t := range tests {
		suite.Run(t, func() {
			switch t {
			case "GetDaemonSet":
				ds, err = m.getResource("DaemonSet", name, namespace)
				assert.NotNil(suite.T(), ds)
				assert.Equal(suite.T(), ds.GetName(), name)
			case "DeleteDaemonSet":
				done, err = m.deleteResource("DaemonSet", name, namespace, dryRun)
				assert.True(suite.T(), done)
			}
			assert.Nil(suite.T(), err)
		})
	}
}

func TestCrud(t *testing.T) {
	s := new(CrudRoosterTest)
	suite.Run(t, s)
}

// ------------------------ HELPERS ------------------------ //

func shell(ctx context.Context, format string, args ...interface{}) (string, error) {
	command := fmt.Sprintf(format, args...)
	c := exec.CommandContext(ctx, "sh", "-c", command)
	bytes, err := c.CombinedOutput()
	return string(bytes), err
}

func isNamespaceSet(namespace string) bool {
	manager, _ := utils.New("")
	timeout := time.Now().Add(60 * time.Second)
	ready := false
outer:
	for {
		time.Sleep(10 * time.Second)
		if time.Now().After(timeout) {
			break
		}
		ns, err := manager.Client.CoreV1().Namespaces().Get(context.Background(), namespace, meta_v1.GetOptions{})
		if err != nil {
			log.Fatal(err)
		}
		if ns == nil {
			continue outer
		}
		return true
	}
	return ready
}
