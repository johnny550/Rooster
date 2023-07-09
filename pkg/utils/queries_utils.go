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
	"reflect"
	"strconv"
	"strings"

	"rooster/pkg/config"

	"gopkg.in/yaml.v2"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/**
* Will compose a ConfigMap object based off the given
* parameters such as the name, namespace, and data
**/
func ComposeConfigMap(ns, name string, cmLabels, cmData map[string]string) (configMap *core_v1.ConfigMap) {
	cmKind := config.Env.CmKind
	apiVersionCoreV1 := config.Env.ApiVersionCoreV1
	configMap = &core_v1.ConfigMap{
		TypeMeta: meta_v1.TypeMeta{
			APIVersion: apiVersionCoreV1,
			Kind:       cmKind,
		},
		ObjectMeta: meta_v1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    cmLabels,
		},
		Data: cmData,
	}
	return
}

/**
* Goal: compose the data to save in a config map
* Will:
* - Create data for just the given project, if nothing was saved into the cm before
* - Combine the new project/version's data with the information previously stored in the cm
**/
func ComposeConfigMapData(action, projectName, projectVersion string, nodeResources []core_v1.Node, previousData CmData) (data map[string]string) {
	data = map[string]string{}
	tempData := CmData{}
	allProjectInfo := ProjectInfo{}
	// re-compose the config map's data
	if reflect.DeepEqual(previousData.Data, allProjectInfo) {
		nodeNames := MakeNodeNames(core_v1.NodeList{Items: nodeResources})
		projectIdentifiableDetails := ProjectIdentifiableInfo{
			Version: projectVersion,
			Current: "true",
			Nodes:   nodeNames,
		}
		allProjectInfo = ProjectInfo{
			Project: projectName,
			Info:    []ProjectIdentifiableInfo{projectIdentifiableDetails},
		}
	} else {
		allProjectInfo = rewriteCMData(action, projectName, projectVersion, nodeResources, previousData)
	}
	tempData.Data = allProjectInfo
	out, err := yaml.Marshal(tempData)
	if err != nil {
		return
	}
	data["Streamfile"] = string(out)
	return
}

/**
* The new data is about one project, its version and the nodes onto which the latter is deployed. Hereafter referred to as Np, Nversion & Nnodes
* The old data is given and is a struct of type CmData.
* Will combine the old and new data in the config map.
* To do so the following steps are taken:
* - Browse the given data, named "previousData"
* - If Nversion is already registered in the config map for the relevant project
*	- the list of nodes is overwritten with Nnodes
* 	- Otherwise, the list of nodes for the other versions is adjusted, and that of Nversion is accepted as-is
* - Add the data for the new project in case it isn't already in the config map's data
* - If a scale-down is on-going, as long as it does not empty the last node in the list of nodes for a version, the CURRENT status is untouched
* - For any other case, the CURRENT status is arranged
**/
func rewriteCMData(action, projectName, projectVersion string, nodeResources []core_v1.Node, previousData CmData) (data ProjectInfo) {
	versions := make(map[string][]string)
	for _, pii := range previousData.Data.Info {
		vrsStat := strings.Join([]string{pii.Version, pii.Current}, ",")
		if projectVersion == pii.Version {
			// following rollout of the same version. INCREMENT
			// add the new version and nodes
			versions[vrsStat] = MakeNodeNames(core_v1.NodeList{Items: nodeResources})
			continue
		}
		// from a version to another. UPDATE
		// & modify the node list in the old version
		// or
		// scale down a version & update the list of nodes
		tempVersion := distributeNodesThroughVersions(projectVersion, nodeResources, pii)
		// Add every previous version's details to the map VERSIONS
		for k, v := range tempVersion {
			versions[k] = v
		}
		// Add the newly updated version's details to the map VERSIONS
		newVrsStat := strings.Join([]string{projectVersion, "true"}, ",")
		versions[newVrsStat] = MakeNodeNames(core_v1.NodeList{Items: nodeResources})
	}
	data = makeDataFromProjectDetails(versions, action, projectName, projectVersion)
	return
}

func setStatus(prj, vrs, currentPrj, currentVrs, isCurrent string) string {
	if prj == currentPrj {
		return strconv.FormatBool(vrs == currentVrs)
	}
	return isCurrent
}

func makeDataFromProjectDetails(versionDetails map[string][]string, action, projectName, projectVersion string) (data ProjectInfo) {
	inf := ProjectIdentifiableInfo{}
	data = ProjectInfo{}
	ref := make(map[string]string)
	data.Project = projectName
	for vk, nodes := range versionDetails {
		emptyNodes := len(nodes) == 0
		// vk's content: <VERSION>,<CURRENT>
		versionStatus := strings.Split(vk, ",")
		vrs := versionStatus[0]
		isCurrent := versionStatus[1]
		// a quick verification to avoid
		// - having a version being repeated
		// - having a version with an empty name because strings.Contains(ref[prj], "")=true
		if strings.Contains(ref[projectName], vrs) {
			continue
		}
		// if scaling down, do not update the CURRENT status, unless all nodes are removed from the NODES list for that version
		// Why: because although we scale a version down, it may still be current
		if action == "scale-down" && !emptyNodes {
			inf.Current = isCurrent
		} else {
			inf.Current = setStatus(projectName, vrs, projectName, projectVersion, isCurrent)
		}
		inf.Nodes = nodes
		inf.Version = vrs
		data.Info = append(data.Info, inf)
		previousVersion := ref[projectName]
		ref[projectName] = strings.Join([]string{previousVersion, vrs}, ",")
	}
	return
}

/**
* Will rearrange the distribution of nodes throughout a current version and a previous one
* A node can only be listed for 1 version at a time.
* Let's version2, & its nodes (A1,B1,C1) are the given, as well as the current version (version1) and its nodes (C1,D1,E1)
* If the next version, versionA is not the current version:
* - version2's nodes should be: A1, B1, & C1
* - version1's nodes should be: D1, & E1
* C1, is won over by the latest version
* Will return a map comprised of the following:
* - key: the current version and Current status (true/false)
* - value: the nodes onto which that version should be deployed
**/
func distributeNodesThroughVersions(nextVersion string, nextNodes []core_v1.Node, currentInfo ProjectIdentifiableInfo) (versionNodes map[string][]string) {
	targets := []string{}
	versionNodes = map[string][]string{}
	// Step 1: from []core_v1.Node to []string containing node names only
	tempNodeNames := MakeNodeNames(core_v1.NodeList{Items: nextNodes})
	// Step 2: from []string of node names to core_v1.NodeList
	// Why 2 steps and not just use ConvertToNodeList(core_v1.NodeList{Items:nextNodes}
	// Because we want to compare 2 NodeLists conatining nodes with only the name attribute. All the rest is cleared to simplify the comparison op
	nodesWithDifferentversion := ConvertToNodeList(tempNodeNames)
	currVersion := currentInfo.Version
	// Same reasoning here.
	currNodes := currentInfo.Nodes
	// Step 1 is skipped because we already have the []string of node names.
	// Straight to step 2. []string to core_v1.NodeList
	nodesWithCurrVersion := ConvertToNodeList(currNodes)
	if currVersion != nextVersion {
		// a node cannot have 2 version at once
		// if a node is in nextNodes, it must be removed from currNodes
		uncommonNodes := ExtractUncommonNodes(nodesWithCurrVersion, nodesWithDifferentversion)
		targets = MakeNodeNames(uncommonNodes)
	} else {
		// Add the new nodes to the current list
		targets = append(targets, tempNodeNames...)
	}
	versionNodes[currVersion+","+currentInfo.Current] = targets
	return
}

/**
* Will extract the data recorded in a given config map containing a Streamfile.
* Returns a formatted record or an error if an issue occurs
**/
func ExtractConfigMapData(cm unstructured.Unstructured) (data CmData, err error) {
	k8sObject := cm.Object
	dataContent := k8sObject["data"].(map[string]interface{})
	if dataContent == nil {
		return
	}
	streamFile := dataContent["Streamfile"]
	if streamFile == nil {
		return
	}
	relevantinfo := streamFile.(string)
	yaml.Unmarshal([]byte(relevantinfo), &data)
	return
}
