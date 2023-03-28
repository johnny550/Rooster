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

	"go.uber.org/zap"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	logger = new(zap.Logger)
)

func init() {
	logger, _ = zap.NewProduction()
}

// --------------------- READ -------------------------------
func GetService(clt K8sClient, namespace string, name string) (svc *unstructured.Unstructured, err error) {
	logger.Info("Getting service " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "Service"
	svc, err = clt.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetDaemonSet(clt K8sClient, namespace string, name string) (ds *unstructured.Unstructured, err error) {
	logger.Info("Getting daemonset " + name + " from namespace " + namespace)
	apiVersion := "apps/v1"
	kind := "DaemonSet"
	ds, err = clt.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetConfigMap(clt K8sClient, namespace string, name string) (cm *unstructured.Unstructured, err error) {
	logger.Info("Getting config map " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "ConfigMap"
	cm, err = clt.Execute(Get, apiVersion, kind, namespace, name)
	return
}

func GetServiceAccount(clt K8sClient, namespace string, name string) (sa *unstructured.Unstructured, err error) {
	logger.Info("Getting serviceAccount " + name + " from namespace " + namespace)
	apiVersion := "v1"
	kind := "ServiceAccount"
	sa, err = clt.Execute(Get, apiVersion, kind, namespace, name)
	return
}

// --------------------- DELETE ------------------------
func DeleteService(clt K8sClient, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	ctx := context.TODO()
	logger.Info("Deleting service " + name + " from namespace " + namespace)
	err := clt.GetClient().CoreV1().Services(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteDaemonSet(clt K8sClient, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	ctx := context.TODO()
	logger.Info("Deleting daemonset " + name + " from namespace " + namespace)
	err := clt.GetClient().AppsV1().DaemonSets(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteConfigMap(clt K8sClient, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	ctx := context.TODO()
	logger.Info("Deleting config map " + name + " from namespace " + namespace)
	err := clt.GetClient().CoreV1().ConfigMaps(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}

func DeleteServiceAccount(clt K8sClient, namespace string, name string, customDeleteOptions meta_v1.DeleteOptions) (bool, error) {
	ctx := context.TODO()
	logger.Info("Deleting serviceAccount " + name + " from namespace " + namespace)
	err := clt.GetClient().CoreV1().ServiceAccounts(namespace).Delete(ctx, name, customDeleteOptions)
	if err != nil {
		return false, err
	}
	return true, err
}
