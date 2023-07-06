package main

import (
	"errors"
	"flag"
	"fmt"
	"runtime"
	"strings"

	"rooster/pkg/config"
	"rooster/pkg/utils"
	"rooster/pkg/worker"

	"go.uber.org/zap"
)

func printVersion(logger *zap.Logger) {
	logger.Sugar().Infof("Go Version: %s", runtime.Version())
	logger.Info("Go OS/Arch: " + runtime.GOOS + "/" + runtime.GOARCH)
	logger.Info("Deployer version: " + config.Env.DeployerVersion)
}

// https://pkg.go.dev/flag#hdr-Command_line_flag_syntax
func gatherOptions() (dryRun bool, manifestPath string, targetLabel string, canaryLabel string, canary int, namespace string, testPackage string, testBinary string, clusterName string, strategy string, updateIfExists bool, increment int) {
	flag.BoolVar(&dryRun, "dry-run", false, "dry-run usage")
	flag.StringVar(&manifestPath, "manifest-path", "", "Path to the manifests to perform a canary release for")
	flag.StringVar(&targetLabel, "target-label", "", "Existing label on nodes to target")
	flag.StringVar(&canaryLabel, "canary-label", "", "Label to put on nodes to control the canary process")
	flag.IntVar(&canary, "canary", 0, "Canary batch size. In percentage")
	flag.StringVar(&namespace, "namespace", "", "Targeted namespace")
	flag.StringVar(&testPackage, "test-package", "", "Test package name")
	flag.StringVar(&testBinary, "test-binary", "", "Test binary name")
	flag.StringVar(&clusterName, "cluster-name", "", "Current cluster name")
	flag.StringVar(&strategy, "strategy", "", "Desired rollout strategy. Canary | linear")
	flag.BoolVar(&updateIfExists, "update-if-exists", true, "Update existing resources")
	flag.IntVar(&increment, "increment", 0, "Rollout increment over time. In percentage")
	flag.Parse()
	return
}

func printOptions(manifestPath string, dryRun bool, targetLabel string, canaryLabel string, canary int, namespace string, testPackage string, testBinary string, clusterName string, strategy string, updateIfExists bool, increment int, logger *zap.Logger) {
	logger.Info("Rollout strategy: " + strings.ToUpper(strategy))
	logger.Info("Cluster name: " + clusterName)
	logger.Info("Namespace: " + namespace)
	// linear
	if strings.EqualFold(strategy, "linear") {
		logger.Sugar().Infof("Rollout increment: %d", increment)
	}
	// canary
	if strings.EqualFold(strategy, "canary") {
		logger.Sugar().Infof("Canay batch size: %d", canary)
		logger.Info("Canary-label:" + canaryLabel)
	}
	logger.Sugar().Infof("Update existing resources: %t ", updateIfExists)
	logger.Info("Manifest path: " + manifestPath)
	logger.Info("Target label: " + targetLabel)
	logger.Info("Test package name: " + testPackage)
	logger.Info("Test binary name: " + testBinary)
	logger.Sugar().Infof("dry-run: %t", dryRun)
}

func createClientManager(kubeconfigPath string) (cm *utils.K8sClientManager, err error) {
	return utils.New(kubeconfigPath)
}

func main() {
	kubernetesClientManager, err := createClientManager("")
	if err != nil {
		panic(err)
	}
	logger := kubernetesClientManager.Logger
	defer logger.Sync()
	printVersion(logger)
	dryRun, manifestPath, targetLabel, canaryLabel, canary, namespace, testPackage, testBinary, clusterName, strategy, updateIfExists, increment := gatherOptions()
	// Omitted fields
	// Batchsize, & RolloutNodes for now. They will be set by each rollout entry func
	// Resources & Namespace will be set later on in this same func
	// NodesWithTargetLabel. It will be set in ProceedToDeployment
	rolloutOptions := worker.RolloutOptions{
		Strategy:    strategy,
		ClusterName: clusterName,
		// Namespace:      namespace,
		ManifestPath:   manifestPath,
		CanaryLabel:    canaryLabel,
		TargetLabel:    targetLabel,
		Increment:      increment,
		Canary:         canary,
		TestPackage:    testPackage,
		TestBinary:     testBinary,
		UpdateIfExists: updateIfExists,
		DryRun:         dryRun,
	}
	targetResources, err := performPreflightCheck(logger, rolloutOptions)
	if err != nil {
		logger.Fatal(err.Error())
	}
	for _, r := range targetResources {
		ns, err := worker.DetermineNamespace(r.Namespace, namespace)
		if err != nil {
			logger.Fatal(err.Error())
		}
		// update namespace. useful if the option wasn't indicated
		namespace = ns
		// limitation: Assume all manifest files point towards the same namespace. Will be improved
		break
	}
	// Set the target resources & namespace
	rolloutOptions.Resources = targetResources
	rolloutOptions.Namespace = namespace
	printOptions(manifestPath, dryRun, targetLabel, canaryLabel, canary, namespace, testPackage, testBinary, clusterName, strategy, updateIfExists, increment, logger)
	backupDir, err := worker.ProceedToDeployment(kubernetesClientManager, rolloutOptions)
	if err == nil {
		return
	}
	logger.Error(err.Error())
	backupDirPrefix := config.Env.BackupDirectory
	if !strings.HasPrefix(backupDir, backupDirPrefix) {
		logger.Fatal("The rollout failed before the backup of resources configuration could be secured.")
	}
	revertResources := defineRevertNeed()
	if !revertResources {
		logger.Info("Newly deployed resources are left untouched")
		return
	}
	if !strings.HasSuffix(backupDir, "/") {
		backupDir = backupDir + "/"
	}
	status, err := worker.RevertDeployment(kubernetesClientManager, backupDir, targetLabel, canaryLabel, namespace)
	logger.Sugar().Infof("Revert operation completion status: %t", status)
	if err != nil {
		logger.Fatal(err.Error())
	}
}

func performPreflightCheck(logger *zap.Logger, opts worker.RolloutOptions) (targetResources []worker.Resource, err error) {
	manifestPath := opts.ManifestPath
	targetNamespace := opts.Namespace
	clusterName := opts.ClusterName
	strategy := opts.Strategy
	canary := opts.Canary
	increment := opts.Increment
	canaryLabel := opts.CanaryLabel
	targetLabel := opts.TargetLabel
	testPackage := opts.TestPackage
	testBinary := opts.TestBinary
	// Cluster name
	if clusterName == "" {
		err = errors.New("Please indicate the cluster name")
		return
	}
	currentClusterName, err := getCurrentCluster()
	if err != nil {
		return
	}
	if clusterName != currentClusterName {
		logger.Info("Current cluster name: " + currentClusterName)
		logger.Info("Indicated cluster: " + clusterName)
		err = errors.New("Indicated cluster is different from the current one")
		return
	}
	// Strategy
	if strategy == "" {
		err = errors.New("Please indicate a rollout strategy")
		return
	}
	allowedStrategies := []string{"canary", "linear"}
	if !sliceContains(allowedStrategies, strategy) {
		err = errors.New("Unsupported rollout strategy " + strategy)
		return
	}
	if strings.EqualFold(strategy, "canary") {
		// Canary options
		if canary == 0 {
			err = errors.New("Please indicate the canary size")
			return
		}
		if canary >= 100 {
			err = errors.New("The canary cannot be equal or superior to 100.")
			return
		}
	}
	// Always required, no matter the rollout strategy
	if canaryLabel == "" {
		err = errors.New("Please indicate the canary label")
		return
	}
	if strings.EqualFold(strategy, "linear") {
		// Linear options
		if increment == 0 {
			err = errors.New("Please indicate the rollout increment")
			return
		}
		if increment >= 100 {
			err = errors.New("The increment cannot be equal or superior to 100.")
			return
		}
	}
	// Target label
	if targetLabel == "" {
		err = errors.New("Please indicate the target label")
		return
	}
	// Test options
	if _, err = utils.ValidateTestOptions(testPackage, testBinary); err != nil {
		return
	}
	// The manifest files
	if manifestPath == "" {
		err = errors.New("Please indicate the manifest path")
		return
	}
	targetResources, err = worker.ReadManifestFiles(logger, manifestPath, targetNamespace)
	if err != nil {
		return
	}
	if len(targetResources) < 1 {
		err = errors.New("Target resources could not be determined. Aborting...")
		return
	}
	return targetResources, nil
}

func getCurrentCluster() (output string, err error) {
	output, err = utils.KubectlEmulator("default", "config", "current-context", "|", "cut", "-d", "'-'", "-f1,2,3")
	output = strings.Replace(output, "\n", "", 1)
	return
}

func defineRevertNeed() bool {
	var response string
	fmt.Println("Should Rooster revert the recent changes? (y/n)")
	fmt.Scanln(&response)
	return strings.EqualFold(response, "Y")
}

func sliceContains(s []string, str string) bool {
	for _, v := range s {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}
