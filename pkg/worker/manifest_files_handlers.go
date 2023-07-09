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
	"io"
	"os"
	"strings"

	"rooster/pkg/config"
	"rooster/pkg/utils"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func ReadManifestFiles(logger *zap.Logger, manifestPath string, indicatedNamespace string) (objectReference []Resource, err error) {
	logger.Info("Reading from " + manifestPath)
	if !strings.HasSuffix(manifestPath, "/") {
		manifestPath = manifestPath + "/"
	}
	// navigate to the indicated folder
	files, err := os.ReadDir(manifestPath)
	if err != nil {
		return
	}
	for _, file := range files {
		myResource := Resource{}
		data := basicK8sConfiguration{}
		myResource.Manifest = manifestPath + file.Name()
		fileInfo, err := os.Stat(manifestPath + file.Name())
		if err != nil {
			return nil, err
		}
		if fileInfo.Size() == 0 {
			logger.Warn(file.Name() + " is empty")
			continue
		}
		f, err := os.Open(manifestPath + file.Name())
		if err != nil {
			return nil, err
		}
		d := yaml.NewDecoder(f)
		for {
			// pass a config reference to data
			err := d.Decode(&data)
			if data.Metadata.Name == "" {
				continue
			}
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			namespaceInManifest := data.Metadata.Namespace
			ns, err := utils.DetermineNamespace(namespaceInManifest, indicatedNamespace)
			if err != nil {
				return nil, err
			}
			myResource.ApiVersion = data.ApiVersion
			myResource.Kind = data.Kind
			myResource.Name = data.Metadata.Name
			myResource.Namespace = ns
			myResource.UpdateStrategy = data.Spec.UpdateStrategy.StrategyType
			objectReference = append(objectReference, myResource)
		}
	}
	return objectReference, err
}

func backupResources(logger *zap.Logger, targetResources []Resource, cluster string, projectOptions ProjectOptions, ignoreResources bool) (backupDirFullName string, err error) {
	backupDir := config.Env.BackupDirectory
	projectName := projectOptions.Project
	currentVersion := projectOptions.CurrVersion
	if ignoreResources {
		logger.Warn("Resources are ignored. Skipping backup operation.")
		return
	}
	if backupDir == "" {
		return "", errors.New("backup directory not found")
	}
	if len(targetResources) == 0 {
		return backupDirFullName, errors.New("no resources to back up")
	}
	nameComponents := []string{backupDir, cluster, projectName, currentVersion}
	backupDirFullName = strings.Join(nameComponents, "/")
	// TODO: do I need this?
	// if found := CheckDirectoryExistence(backupDirFullName); found {
	// 	err = errors.New("version backup already found")
	// }
	if err = os.MkdirAll(backupDirFullName, os.ModePerm); err != nil {
		return
	}
	logger.Info("Created backup directory at " + backupDirFullName)
	for _, currRes := range targetResources {
		fileName := backupDirFullName + "/" + currRes.Kind + "_" + currRes.Name + ".yaml"
		cmd, err := utils.KubectlEmulator(currRes.Namespace, "get", currRes.Kind, currRes.Name, "--ignore-not-found=true -oyaml>"+fileName)
		if err != nil {
			// cmd is the command itself
			logger.Info(cmd)
			return "", err
		}
	}
	logger.Info("Resource backup complete.")
	return
}

func CheckDirectoryExistence(path string) (exists bool) {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		exists = true
	}
	return
}
