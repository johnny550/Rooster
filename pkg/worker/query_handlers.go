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

	"go.uber.org/zap"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAttribute(d string, i int) (attribute string) {
	data := strings.Split(d, ",")
	// i=0 : Kind
	// i=1 : Name
	attribute = data[i]
	return
}

func (c Clients) queryResources(logger *zap.Logger, verb utils.Verb, targetResources map[string]string, dryRun bool) (allExist bool, resources []unstructured.Unstructured) {
	resources = []unstructured.Unstructured{}
	allExist = true
	for kindName, namespace := range targetResources {
		kind := getAttribute(kindName, 0)
		name := getAttribute(kindName, 1)
		switch verb {
		case utils.Get:
			resource, err := c.getResource(kind, name, namespace)
			if resource != nil {
				resources = append(resources, *resource)
			}
			if err != nil {
				logger.Warn(err.Error())
				allExist = false
			}
		case utils.Delete:
			c.deleteResource(kind, name, namespace, dryRun)
		case utils.Update:
			logger.Warn("Update not defined yet...")
		case utils.Create:
			logger.Warn("Create not defined yet...")
		default:
			logger.Error("Verb is unknown")
			return
		}

	}
	return
}

func (c Clients) getResource(kind string, name string, namespace string) (resource *unstructured.Unstructured, err error) {
	switch kind {
	case "Service":
		resource, err = utils.GetService(c.K8sClient, namespace, name)
	case "DaemonSet":
		resource, err = utils.GetDaemonSet(c.K8sClient, namespace, name)
	case "ConfigMap":
		resource, err = utils.GetConfigMap(c.K8sClient, namespace, name)
	case "ServiceAccount":
		resource, err = utils.GetServiceAccount(c.K8sClient, namespace, name)
	}
	return
}

func (c Clients) deleteResource(kind string, name string, namespace string, dryRun bool) (opComplete bool, err error) {
	customDeleteOptions := meta_v1.DeleteOptions{}
	if dryRun {
		customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	}
	switch kind {
	case "Service":
		opComplete, err = utils.DeleteService(c.K8sClient, namespace, name, customDeleteOptions)
	case "DaemonSet":
		opComplete, err = utils.DeleteDaemonSet(c.K8sClient, namespace, name, customDeleteOptions)
	case "ConfigMap":
		opComplete, err = utils.DeleteConfigMap(c.K8sClient, namespace, name, customDeleteOptions)
	case "ServiceAccount":
		opComplete, err = utils.DeleteServiceAccount(c.K8sClient, namespace, name, customDeleteOptions)
	}
	return
}
