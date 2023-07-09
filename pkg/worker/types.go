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
	"rooster/pkg/config"
	"rooster/pkg/utils"

	core_v1 "k8s.io/api/core/v1"
)

type basicK8sConfiguration struct {
	ApiVersion string           `yaml:"apiVersion"`
	Kind       string           `yaml:"kind"`
	Metadata   basicK8sMetadata `yaml:"metadata"`
	Spec       basicK8sSpec     `yaml:"spec"`
}

type basicK8sMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type basicK8sSpec struct {
	ServiceAccountName string           `yaml:"serviceAccountName"`
	UpdateStrategy     dsUpdateStrategy `yaml:"updateStrategy"`
}

type dsUpdateStrategy struct {
	StrategyType string `yaml:"type"`
}

type Manager struct {
	kcm utils.K8sClientManager
}

type ProjectOptions struct {
	CurrVersion    string // Version for resources currently deployed for the project
	DesiredVersion string // Desired version for deployed resources. Could be the next or previous one
	Project        string // Project name
}

type Resource struct {
	ApiVersion     string
	Kind           string
	Manifest       string
	Name           string
	Namespace      string
	Ready          bool
	UpdateStrategy string
}

type RoosterOptions struct {
	Action               string           // Action to perform. A rollout, a rollback, scale down, or update?
	BatchSize            float64          // Number of nodes onto which to rollout
	Canary               int              // Canary batch size. In percentage
	CanaryLabel          string           // Label to put on nodes to control the canary process
	ClusterID            string           // Current cluster ID
	Decrement            int              // Rollback increment
	DryRun               bool             // Dry run
	IgnoreResources      bool             // To ignore creating, verifying resources after an action is complete, or while it is being completed
	Increment            int              // Rollout increment over time. In percentage
	ManifestPath         string           // Path to the manifests to perform a canary release for
	Namespace            string           // Targeted namespace
	NodesWithTargetlabel core_v1.NodeList // Nodes carrying the indicated target label
	ProjectOpts          ProjectOptions   // Project name, current & desired versions
	Resources            []Resource       // Resources to rollout
	RolloutNodes         []core_v1.Node   // Nodes onto which to rollout
	Strategy             string           // Indicated rollout strategy
	TargetLabel          string           // Label identifying the nodes in the cluster
	TestSuite            string           // Test suite name
	TestBinary           string           // Test binary name
	UpdateIfExists       bool             // Update existing resources
}

const (
	annotationPrefix = "/metadata/annotations/"
	cmDataPrefix     = "/data/"
	labelPrefix      = "/metadata/labels/"
)

var (
	apiVersionCoreV1       = config.Env.ApiVersionCoreV1
	cmKind                 = config.Env.CmKind
	cmName                 = config.Env.CmName
	cmNamespace            = config.Env.DefaultNamespace
	nodeKind               = config.Env.NodeKind
	STREAMLINER_LBL_PREFIX = config.Env.LabelPrefix

	cmResource = Resource{
		ApiVersion: apiVersionCoreV1,
		Kind:       cmKind,
		Name:       cmName,
		Namespace:  cmNamespace,
	}
	dummyNode = Resource{
		ApiVersion: apiVersionCoreV1,
		Kind:       nodeKind,
	}
)
