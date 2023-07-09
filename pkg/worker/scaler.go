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
	"errors"

	"rooster/pkg/utils"
)

/**
* Goal: Reduces the scope of the already deployed resources
* Will
* - define batch size
* - unlabel those nodes
* - update cm
* Basically, cleanResources but without patching all the nodes that have been deployed on at once
**/
func ScaleDown(kubernetesClientManager *utils.K8sClientManager, opts RoosterOptions) (err error) {
	// Manager settings
	m, _ := newManager(kubernetesClientManager)
	// make sure the decrement is indicated
	decrement := opts.Decrement
	if decrement < 1 {
		return errors.New("wrong decrement indicated")
	}
	// Limitation: scaling down a version that isn't current is forbidden
	projectOpts := opts.ProjectOpts
	versionToScale := projectOpts.DesiredVersion
	project := projectOpts.Project
	if versionToScale != "" {
		cmResourcePrj := makeCMName(project)
		// get the cm and extract its content
		cmdata, queryErr := m.retrieveConfigMapContent(cmResourcePrj)
		if err != nil {
			return queryErr
		}
		// get the current version
		v, vErr := m.getCurrentVersion(project, cmdata)
		if vErr != nil {
			return vErr
		}
		if v != versionToScale {
			return errors.New("cannot scale down a version that is not current")
		}
	}
	return m.cleanResources(opts)
}
