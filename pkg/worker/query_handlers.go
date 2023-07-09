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

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (m *Manager) queryResources(verb utils.Verb, targetResources []Resource, dynamicOptions utils.DynamicQueryOptions) (resources []unstructured.Unstructured, err error) {
	logger := m.kcm.Logger
	resources = []unstructured.Unstructured{}
	for _, currRes := range targetResources {
		kind := currRes.Kind
		name := currRes.Name
		namespace := currRes.Namespace
		apiVersion := currRes.ApiVersion
		switch verb {
		case utils.Get:
			getOpts := dynamicOptions.GetOptions
			resource, err := m.kcm.GetResourcesDynamically(apiVersion, kind, namespace, name, getOpts)
			if resource != nil {
				resources = append(resources, *resource)
			}
			if err != nil {
				return resources, err
			}
		case utils.Delete:
			deleteOpts := dynamicOptions.DeleteOptions
			_, err := m.kcm.DeleteResourcesDynamically(apiVersion, kind, namespace, name, deleteOpts)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return resources, err
			}
		case utils.Patch:
			patchOpts := dynamicOptions.PatchOptions
			patchType := dynamicOptions.PatchType
			patchData := dynamicOptions.PatchData
			_, err := m.kcm.PatchResourcesDynamically(apiVersion, kind, namespace, name, patchType, patchData, patchOpts)
			if err != nil {
				return resources, err
			}
		case utils.List:
			listOpts := dynamicOptions.ListOptions
			r, err := m.kcm.ListResourcesDynamically(apiVersion, kind, namespace, listOpts)
			if err != nil {
				return resources, err
			}
			resources = r.Items
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

func (m *Manager) retrieveConfigMapContent(cmRes Resource) (cmdata utils.CmData, queryErr error) {
	// get the cm
	dynamicOpts := utils.DynamicQueryOptions{}
	objs, queryErr := m.queryResources(utils.Get, []Resource{cmRes}, dynamicOpts)
	if queryErr != nil {
		return
	}
	// extract the cm's data
	cmdata, queryErr = utils.ExtractConfigMapData(objs[0])
	if queryErr != nil {
		return
	}
	return
}
