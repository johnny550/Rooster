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
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"rooster/pkg/config"

	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func DefineVersion(indicatedVersion, action string) string {
	// why? because for those two actions, no need to create a version. Rooster works with the current version
	exemptions := []string{"rollback", "scale-down"}
	if indicatedVersion == "" && !strings.Contains(strings.Join(exemptions, ","), action) {
		ts := time.Now().Format("2006.01.02_15:04:05")
		return strings.ReplaceAll(ts, ":", "-")
	}
	return indicatedVersion
}

func VerifyVersion(version string) (bool, error) {
	return regexp.MatchString("^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$", version)
}

// Ensures the nodes that will be targeted for a rollout (batchNodes), are not as many as nodes carrying the target label.
// With this method, Streamliner denies Canary, or increment values equal to 100%
func MatchBatch(nodesWithTargetLabel, batchNodes []core_v1.Node) (err error) {
	if len(nodesWithTargetLabel) == len(batchNodes) && len(nodesWithTargetLabel) > 1 {
		err = errors.New("the batch size cannot be equal to the total number of nodes to consider for the rollout. It must be inferior to the latter")
	}
	return
}

func MakePatchOptions(dryRun bool) (opts meta_v1.PatchOptions) {
	opts = meta_v1.PatchOptions{}
	if dryRun {
		opts.DryRun = append(opts.DryRun, "All")
	}
	return
}

func MakeDeleteOptions(dryRun bool) (opts meta_v1.DeleteOptions) {
	opts = meta_v1.DeleteOptions{}
	if dryRun {
		opts.DryRun = append(opts.DryRun, "All")
	}
	return
}

func MakeCreateOptions(dryRun bool) (opts meta_v1.CreateOptions) {
	opts = meta_v1.CreateOptions{}
	if dryRun {
		opts.DryRun = append(opts.DryRun, "All")
	}
	return
}

func MakeRollloutLimitErr() error {
	return errors.New("all nodes already carry the control label")
}

func MakePatchData(prefix, op string, keyVal map[string]string) (data []byte, err error) {
	payload := []patchStringValue{}
	for k, v := range keyVal {
		path := strings.Join([]string{prefix, k}, "")
		p := patchStringValue{
			Op:   op,
			Path: path,
		}
		if v != "" {
			p.Value = v
		}
		payload = append(payload, p)
	}
	data, err = json.Marshal(payload)
	if err != nil {
		return
	}
	return
}

func ValidateBatchSize(batch int) (err error) {
	if batch == 0 {
		err = errors.New("you may want to review the canary/increment")
	}
	return
}

func DetermineNamespace(manifestIndicatedNamespace, optionIndicatedNamespace string) (finalNamespace string, err error) {
	if manifestIndicatedNamespace == "" && optionIndicatedNamespace == "" {
		finalNamespace = "default"
	} else if manifestIndicatedNamespace == "" && optionIndicatedNamespace != "" {
		finalNamespace = optionIndicatedNamespace
	} else if manifestIndicatedNamespace != "" && optionIndicatedNamespace == "" {
		finalNamespace = manifestIndicatedNamespace
	}
	switch finalNamespace != "" {
	case false:
		if optionIndicatedNamespace != manifestIndicatedNamespace {
			err = errors.New("!!! Namespace conflict detected !!!" + manifestIndicatedNamespace + " vs " + optionIndicatedNamespace)
		} else {
			finalNamespace = manifestIndicatedNamespace
		}
	}
	return
}

func MakeVersionLabel(prefix, project, version string) (versionLabelKey, versionLabel string) {
	versionLabelKey = strings.Join([]string{prefix, project}, ".")
	versionLabel = strings.Join([]string{versionLabelKey, version}, "=")
	return
}

func ExtractUncommonNodes(targetNodes, canaryNodes core_v1.NodeList) (nodesWithoutCanaryLabel core_v1.NodeList) {
	markedNodes := make(map[string]core_v1.Node)
	for _, n := range canaryNodes.Items {
		markedNodes[n.Name] = n
	}
	for _, n := range targetNodes.Items {
		if node := markedNodes[n.Name]; node.Name == "" {
			nodesWithoutCanaryLabel.Items = append(nodesWithoutCanaryLabel.Items, n)
		}
	}
	return
}

func SplitLabel(labels []string) (structuredLabel map[string]string) {
	structuredLabel = map[string]string{}
	for _, l := range labels {
		cL := strings.Split(l, "=")
		labelKey := cL[0]
		labelVal := cL[1]
		structuredLabel[labelKey] = labelVal
	}
	return
}

func IndicateNextAction() bool {
	var response string
	fmt.Println("At least one node was found carrying the indicated canary label.")
	fmt.Println("Would you like to continue? (y/n)")
	fmt.Scanln(&response)
	return strings.EqualFold(response, "Y")
}

func CheckDaemonSetStatus(dsStatus map[string]interface{}) (ready bool, err error) {
	if dsStatus == nil {
		return false, errors.New("daemonSet status was not retrieved")
	}
	desiredNumberScheduled := dsStatus["desiredNumberScheduled"]
	numberReady := dsStatus["numberReady"]
	return desiredNumberScheduled == numberReady, nil
}

func ConvertToNodeList(nodes []string) (nodeList core_v1.NodeList) {
	nodeList = core_v1.NodeList{}
	for _, n := range nodes {
		nodeObj := core_v1.Node{
			ObjectMeta: meta_v1.ObjectMeta{Name: n},
		}
		nodeList.Items = append(nodeList.Items, nodeObj)
	}
	return
}

func MakeNodeList(unstructuredNodes []unstructured.Unstructured) (nodes core_v1.NodeList) {
	nodes = core_v1.NodeList{}
	for _, n := range unstructuredNodes {
		meta := meta_v1.ObjectMeta{Name: n.GetName()}
		no := core_v1.Node{ObjectMeta: meta}
		nodes.Items = append(nodes.Items, no)
	}
	return
}

func MakeNodeNames(nodeList core_v1.NodeList) (nodes []string) {
	for _, n := range nodeList.Items {
		nodes = append(nodes, n.Name)
	}
	return
}

func ComposeConfigMapLabels() (labels map[string]string) {
	ownerTag := config.Env.CmOwnerTag
	keyVal := strings.Split(ownerTag, "=")
	labels = map[string]string{}
	labels[keyVal[0]] = keyVal[1]
	return
}
