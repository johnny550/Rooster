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
	"errors"
	"fmt"
	"os/exec"
	"strings"

	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func sh(ctx context.Context, format string, args ...interface{}) (string, error) {
	command := fmt.Sprintf(format, args...)
	c := exec.CommandContext(ctx, "sh", "-c", command)
	bytes, err := c.CombinedOutput()
	return string(bytes), err
}

func Shell(format string, args ...interface{}) (string, error) {
	return sh(context.Background(), format, args...)
}

func KubectlEmulator(namespace, subcommand string, args ...string) (string, error) {
	var cmd string
	rest := strings.Join(args, " ")
	switch len(args) {
	case 0:
		cmd = fmt.Sprintf("kubectl %s %s", subcommand, rest)
	case 1:
		cmd = fmt.Sprintf("kubectl -n %s %s -f %s", namespace, subcommand, args[0])
	default:
		cmd = fmt.Sprintf("kubectl -n %s %s %s", namespace, subcommand, rest)
	}
	return Shell(cmd)
}

func UnsafeGuessGroupVersionResource(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	// get group version
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}

	// guess resource from kind
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Group:   groupVersion.Group,
		Version: groupVersion.Version,
		Kind:    kind,
	})

	return &plural, nil
}

func ValidateTestOptions(testPackage, testBinary string) (skip bool, err error) {
	// If the test related options were not specified, skip tests
	if testPackage == "" && testBinary == "" {
		skip = true
		return
	}
	if testPackage == "" {
		err = errors.New("test package not defined")
		return
	}
	if testBinary == "" {
		err = errors.New("test binary not defined")
		return
	}
	return
}

// Ensures the nodes that will be targeted for a rollout (batchNodes), are not as many as nodes carrying the target label.
// With this method, Rooster denies Canary, or increment values equal to 100%
func MatchBatch(nodesWithTargetLabel, batchNodes []core_v1.Node) (err error) {
	if len(nodesWithTargetLabel) == len(batchNodes) {
		err = errors.New("The batch size cannot be equal to the total number of nodes to consider for the rollout. It must be inferior to the latter.")
	}
	return
}
