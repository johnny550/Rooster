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
	"k8s.io/apimachinery/pkg/types"
)

type CmData struct {
	Data ProjectInfo `yaml:"data"`
}

type DynamicQueryOptions struct {
	PatchData     []byte
	PatchType     types.PatchType
	GetOptions    meta_v1.GetOptions
	DeleteOptions meta_v1.DeleteOptions
	PatchOptions  meta_v1.PatchOptions
	UdateOptions  meta_v1.UpdateOptions
	ListOptions   meta_v1.ListOptions
}

type patchStringValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value,omitempty"`
}

type ProjectInfo struct {
	Project string                    `yaml:"project"`
	Info    []ProjectIdentifiableInfo `yaml:"info"`
}

type ProjectIdentifiableInfo struct {
	Version string   `yaml:"version"`
	Current string   `yaml:"current"`
	Nodes   []string `yaml:"nodes"`
}

type ErrDef struct {
	SampleErrors []SamplingErrs
}

type SamplingErrs struct {
	Sampler          string
	ErrorDefinitions map[bool]error
}
