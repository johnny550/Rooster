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
	"context"
	"fmt"
	"testing"

	"rooster/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubernetesClientTest struct {
	suite.Suite
}

func (suite *KubernetesClientTest) TestKubernetesClient() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	_, err = m.GetClient().CoreV1().Pods("default").List(context.TODO(), meta_v1.ListOptions{})
	assert.Nil(suite.T(), err)
}

func (suite *KubernetesClientTest) TestKubernetesDynamicClientGet() {
	svcName := "kube-dns"
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	svc, err := m.Execute(utils.Get, "v1", "Service", "kube-system", svcName)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), svc.GetName(), svcName)
}

func (suite *KubernetesClientTest) TestKubernetesClientDelete() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	ns := "default"
	podLists, err := getPodsList(m, ns)
	assert.Nil(suite.T(), err)
	if len(podLists.Items) == 0 {
		ns = "caas-sentinel"
		podLists, err = getPodsList(m, ns)
		assert.Nil(suite.T(), err)
	}
	assert.NotEmpty(suite.T(), podLists.Items)
	// Test on the 1st pod we get
	targetPod := podLists.Items[0].Name
	ctx := context.TODO()
	customDeleteOptions := meta_v1.DeleteOptions{}
	customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	fmt.Printf("target ns: %v\n", ns)
	fmt.Printf("target Pod: %v\n", targetPod)
	err = m.GetClient().CoreV1().Pods(ns).Delete(ctx, targetPod, customDeleteOptions)
	assert.Nil(suite.T(), err)
}

func (suite *KubernetesClientTest) TestKubernetesDynamicClientDelete() {
	m, err := utils.New("")
	assert.Nil(suite.T(), err)
	ns := "default"
	podLists, err := getPodsList(m, ns)
	assert.Nil(suite.T(), err)
	if len(podLists.Items) == 0 {
		ns = "caas-sentinel"
		podLists, err = getPodsList(m, ns)
		assert.Nil(suite.T(), err)
	}
	assert.NotEmpty(suite.T(), podLists.Items)
	// Test on the 1st pod we get
	targetPod := podLists.Items[0].Name
	customDeleteOptions := meta_v1.DeleteOptions{}
	customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	fmt.Printf("target ns: %v\n", ns)
	fmt.Printf("target Pod: %v\n", targetPod)
	_, err = m.Execute(utils.Delete, "v1", "Pod", ns, targetPod)
	assert.Nil(suite.T(), err)
}

func getPodsList(c *utils.K8sClient, namespace string) (*core_v1.PodList, error) {
	return c.GetClient().CoreV1().Pods(namespace).List(context.TODO(), meta_v1.ListOptions{})
}

func TestClient(t *testing.T) {
	s := new(KubernetesClientTest)
	suite.Run(t, s)
}
