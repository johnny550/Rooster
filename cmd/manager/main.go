/*
Copyright 2023 The Streamliner Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"rooster/pkg/config"
	"rooster/pkg/utils"
	"rooster/pkg/worker"

	"go.uber.org/zap"
)

func printVersion(logger *zap.Logger) {
	logger.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	logger.Info("Go OS/Arch: " + runtime.GOOS + "/" + runtime.GOARCH)

	logger.Info("Deployer version: " + config.Env.DeployerVersion)

}

func gatherOptions() (manifestPath string, dryRun bool, targetLabel string, canaryLabel string, canary int, namespace string, testPackage string, testBinary string) {
	flag.BoolVar(&dryRun, "dry-run", false, "dry-run usage")
	flag.StringVar(&manifestPath, "manifest-path", "", "Path to the manifests to perform a canary release for")
	flag.StringVar(&targetLabel, "target-label", "", "Existing label on nodes to target")
	flag.StringVar(&canaryLabel, "canary-label", "", "Label to put on nodes to control the canary process")
	flag.IntVar(&canary, "canary", 0, "Canary batch size. In percentage")
	flag.StringVar(&namespace, "namespace", "", "Targeted namespace")
	flag.StringVar(&testPackage, "test-package", "", "Test package name")
	flag.StringVar(&testBinary, "test-binary", "", "Test binary name")
	flag.Parse()
	return
}

func printOptions(manifestPath string, dryRun bool, targetLabel string, canaryLabel string, canary int, namespace string, testPackage string, testBinary string, logger *zap.Logger) {
	logger.Info("Canay batch size: " + strconv.Itoa(canary))
	logger.Info("Canary-label:" + canaryLabel)
	logger.Info("dry-run: " + strconv.FormatBool(dryRun))
	logger.Info("Manifest path: " + manifestPath)
	logger.Info("Namespace: " + namespace)
	logger.Info("Target label: " + targetLabel)
	logger.Info("Test package name: " + testPackage)
	logger.Info("Test binary name: " + testBinary)
}

func createNewk8sClient(logger *zap.Logger, kubeconfigPath string) (client *utils.K8sClient, err error) {
	return utils.New(kubeconfigPath)
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	printVersion(logger)
	manifestPath, dryRun, targetLabel, canaryLabel, canary, namespace, testPackage, testBinary := gatherOptions()
	printOptions(manifestPath, dryRun, targetLabel, canaryLabel, canary, namespace, testPackage, testBinary, logger)
	kubernetesClient, err := createNewk8sClient(logger, "")
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
	status := worker.ProceedToDeployment(kubernetesClient, logger, manifestPath, dryRun, targetLabel, canaryLabel, canary, namespace, testPackage, testBinary)
	if status {
		return
	}
	revertResources := defineRevertNeed()
	if !revertResources {
		logger.Info("Newly deployed resources are left untouched")
		return
	}
	status = worker.RevertDeployment(kubernetesClient, logger, manifestPath, targetLabel, canaryLabel, namespace)
	logger.Info("Revert operation completion status: " + strconv.FormatBool(status))
}

func defineRevertNeed() bool {
	var response string
	fmt.Println("Should Streamliner revert the recent changes? (y/n)")
	fmt.Scanln(&response)
	return strings.EqualFold(response, "Y")
}
