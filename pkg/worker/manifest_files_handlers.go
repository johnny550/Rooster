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

	"rooster/pkg/config"
	"rooster/pkg/utils"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

func readmanifestFiles(logger *zap.Logger, manifestPath string, indicatedNamespace string) (objectReference map[string]string) {
	// map of kind,name: namespace ---- Service,kube-dns-upstream:kube-system
	objectReference = make(map[string]string)
	// navigate to the indicated file
	files, err := os.ReadDir(manifestPath)
	if err != nil {
		logger.Error(err.Error())
	}
	for _, file := range files {
		data := basicK8sConfiguration{}
		logger.Info("Reading file: " + file.Name())
		f, err := os.Open(manifestPath + file.Name())
		if err != nil {
			logger.Error(err.Error())
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
				logger.Panic(err.Error())
			}
			kind := data.Kind
			name := data.Metadata.Name
			namespace := data.Metadata.Namespace
			ns, err := determineNamespace(namespace, indicatedNamespace)
			if err != nil {
				logger.Panic(err.Error())
			}
			objectReference[kind+","+name] = ns
		}
	}
	return objectReference
}

func backupResources(logger *zap.Logger, targetResources map[string]string) (OpComplete bool, backupDir string) {
	backupDir = config.Env.BackupDirectory
	if backupDir == "" {
		return
	}
	if err := os.Mkdir(backupDir, os.ModePerm); err != nil {
		if !errors.Is(err, os.ErrExist) {
			logger.Error(err.Error())
			return
		}
		logger.Warn(err.Error())
	}
	logger.Info("Created backup directory at " + backupDir)
	for kindName, namespace := range targetResources {
		kind := getAttribute(kindName, 0)
		name := getAttribute(kindName, 1)
		fileName := backupDir + "/" + kind + "_" + name + ".yaml"

		cmd, err := utils.Kubectl(namespace, "get", kind, name, "-oyaml>"+fileName)
		if err != nil {
			logger.Error(cmd)
			return
		}
	}
	OpComplete = true
	logger.Info("Resource backup complete.")
	return
}

func checkDirectoryExistence(path string) (exists bool) {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		exists = true
	}
	return
}
