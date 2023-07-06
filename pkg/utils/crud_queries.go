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

package utils

import (
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// Dynamic queries
func (m *K8sClientManager) PatchResourcesDynamically(apiVersion string, kind string, namespace string, name string, patchType types.PatchType, patchData []byte, patchOpts meta_v1.PatchOptions) (
	updatedResource *unstructured.Unstructured, err error) {
	opts := DynamicQueryOptions{
		PatchData:    patchData,
		PatchType:    patchType,
		PatchOptions: patchOpts,
	}
	updatedResource, err = m.Execute(Patch, apiVersion, kind, namespace, name, opts)
	if err != nil {
		return updatedResource, err
	}
	m.Logger.Sugar().Infof("Patched %s %s", kind, name)
	return updatedResource, err
}

func (m *K8sClientManager) GetResourcesDynamically(apiVersion, kind, namespace, name string, getOpts meta_v1.GetOptions) (res *unstructured.Unstructured, err error) {
	opts := DynamicQueryOptions{
		GetOptions: getOpts,
	}
	res, err = m.Execute(Get, apiVersion, kind, namespace, name, opts)
	if err != nil {
		return nil, err
	}
	return
}

func (m *K8sClientManager) DeleteResourcesDynamically(apiVersion, kind, namespace, name string, deleteOpts meta_v1.DeleteOptions) (res *unstructured.Unstructured, err error) {
	opts := DynamicQueryOptions{
		DeleteOptions: deleteOpts,
	}
	_, err = m.Execute(Delete, apiVersion, kind, namespace, name, opts)
	if err != nil {
		return nil, err
	}
	return
}

func (m *K8sClientManager) ListResourcesDynamically(apiVersion, kind, namespace string, listOpts meta_v1.ListOptions) (objList *unstructured.UnstructuredList, err error) {
	opts := DynamicQueryOptions{
		ListOptions: listOpts,
	}
	objList, err = m.ExecuteList(List, apiVersion, kind, namespace, opts)
	if err != nil {
		return nil, err
	}
	return
}
