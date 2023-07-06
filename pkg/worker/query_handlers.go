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

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getAttribute(d string, i int) (attribute string) {
	data := strings.Split(d, ",")
	attribute = data[i]
	return
}

func (m *Manager) queryResources(verb utils.Verb, targetResources []Resource, dryRun bool) (resources []unstructured.Unstructured, err error) {
	logger := m.kcm.Logger
	resources = []unstructured.Unstructured{}
	for _, currRes := range targetResources {
		kind := currRes.Kind
		name := currRes.Name
		namespace := currRes.Namespace
		switch verb {
		case utils.Get:
			resource, err := m.getResource(kind, name, namespace)
			if resource != nil {
				resources = append(resources, *resource)
			}
			if err != nil {
				return resources, err
			}
		case utils.Delete:
			_, err := m.deleteResource(kind, name, namespace, dryRun)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return resources, err
			}
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

func (m *Manager) getResource(kind string, name string, namespace string) (resource *unstructured.Unstructured, err error) {
	switch kind {
	case "Service":
		resource, err = utils.GetService(m.kcm, namespace, name)
	case "DaemonSet":
		resource, err = utils.GetDaemonSet(m.kcm, namespace, name)
	case "ConfigMap":
		resource, err = utils.GetConfigMap(m.kcm, namespace, name)
	case "ServiceAccount":
		resource, err = utils.GetServiceAccount(m.kcm, namespace, name)
	}
	return
}

func (m *Manager) deleteResource(kind string, name string, namespace string, dryRun bool) (opComplete bool, err error) {
	customDeleteOptions := meta_v1.DeleteOptions{}
	if dryRun {
		customDeleteOptions.DryRun = append(customDeleteOptions.DryRun, "All")
	}
	switch kind {
	case "Service":
		opComplete, err = utils.DeleteService(m.kcm, namespace, name, customDeleteOptions)
	case "DaemonSet":
		opComplete, err = utils.DeleteDaemonSet(m.kcm, namespace, name, customDeleteOptions)
	case "ConfigMap":
		opComplete, err = utils.DeleteConfigMap(m.kcm, namespace, name, customDeleteOptions)
	case "ServiceAccount":
		opComplete, err = utils.DeleteServiceAccount(m.kcm, namespace, name, customDeleteOptions)
	}
	return
}
