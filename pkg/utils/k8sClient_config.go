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
	"fmt"
	"log"
	"os"
	"path/filepath"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sClient struct {
	client        *kubernetes.Clientset
	dynamicClient *dynamic.Interface
}

func getConfig(kubeconfigPath string) (config *rest.Config, err error) {
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(
			os.Getenv("HOME"), ".kube", "config",
		)
	}
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	return config, err
}

func New(kubeConfig string) (*K8sClient, error) {
	client, err := newClient(kubeConfig)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := newDynamicClient(kubeConfig)
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		client:        client,
		dynamicClient: &dynamicClient,
	}, nil
}

func newClient(kubeConfig string) (client *kubernetes.Clientset, err error) {
	config, err := getConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	return client, err
}

func newDynamicClient(kubeConfig string) (client dynamic.Interface, err error) {
	config, err := getConfig(kubeConfig)
	if err != nil {
		log.Fatal(err)
	}
	client, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	return client, err
}

func (m *K8sClient) GetClient() *kubernetes.Clientset {
	return m.client
}

func (m *K8sClient) GetDynamicClient() *dynamic.Interface {
	return m.dynamicClient
}

func (m *K8sClient) Execute(verb Verb, apiVersion string, kind string, namespace string, name string) (*unstructured.Unstructured, error) {
	// Define the context
	ctx := context.TODO()
	// Define the Group-Version-Resource object
	gvr, err := UnsafeGuessGroupVersionResource(apiVersion, kind)
	if err != nil {
		logger.Error(err.Error())
	}
	// Run the command
	switch verb {
	case Get:
		return (*m.dynamicClient).Resource(*gvr).Namespace(namespace).Get(ctx, name, meta_v1.GetOptions{})
	case Delete:
		return nil, (*m.dynamicClient).Resource(*gvr).Namespace(namespace).Delete(ctx, name, meta_v1.DeleteOptions{})
	default:
		return nil, fmt.Errorf("verb is invalid. (%+v)", verb)
	}
}
