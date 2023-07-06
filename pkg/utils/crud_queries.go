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
	"context"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// --------------------- READ ------------------------------- //
func GetService(kcm K8sClientManager, namespace string, name string) (svc *unstructured.Unstructured, err error) {
	logger := kcm.Logger
	logger.Info("Getting service " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "Service"
	svc, err = kcm.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetDaemonSet(kcm K8sClientManager, namespace string, name string) (ds *unstructured.Unstructured, err error) {
	logger := kcm.Logger
	logger.Info("Getting daemonset " + name + " from namespace " + namespace)
	apiVersion := "apps/v1"
	kind := "DaemonSet"
	ds, err = kcm.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetConfigMap(kcm K8sClientManager, namespace string, name string) (cm *unstructured.Unstructured, err error) {
	logger := kcm.Logger
	logger.Info("Getting config map " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "ConfigMap"
	cm, err = kcm.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetServiceAccount(kcm K8sClientManager, namespace string, name string) (sa *unstructured.Unstructured, err error) {
	logger := kcm.Logger
	logger.Info("Getting serviceAccount " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "ServiceAccount"
	sa, err = kcm.Execute(Get, apiVersion, kind, namespace, name)
	return
}

// --------------------- DELETE ------------------------ //
func DeleteService(kcm K8sClientManager, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	logger := kcm.Logger
	ctx := context.TODO()
	logger.Info("Deleting service " + name + " from namespace " + namespace)
	err := kcm.Client.CoreV1().Services(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteDaemonSet(kcm K8sClientManager, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	logger := kcm.Logger
	ctx := context.TODO()
	logger.Info("Deleting daemonset " + name + " from namespace " + namespace)
	err := kcm.Client.AppsV1().DaemonSets(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteConfigMap(kcm K8sClientManager, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	logger := kcm.Logger
	ctx := context.TODO()
	logger.Info("Deleting config map " + name + " from namespace " + namespace)
	err := kcm.Client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteServiceAccount(kcm K8sClientManager, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	logger := kcm.Logger
	ctx := context.TODO()
	logger.Info("Deleting serviceAccount " + name + " from namespace " + namespace)
	err := kcm.Client.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}
