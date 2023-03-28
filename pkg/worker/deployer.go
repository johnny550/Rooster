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
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"rooster/pkg/config"
	"rooster/pkg/utils"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	targetNamespace = "kube-system"
)

type Clients struct {
	utils.K8sClient
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type basicK8sConfiguration struct {
	ApiVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   basicK8sMetadata `json:"metadata"`
}

type basicK8sMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func ProceedToDeployment(kubernetesClient *utils.K8sClient, logger *zap.Logger, manifestPath string, dryRun bool, targetLabel string, canaryLabel string, canary int, targetNamespace string, testPackage string, testBinary string) bool {
	// Client settings
	clients := Clients{}
	clients.K8sClient = *kubernetesClient
	// What to deploy
	targetResources := readmanifestFiles(logger, manifestPath, targetNamespace)
	// Verify the canary label
	if valid := clients.validateCanaryLabel(logger, canaryLabel); !valid {
		return false
	}
	// Where to deploy it
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	targetNodes := clients.getTargetNodes(logger, targetLabel, customOptions)
	canaryTargetNodes, batchSize := defineCanaryBatchSize(logger, targetNodes, canary)
	logger.Info("Patching nodes...")
	patchComplete := clients.patchTargetNodes(logger, canaryTargetNodes, canaryLabel, batchSize, dryRun)
	if !patchComplete {
		logger.Warn("Issues encountered while patching nodes. Aborting...")
		return false
	}
	// make sure the latest version will be deployed by removing the old ones first
	_, err := clients.deletePreviousSettings(logger, targetResources, dryRun, true)
	if err != nil {
		return false
	}
	if dryRun {
		logger.Info("As dry as it gets")
		return true
	}
	err = deployResources(logger, manifestPath)
	if err != nil {
		logger.Error(err.Error())
		return false
	}
	statusReport := clients.areResourcesReady(logger, targetResources)
	if statusReport == nil {
		return false
	}
	for resource, readinessStatus := range statusReport {
		if !readinessStatus {
			kind := getAttribute(resource, 0)
			name := getAttribute(resource, 1)
			logger.Warn("Issues encountered with " + kind + " " + name)
			return false
		}
	}
	// Run the tests
	err = runTests(logger, testPackage, testBinary)
	if err != nil {
		logger.Error(err.Error())
		logger.Warn("Tests have failed.")
		return false
	}
	// Complete the rollout
	otherNodes := defineRestOfNodes(targetNodes, len(canaryTargetNodes))
	logger.Info("Patching remaining nodes...")
	patchComplete = clients.patchTargetNodes(logger, otherNodes, canaryLabel, batchSize, dryRun)
	if !patchComplete {
		logger.Warn("Issues encountered while patching nodes. Aborting...")
		return false
	}
	// Check if all resources are ready after the patch operation
	statusReport = clients.areResourcesReady(logger, targetResources)
	if statusReport == nil {
		return false
	}
	for resource, readinessStatus := range statusReport {
		if !readinessStatus {
			kind := getAttribute(resource, 0)
			name := getAttribute(resource, 1)
			logger.Warn("Issues encountered with " + kind + " " + name)
			return false
		}
	}
	logger.Info("The canary realease is now complete.")
	return true
}

func RevertDeployment(kubernetesClient *utils.K8sClient, logger *zap.Logger, manifestPath string, targetLabel string, canaryLabel string, targetNamespace string) bool {
	// Client settings
	clients := Clients{}
	clients.K8sClient = *kubernetesClient
	// the labels
	canaryLabelElements := strings.Split(canaryLabel, "=")
	canaryLabelKey := canaryLabelElements[0]
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = targetLabel
	targetNodes := clients.getTargetNodes(logger, targetLabel, customOptions)
	for _, targetNode := range targetNodes.Items {
		_, err := clients.removeLabelFromNode(logger, targetNode, targetLabel, canaryLabelKey)
		if err != nil {
			logger.Error(err.Error())
		}
	}
	// The resources
	// Get the new resources
	targetResources := readmanifestFiles(logger, manifestPath, targetNamespace)
	// Get the backup directory
	backupDirectory := config.Env.BackupDirectory
	if backupDirectory == "" {
		logger.Warn("Error when reverting resources. The indicated backup directory could not be found: " + backupDirectory)
		return false
	}
	// delete new resources & redeploy the old ones
	opComplete, err := clients.rollbackToPreviousSettings(logger, targetResources, backupDirectory)
	if err != nil {
		logger.Error(err.Error())
		return opComplete
	}
	// Check if all resources are ready after the patch operation
	statusReport := clients.areResourcesReady(logger, targetResources)
	if statusReport == nil {
		return false
	}
	for resource, readinessStatus := range statusReport {
		if !readinessStatus {
			kind := getAttribute(resource, 0)
			name := getAttribute(resource, 1)
			logger.Warn("Issues encountered with " + kind + " " + name)
			return false
		}
	}
	logger.Info("The canary deployment has failed. All resources were reverted")
	return true
}

func (c Clients) validateCanaryLabel(logger *zap.Logger, canaryLabel string) bool {
	// Get nodes that are already labeled with the indicated caanary label
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = canaryLabel
	nodes := c.getTargetNodes(logger, canaryLabel, customOptions)
	if len(nodes.Items) > 0 {
		decision := indicateNextAction()
		return decision
	}
	return true
}

func indicateNextAction() bool {
	var response string
	fmt.Println("At least one node was found carrying the indicated canary label.")
	fmt.Println("Would you like to continue? (y/n)")
	fmt.Scanln(&response)
	return strings.EqualFold(response, "Y")
}

func (c Clients) removeLabelFromNode(logger *zap.Logger, targetNode core_v1.Node, targetLabel string, labelKey string) (done bool, err error) {
	// Get all the nodes matching the target label
	// customOptions := meta_v1.ListOptions{}
	// customOptions.LabelSelector = targetLabel
	// targetNodes := c.getTargetNodes(logger, targetLabel, customOptions)
	// for _, node := range targetNodes.Items {
	_, err = utils.Kubectl("", "label node "+targetNode.Name+" "+labelKey+"-")
	if err != nil {
		return false, err
	}
	// }
	return true, nil
}

func (c Clients) rollbackToPreviousSettings(logger *zap.Logger, targetResources map[string]string, pathToBackupDirectory string) (bool, error) {
	logger.Info("----Rolling back to the previous settings------")
	// delete the resources that are deployed in the cluster
	_, err := c.deletePreviousSettings(logger, targetResources, false, false)
	if err != nil {
		return false, err
	}
	// deploy the resources that had their config backed up before
	err = deployResources(logger, pathToBackupDirectory)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c Clients) areResourcesReady(logger *zap.Logger, targetResources map[string]string) (resourcesStatus map[string]bool) {
	logger.Info("Waiting for resources to be ready...")
	waitForResources(20 * time.Second)
	resourcesStatus = make(map[string]bool, len(targetResources))
	// 0 for the verb GET
	allResourcesWereFound, resources := c.queryResources(logger, 0, targetResources, false)
	if !allResourcesWereFound {
		logger.Warn("Not all indicated resources were found in the cluster. Aborting....")
		return
	}
	for _, kubernetesResource := range resources {
		k8sObject := kubernetesResource.Object
		kind := k8sObject["kind"].(string)
		name := k8sObject["metadata"].(map[string]interface{})["name"].(string)
		status := make(map[string]interface{})
		logger.Info("Found " + kind + " " + name)
		if kind == "DaemonSet" {
			status = k8sObject["status"].(map[string]interface{})
		}
		ready := checkResourceStatus(logger, kind, status)
		resourcesStatus[kind+","+name] = ready
	}
	return resourcesStatus
}

func checkResourceStatus(logger *zap.Logger, kind string, status map[string]interface{}) (result bool) {
	if kind == "DaemonSet" {
		ready, err := checkDaemonSetStatus(status)
		if err != nil {
			logger.Error(err.Error())
		}
		result = ready
	} else {
		result = true
	}
	return result
}

func checkDaemonSetStatus(dsStatus map[string]interface{}) (ready bool, err error) {
	if dsStatus == nil {
		return false, errors.New("daemonSet status was not retrieved")
	}
	desiredNumberScheduled := dsStatus["desiredNumberScheduled"]
	numberReady := dsStatus["numberReady"]
	return desiredNumberScheduled == numberReady, nil
}

func deployResources(logger *zap.Logger, manifestPath string) (err error) {
	if manifestPath == "" {
		err = errors.New("missing manifest path")
		return
	}
	if exists := checkDirectoryExistence(manifestPath); !exists {
		err = errors.New(manifestPath + ": No such file or directory")
		return
	}
	logger.Info("Deploying resources...")
	logger.Info("Resource path: " + manifestPath)
	// Follow the given path. Deploy the yaml files in there
	_, err = utils.Kubectl(targetNamespace, "apply", manifestPath)
	if err == nil {
		logger.Info("Resources were deployed")
	}
	return
}

func determineNamespace(manifestIndicatedNamespace string, optionIndicatedNamespace string) (finalNamespace string, err error) {
	if manifestIndicatedNamespace == "" {
		finalNamespace = optionIndicatedNamespace
	} else {
		finalNamespace = manifestIndicatedNamespace
	}
	if manifestIndicatedNamespace != optionIndicatedNamespace && optionIndicatedNamespace != "" {
		err = errors.New("!!! Namespace conflict detected !!!" + manifestIndicatedNamespace + " vs " + optionIndicatedNamespace)
	}

	return
}

func (c Clients) patchTargetNodes(logger *zap.Logger, targetNodes []core_v1.Node, canaryLabel string, batchSize float64, dryRun bool) bool {
	ctx := context.TODO()
	// Split the canary label
	cL := strings.Split(canaryLabel, "=")
	canaryLabelKey := cL[0]
	canaryLabelValue := cL[1]
	nodesToRevert := c.ensureCanaryLabelPropagation(logger, canaryLabelKey, canaryLabel)
	logger.Info("Batch size---: " + strconv.Itoa(int(batchSize)))
	// Case 1: More nodes than specified by the canary batch size have the canary label already. This may be a resource update situation
	// E.g: batch size=3. nodes with the canary label in the cluster at this point: 4 or more
	if len(nodesToRevert) > int(batchSize) {
		// iterate in reverse to start with the last nodes
		for i := len(nodesToRevert) - 1; i >= 0; i-- {
			// Remove the canary label from the nodes part of the 2nd batch
			if i == int(batchSize)-1 {
				logger.Info("Reached the batch size limit")
				break
			}
			logger.Info("Removing canary label from " + nodesToRevert[i].Name)
			_, err := c.removeLabelFromNode(logger, nodesToRevert[i], canaryLabel, canaryLabelKey)
			if err != nil {
				logger.Error(err.Error())
				return false
			}
		}
		waitForResources(10 * time.Second)
		return true
	}
	// Case 2: Either no node has the canary label yet, less nodes specified by the canary batch size do
	// E.g: batch size=3. nodes with the canary label in the cluster at this point: 1, or 0
	customPatchOptions := meta_v1.PatchOptions{}
	if dryRun {
		customPatchOptions.DryRun = append(customPatchOptions.DryRun, "All")
	}
	p := types.JSONPatchType
	payload := []patchStringValue{{
		Op:    "replace",
		Path:  "/metadata/labels/" + canaryLabelKey,
		Value: canaryLabelValue,
	}}
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Error(err.Error())
		logger.Info("Operation was aborted")
		return true
	}
	for _, targetNode := range targetNodes {
		// Label the nodes (canary 1st batch) with the canaryLabel
		logger.Info("Node to patch: " + targetNode.Name)
		_, err := c.K8sClient.GetClient().CoreV1().Nodes().Patch(ctx, targetNode.Name, p, data, customPatchOptions)
		if err != nil {
			logger.Error(err.Error())
			return false
		}
	}
	logger.Info("Patching complete")
	return true
}

func waitForResources(duration time.Duration) {
	time.Sleep(duration)
}

func (c Clients) ensureCanaryLabelPropagation(logger *zap.Logger, key string, label string) (canaryLabeledNodes []core_v1.Node) {
	customOptions := meta_v1.ListOptions{}
	customOptions.LabelSelector = label
	nodes := c.getTargetNodes(logger, label, customOptions)
	// Ensure no node already has the canary label
	// If anyone does, return it
	for _, targetNode := range nodes.Items {
		labels := targetNode.Labels
		if labels[key] != "" {
			canaryLabeledNodes = append(canaryLabeledNodes, targetNode)
		}
	}
	return
}

func (c Clients) getTargetNodes(logger *zap.Logger, targetLabel string, customOptions meta_v1.ListOptions) (targetNodes core_v1.NodeList) {
	ctx := context.TODO()
	// Get all the nodes with the indicated label.
	nodeList, err := c.K8sClient.GetClient().CoreV1().Nodes().List(ctx, customOptions)
	if err != nil {
		logger.Error(err.Error())
	}
	if len(nodeList.Items) == 0 {
		logger.Warn("No node labelled \"" + targetLabel + "\" was found. Review the indicated label.")
		return
	}
	targetNodes = *nodeList
	return
}

func defineRestOfNodes(nodeList core_v1.NodeList, NumberOfCanaryNodes int) (otherNodes []core_v1.Node) {
	otherNodes = nodeList.Items[NumberOfCanaryNodes:]
	return
}

func defineCanaryBatchSize(logger *zap.Logger, nodeList core_v1.NodeList, canary int) (canaryTargetNodes []core_v1.Node, batchSize float64) {
	// Deduce the batch size
	logger.Info("Defining batch size...")
	batchSize = math.Round(float64(len(nodeList.Items)*canary) / 100)
	logger.Info("Batch size: " + strconv.Itoa(int(batchSize)) + "/" + strconv.Itoa(len(nodeList.Items)))
	canaryTargetNodes = nodeList.Items[:int(batchSize)]
	return
}

func (c Clients) deletePreviousSettings(logger *zap.Logger, targetResources map[string]string, dryRun bool, backup bool) (backupDir string, err error) {
	if backup {
		logger.Info("Backing up resources")
		completed, backupDirectory := backupResources(logger, targetResources)
		backupDir = backupDirectory
		if !completed {
			logger.Info("Backup failed. Aborting...")
			return
		}
	}
	// 0 for the verb GET
	resourcesExist, _ := c.queryResources(logger, 0, targetResources, dryRun)
	if !resourcesExist {
		logger.Info("Resources were not found. Deletion complete")
		return
	}
	// Delete them
	// 3 for the verb DELETE
	resourcesAreDeleted, _ := c.queryResources(logger, 3, targetResources, dryRun)
	if !resourcesAreDeleted {
		err = errors.New("Issues were encountered while deleting resources. Unchanged resources were backed up at " + backupDir)
		return
	}
	logger.Info("Resources deletion is now complete.")
	return
}
