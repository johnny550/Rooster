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
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (m *Manager) incrementalNodePatch(nodes []core_v1.Node, controlLabel string, dryRun, ignoreNotFound bool) (err error) {
	logger := m.kcm.Logger
	opts := RoosterOptions{CanaryLabel: controlLabel, DryRun: dryRun}
	// patch
	logger.Info("Preparing to patch nodes. Op: Remove")
	opts.RolloutNodes = nodes
	// define the labels to remove/add
	data := utils.SplitLabel([]string{controlLabel})
	_, err = m.patchNodes(opts, "remove", data)
	if err != nil && !ignoreNotFound {
		return
	}
	// wait
	time.Sleep(10 * time.Second)
	// patch
	logger.Info("Preparing to patch nodes. Op: Replace")
	opts.RolloutNodes = nodes
	_, err = m.patchNodes(opts, "replace", data)
	if err != nil {
		return
	}
	return
}

func (m *Manager) patchNodes(opts RoosterOptions, operation string, data map[string]string) (bool, error) {
	logger := m.kcm.Logger
	rolloutNodes := opts.RolloutNodes
	dryRun := opts.DryRun
	p := types.JSONPatchType
	logger.Info("Patching nodes")
	patchData, err := utils.MakePatchData(labelPrefix, operation, data)
	if err != nil {
		return false, err
	}
	patchOpts := utils.MakePatchOptions(dryRun)
	dynamicOpts := utils.DynamicQueryOptions{PatchOptions: patchOpts, PatchData: patchData, PatchType: p}
	nodeResources := convertToStreamlinerResource(rolloutNodes)
	// Label the nodes with the control/canary label
	_, err = m.queryResources(utils.Patch, nodeResources, dynamicOpts)
	if err != nil {
		return false, err
	}
	logger.Info("Patch operation complete")
	return true, nil
}

// Will create a config map in a given namespace or just return it if it already exists
func (m *Manager) createConfigMap(namespace string, cm core_v1.ConfigMap, dryRun bool) (myConfigmap *core_v1.ConfigMap, err error) {
	logger := m.kcm.Logger
	ctx := context.TODO()
	opts := utils.MakeCreateOptions(dryRun)
	logger.Info("Creating config map")
	return m.kcm.Client.CoreV1().ConfigMaps(namespace).Create(ctx, &cm, opts)
}

func (m *Manager) patchConfigmap(action string, projectOpts ProjectOptions, cmdata map[string]string, dryRun bool) (output []unstructured.Unstructured, err error) {
	p := types.JSONPatchType
	op := "replace"
	projectName := projectOpts.Project
	cmResourcePrj := makeCMName(projectName)
	data, err := utils.MakePatchData(cmDataPrefix, op, cmdata)
	if err != nil {
		return
	}
	patchOpts := utils.MakePatchOptions(dryRun)
	dynamicOpts := utils.DynamicQueryOptions{PatchOptions: patchOpts, PatchData: data, PatchType: p}
	output, err = m.queryResources(utils.Patch, []Resource{cmResourcePrj}, dynamicOpts)
	return
}

func (m *Manager) determineRolloutAction(opts RoosterOptions, missingResources []Resource) (rolloutAction string) {
	updateIfExists := opts.UpdateIfExists
	if updateIfExists {
		rolloutAction = "apply-all"
	} else if len(missingResources) != 0 && !updateIfExists {
		rolloutAction = "apply-selective"
	}
	return
}

func (m *Manager) applyRolloutAction(action, manifestPath, namespace string, resources []Resource, ignoreResources, dryRun bool) (err error) {
	logger := m.kcm.Logger
	logger.Sugar().Infof("ACTION: %s", action)
	if ignoreResources {
		logger.Warn("Resources are ignored. Skipping resource creation.")
		return
	}
	deleteOpts := utils.MakeDeleteOptions(dryRun)
	dynamicOpts := utils.DynamicQueryOptions{DeleteOptions: deleteOpts}
	if strings.EqualFold(action, "apply-all") {
		// make sure the latest version will be deployed by removing the old ones first
		_, err = m.queryResources(utils.Delete, resources, dynamicOpts)
		if err != nil {
			return err
		}
		logger.Info("Resources deletion is now complete.")
	}
	err = deployResources(logger, manifestPath, namespace, dryRun)
	if err != nil {
		return err
	}
	return
}

func (m *Manager) getMissingResources(targetResources []Resource) (missingResources []Resource, err error) {
	missingResources = []Resource{}
	for _, currRes := range targetResources {
		_, err := m.kcm.GetResourcesDynamically(currRes.ApiVersion, currRes.Kind, currRes.Namespace, currRes.Name, meta_v1.GetOptions{})
		if k8s_errors.IsNotFound(err) {
			rs := Resource{
				ApiVersion: currRes.ApiVersion,
				Kind:       currRes.Kind,
				Name:       currRes.Name,
				Namespace:  currRes.Namespace,
				Manifest:   currRes.Manifest,
			}
			missingResources = append(missingResources, rs)
			continue
		}
		if err != nil {
			return missingResources, err
		}
	}
	return
}

func waitForResources(duration time.Duration) {
	time.Sleep(duration)
}

func (m *Manager) verifyResourcesStatus(ignoreResources bool, targetResources []Resource) (err error) {
	logger := m.kcm.Logger
	if ignoreResources {
		logger.Warn("Resources are ignored. Skipping resources status check.")
		return
	}
	resourceReport, err := m.areResourcesReady(targetResources)
	if err != nil {
		return
	}
	if len(resourceReport) == 0 {
		// TODO_1b the next line won't be needed anymore once areResourcesReady is improved.
		// A more explicit error should be returned
		err = errors.New("resources readiness could not be defined")
		return
	}
	for _, rs := range resourceReport {
		if !rs.Ready {
			return errors.New("issues encountered with the " + rs.Kind + " " + rs.Name)
		}
	}
	return
}

func (m *Manager) areResourcesReady(targetResources []Resource) (resourcesStatus []Resource, err error) {
	logger := m.kcm.Logger
	logger.Info("Waiting for resources to be ready...")
	waitForResources(20 * time.Second)
	resourcesStatus = []Resource{}
	rs := Resource{}
	// 0 for the verb GET
	dynamicOpts := utils.DynamicQueryOptions{GetOptions: meta_v1.GetOptions{}}
	resources, err := m.queryResources(utils.Get, targetResources, dynamicOpts)
	if err != nil {
		return
	}
	for _, kubernetesResource := range resources {
		k8sObject := kubernetesResource.Object
		kind := k8sObject["kind"].(string)
		name := k8sObject["metadata"].(map[string]interface{})["name"].(string)
		namespace := k8sObject["metadata"].(map[string]interface{})["namespace"].(string)
		status := make(map[string]interface{})
		logger.Info("Found " + kind + " " + name)
		if kind == "DaemonSet" {
			status = k8sObject["status"].(map[string]interface{})
		}
		rs.Name = name
		rs.Kind = kind
		rs.Namespace = namespace
		ready, err := m.checkResourceStatus(kind, status, rs)
		if err != nil {
			return resourcesStatus, err
		}
		rs.Ready = ready
		resourcesStatus = append(resourcesStatus, rs)
	}
	return resourcesStatus, err
}

func (m *Manager) checkResourceStatus(kind string, status map[string]interface{}, rs Resource) (result bool, err error) {
	switch kind {
	case "DaemonSet":
		ready, err := utils.CheckDaemonSetStatus(status)
		if err != nil {
			return ready, err
		}
	default:
		// do nothing particular
	}
	return true, err
}

/**
* Ensures the following:
* Nodes with the target label must exist
* If no node is found, bail
* Nodes with the canary labels should not exist
* If some are found, the choice to continue the rollout is given to the user
**/
func (m *Manager) verifyLabelValidity(label string, shouldExist string) (output bool, err error) {
	// Review nodes labels
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = label
	nodes, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	expectedStatus, _ := strconv.ParseBool(shouldExist)
	// Nodes with the canary label were found, but it may be okay to carry on
	if !expectedStatus && len(nodes) > 0 {
		decision := utils.IndicateNextAction()
		if !decision {
			err = errors.New("user cancelled action")
		}
		return decision, err
	}
	// No nodes carrying the target label were found. Abort!
	if expectedStatus && len(nodes) == 0 {
		err = errors.New("no node carrying the target label was found")
		return
	}
	output = true
	return
}

/**
* GOAL: Rollout resources onto a given cluster
* Will:
* - Determine if some of the indicated resources are already in the cluster
* - Create the missing resource (Will apply to all if none was found)
* - Label the nodes with the canary/control label
* - Make sure the resources are ready
* - Run the indicated tests
**/
func (m *Manager) performRollout(rolloutOpts RoosterOptions) (backupDirectory string, err error) {
	dryRun := rolloutOpts.DryRun
	manifestPath := rolloutOpts.ManifestPath
	namespace := rolloutOpts.Namespace
	resourcesToDeploy := rolloutOpts.Resources
	clusterID := rolloutOpts.ClusterID
	projectOptions := rolloutOpts.ProjectOpts
	testBinary := rolloutOpts.TestBinary
	testSuite := rolloutOpts.TestSuite
	rolloutNodes := rolloutOpts.RolloutNodes
	controlLabel := rolloutOpts.CanaryLabel
	logger := m.kcm.Logger
	ignoreResources := rolloutOpts.IgnoreResources
	// Check all the resources. See if they are in the cluster
	missingResources, err := m.getMissingResources(resourcesToDeploy)
	if err != nil {
		return backupDirectory, err
	}
	logger.Sugar().Infof("Missing resource: %t", len(missingResources) > 0)
	rolloutAction := m.determineRolloutAction(rolloutOpts, missingResources)
	// If none of the resources to deploy are available, no need to take a backup
	// If resources should be ignored, no need to take a backup
	if len(resourcesToDeploy) > len(missingResources) && !ignoreResources {
		logger.Info("Backing up resources...")
		// Back up existing resources
		backupDirectory, err = backupResources(logger, resourcesToDeploy, clusterID, projectOptions, ignoreResources)
		if err != nil {
			return backupDirectory, err
		}
	}
	switch rolloutAction {
	case "apply-all":
		err = m.applyRolloutAction(rolloutAction, manifestPath, namespace, resourcesToDeploy, ignoreResources, dryRun)
		if err != nil {
			return
		}
	case "apply-selective":
		// Get manifest of missing resource
		// Only create missing resources
		for _, rs := range missingResources {
			myRs := []Resource{rs}
			logger.Info("Creating missing " + rs.Kind + " " + rs.Name + ", in namespace: " + rs.Namespace)
			err = m.applyRolloutAction(rolloutAction, rs.Manifest, rs.Namespace, myRs, ignoreResources, dryRun)
			if err != nil {
				return backupDirectory, err
			}
		}
	}
	// patch nodes
	err = m.incrementalNodePatch(rolloutNodes, controlLabel, dryRun, true)
	if err != nil {
		return backupDirectory, err
	}
	if dryRun {
		logger.Info("Dry run operation. No errors encountered")
		return
	}
	// Check if all resources are ready
	err = m.verifyResourcesStatus(ignoreResources, resourcesToDeploy)
	if err != nil {
		return backupDirectory, err
	}
	// Run the tests
	err = runTests(logger, testSuite, testBinary)
	if err != nil {
		logger.Warn("Tests have failed.")
		return backupDirectory, err
	}
	return backupDirectory, err
}

/**
* Will get the current version for the given project.
* Two sources of truth are required to match
* Source of truth 1: The data of type CmData, reflected by the configmap <str-versioning-cache-PROJECT>
* Source of truth 2: The labels on the nodes in the current cluster
* Will:
* - Get the current version based off the given data
* - Get the nodes carrying the label specifying the current version
* - Ensure the nodes listed in the CM and the labeled ones match
**/
func (m *Manager) getCurrentVersion(project string, cmdata utils.CmData) (version string, err error) {
	logger := m.kcm.Logger
	// get the current version and the nodes it touches
	vd := m.ExtractCurrentVersionDetails(project, cmdata)
	if len(vd) > 1 {
		err = errors.New("No more than one version can be current for project " + project)
		return
	}
	nodes := []string{}
	// vrs: current version
	// n: node names
	for vrs, n := range vd {
		version = vrs
		nodes = strings.Split(n, ",")
	}
	// get nodes marked with deploy.streamliner.<PROJECT>=<CURRENT_VERSION>
	markedNodes, err := m.getMarkedNodes(project, version)
	if err != nil {
		return
	}
	// make sure the nodes in resources and the ones in the cm data are the same
	sort.Slice(markedNodes, func(i, j int) bool {
		return markedNodes[i] < markedNodes[j]
	})
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})
	if !reflect.DeepEqual(markedNodes, nodes) {
		err = errors.New("versions from the cache and labels do not match. Cache drift detected")
		logger.Sugar().Infof("markedNodes: %v\n", markedNodes)
		logger.Sugar().Infof("cm saved nodes: %v\n", nodes)
		return
	}
	return
}

/**
* Will get the nodes carrying the label specifying the given project and version
**/
func (m *Manager) getMarkedNodes(project, version string) (markedNodes []string, err error) {
	// compose the selector
	_, versionSelector := utils.MakeVersionLabel(STREAMLINER_LBL_PREFIX, project, version)
	listOpts := meta_v1.ListOptions{
		LabelSelector: versionSelector,
	}
	nodes, err := m.getNodes(listOpts)
	markedNodes = []string{}
	for _, n := range nodes {
		markedNodes = append(markedNodes, n.GetName())
	}
	return
}

/**
* Will label the given resources with the project name and running version
**/
func (m *Manager) applyVersionPatch(resources []Resource, projectOptions ProjectOptions, dryRun bool) (err error) {
	logger := m.kcm.Logger
	p := types.JSONPatchType
	prj := projectOptions.Project
	vrs := projectOptions.DesiredVersion
	patchOpts := utils.MakePatchOptions(dryRun)
	labelKey, _ := utils.MakeVersionLabel(STREAMLINER_LBL_PREFIX, prj, vrs)
	labels := make(map[string]string)
	labels[labelKey] = vrs
	op := "replace"
	data, err := utils.MakePatchData(labelPrefix, op, labels)
	if err != nil {
		return err
	}
	dynamicOpts := utils.DynamicQueryOptions{PatchOptions: patchOpts, PatchData: data, PatchType: p}
	_, err = m.queryResources(utils.Patch, resources, dynamicOpts)
	logger.Info("Version patch effective")
	return
}

func (m *Manager) calBatchSize(nodeList core_v1.NodeList, canaryOrIncrement int) (nodes []core_v1.Node, batchSize float64) {
	logger := m.kcm.Logger
	// Set the batch size
	logger.Info("Defining batch size...")
	if canaryOrIncrement > 100 {
		logger.Warn("Batch size cannot be defined. Invalid canary/increment")
		return
	}
	switch len(nodeList.Items) {
	case 0:
		batchSize = 0
		nodes = nil
	case 1:
		batchSize = 1
		nodes = nodeList.Items
	default:
		batchSize = math.Round(float64(len(nodeList.Items)*canaryOrIncrement) / 100)
		nodes = nodeList.Items[:int(batchSize)]
		logger.Sugar().Infof("Targeted nodes count: %g/%d", batchSize, len(nodeList.Items))
	}
	return
}

func (m *Manager) getNodes(listOpts meta_v1.ListOptions) (targetNodes []unstructured.Unstructured, err error) {
	dynamicOpts := utils.DynamicQueryOptions{
		ListOptions: listOpts,
	}
	return m.queryResources(utils.List, []Resource{dummyNode}, dynamicOpts)
}

/**
* Receives a project name and a struct of type CmData
* Extracts the old versions for the given project.
* Will return a map comprised of the following:
* - key: a previous version
* - value: the nodes onto which that version should be deployed based of the given CmData
**/
func (m *Manager) ExtractPreviousVersionDetails(project string, cmdata utils.CmData) (oldVersionDetails map[string]string) {
	oldVersionDetails = make(map[string]string)
	emptyNodes := []string{""}
	d := cmdata.Data
	for _, pii := range d.Info {
		if curr, _ := strconv.ParseBool(pii.Current); !curr && !reflect.DeepEqual(emptyNodes, pii.Nodes) {
			oldVersionDetails[pii.Version] = strings.Join(pii.Nodes, ",")
		}
	}
	return
}

// Ensures previous versions are all inactive (deployed on no node)
func (m *Manager) CheckPreviousVersions(cmdata utils.CmData, projectName, action string) error {
	previousVersions := m.ExtractPreviousVersionDetails(projectName, cmdata)
	for versionName, nodes := range previousVersions {
		if len(nodes) > 0 {
			return fmt.Errorf("previous version [%s] is registered as currently being rolled out. %s failed", versionName, action)
		}
	}
	return nil
}

// Ensures the current version is fully rolled out
func (m *Manager) CheckCurrentVersion(cmdata utils.CmData, projectName, targetLabel, action string) (err error) {
	// get nodes flagged with the target label
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	unstructNodes, err := m.getNodes(customOptions)
	if err != nil {
		return err
	}
	targetNodes := utils.MakeNodeList(unstructNodes)
	vd := m.ExtractCurrentVersionDetails(projectName, cmdata)
	if len(vd) != 1 {
		return fmt.Errorf("number of supported current version for project %s is 1. %s failed", projectName, action)
	}
	nodesWithCurrVrs := []string{}
	currVrs := ""
	// vrs: current version
	// n: node names
	for vrs, n := range vd {
		currVrs = vrs
		nodesWithCurrVrs = strings.Split(n, ",")
	}
	targetNodeNames := utils.MakeNodeNames(targetNodes)
	sort.Slice(nodesWithCurrVrs, func(i, j int) bool {
		return nodesWithCurrVrs[i] < nodesWithCurrVrs[j]
	})
	sort.Slice(targetNodeNames, func(i, j int) bool {
		return targetNodeNames[i] < targetNodeNames[j]
	})
	// what we want: nodes with current version = given targetNodes (All nodes with control/canary label)
	if !reflect.DeepEqual(nodesWithCurrVrs, targetNodeNames) {
		err = fmt.Errorf("the current version %s isn't fully rolled out. %s failed", currVrs, action)
	}
	return
}

/**
* Receives a project name and a struct of type CmData
* Determines the current version of a given project based off the also given data.
* Will return a map comprised of the following:
* - key: the current version
* - value: the nodes onto which that version should be deployed based of the given CmData
**/
func (m *Manager) ExtractCurrentVersionDetails(project string, cmdata utils.CmData) (versionDetails map[string]string) {
	d := cmdata.Data
	versionDetails = make(map[string]string)
	for _, pii := range d.Info {
		if curr, _ := strconv.ParseBool(pii.Current); curr {
			versionDetails[pii.Version] = strings.Join(pii.Nodes, ",")
		}
	}
	return
}

/**
* Will remove the last-applied-config annotation from indicated resources
**/
func (m *Manager) RemoveLastAppliedAnnotation(resources []Resource, dryRun bool) (err error) {
	if len(resources) == 0 {
		m.kcm.Logger.Warn("Resources are ignored. Skipping config patch.")
		return
	}
	dynamicOpts := utils.DynamicQueryOptions{}
	op := "remove"
	annotationPrefix := "/metadata/annotations/"
	p := types.JSONPatchType
	// ~1 for /
	lastConfig := "kubectl.kubernetes.io~1last-applied-configuration="
	annotation := utils.SplitLabel([]string{lastConfig})
	patchData, err := utils.MakePatchData(annotationPrefix, op, annotation)
	if err != nil {
		return
	}
	patchOpts := utils.MakePatchOptions(dryRun)
	dynamicOpts.PatchData = patchData
	dynamicOpts.PatchOptions = patchOpts
	dynamicOpts.PatchType = p
	// get the obj, delete the last-applied-config (use patch for that)
	_, err = m.queryResources(utils.Patch, resources, dynamicOpts)
	return
}

func (m *Manager) DefineTargetNodes(opts RoosterOptions) (targets core_v1.NodeList, err error) {
	targetNodes := opts.NodesWithTargetlabel
	canaryLabel := opts.CanaryLabel
	projectOptions := opts.ProjectOpts
	// Not using getMarkedNodes because the selector is different
	// To get nodes marked with the desired version label
	_, versionSelector := utils.MakeVersionLabel(STREAMLINER_LBL_PREFIX, projectOptions.Project, projectOptions.DesiredVersion)
	customOptions := meta_v1.ListOptions{}
	// To get nodes that have a version and canary/control labels
	customOptions.LabelSelector = strings.Join([]string{canaryLabel, versionSelector}, ",")
	nodesWithControlLabel, err := m.getNodes(customOptions)
	if err != nil {
		return
	}
	canaryNodes := utils.MakeNodeList(nodesWithControlLabel)
	return utils.ExtractUncommonNodes(targetNodes, canaryNodes), nil
}
