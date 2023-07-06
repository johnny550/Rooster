package main

import (
	"errors"
	"flag"
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
func gatherOptions() (dryRun bool, manifestPath string, targetLabel string, canaryLabel string, canary int, namespace string, testSuite string, testBinary string, clusterID string, strategy string, updateIfExists bool, increment int, action string, project string, version string, decrement int) {
	flag.BoolVar(&dryRun, "dry-run", false, "dry-run usage")
	flag.StringVar(&action, "action", "", "Action to perform. Rollout | Rollback")
	flag.StringVar(&project, "project", "", "Name encompassing resources to handle")
	flag.StringVar(&version, "version", "", "Version to roll resources out/back")
	flag.StringVar(&manifestPath, "manifest-path", "", "Path to the manifests to perform a canary release for")
	flag.StringVar(&targetLabel, "target-label", "", "Existing label on nodes to target")
	flag.StringVar(&canaryLabel, "canary-label", "", "Label to put on nodes to control the canary process")
	flag.IntVar(&canary, "canary", 0, "Canary batch size. In percentage")
	flag.StringVar(&namespace, "namespace", "", "Targeted namespace")
	flag.StringVar(&testSuite, "test-suite", "", "Test suite name")
	flag.StringVar(&testBinary, "test-binary", "", "Test binary name")
	flag.StringVar(&clusterID, "cluster-id", "", "Current cluster ID")
	flag.StringVar(&strategy, "strategy", "", "Desired rollout strategy. Canary | linear")
	flag.BoolVar(&updateIfExists, "update-if-exists", false, "Update existing resources")
	flag.IntVar(&increment, "increment", 0, "Rollout increment over time. In percentage")
	flag.IntVar(&decrement, "decrement", 0, "Rollback increment over time. In percentage")
	flag.Parse()
	return
}

func printOptions(roosterOpts worker.RoosterOptions, logger *zap.Logger) {
	clusterID := roosterOpts.ClusterID
	namespace := roosterOpts.Namespace
	project := roosterOpts.ProjectOpts.Project
	version := roosterOpts.ProjectOpts.DesiredVersion
	action := roosterOpts.Action
	strategy := roosterOpts.Strategy
	updateIfExists := roosterOpts.UpdateIfExists
	canaryLabel := roosterOpts.CanaryLabel
	canary := roosterOpts.Canary
	increment := roosterOpts.Increment
	decrement := roosterOpts.Decrement
	manifestPath := roosterOpts.ManifestPath
	targetLabel := roosterOpts.TargetLabel
	testSuite := roosterOpts.TestSuite
	testBinary := roosterOpts.TestBinary
	dryRun := roosterOpts.DryRun
	ignoreResources := roosterOpts.IgnoreResources
	logger.Sugar().Infof("Cluster ID: %s", clusterID)
	logger.Sugar().Infof("Namespace: %s", namespace)
	logger.Sugar().Infof("Project: %s", project)
	logger.Sugar().Infof("Update existing resources: %t", updateIfExists)
	logger.Sugar().Infof("Skip resource deployment: %t", ignoreResources)
	switch action {
	case "rollout":
		printRolloutOptions(action, strategy, canaryLabel, canary, increment, logger)
	case "rollback":
		printRollbackOptions(action, version, decrement, logger)
	case "update":
		printUpdateOptions(action, version, increment, logger)
	}
	logger.Sugar().Infof("Manifest path: %s", manifestPath)
	logger.Sugar().Infof("Target label: %s", targetLabel)
	logger.Sugar().Infof("Test suite name: %s", testSuite)
	logger.Sugar().Infof("Test binary name: %s", testBinary)
	logger.Sugar().Infof("dry-run: %t", dryRun)
}

func printRolloutOptions(action, strategy, canaryLabel string, canary, increment int, logger *zap.Logger) {
	logger.Sugar().Infof("Action: %s", action)
	logger.Sugar().Infof("Rollout strategy: %s", strategy)
	switch strategy {
	case "linear":
		logger.Sugar().Infof("Linear increment: %d%%", increment)
		logger.Sugar().Infof("Control label: %s", canaryLabel)
	case "canary":
		logger.Sugar().Infof("Canay batch size: %d%%", canary)
		logger.Info("Canary label:" + canaryLabel)
	}
}

func printRollbackOptions(action, version string, decrement int, logger *zap.Logger) {
	logger.Sugar().Infof("Action: %s", action)
	if version != "" {
		logger.Info("Rollback type: ToVersion")
		logger.Sugar().Infof("Rollback target: %s", version)
	} else {
		logger.Info("Rollback type: Complete")
	}
}

func printUpdateOptions(action, version string, increment int, logger *zap.Logger) {
	logger.Sugar().Infof("Action: %s", action)
	logger.Sugar().Infof("Update target: %s", version)
	logger.Sugar().Infof("Update increment: %d%%", increment)
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
	dryRun, manifestPath, targetLabel, canaryLabel, canary, namespace, testSuite, testBinary, clusterID, strategy, updateIfExists, increment, action, project, version, decrement := gatherOptions()
	// If no version is indicated and it is NOT a rollback or a scale down, automatically define one
	v := utils.DefineVersion(version, action)
	// Omitted fields
	// Batchsize, & RolloutNodes for now. They will be set by each rollout entry func
	// Resources & Namespace will be set later on in this same func
	// NodesWithTargetLabel. It will be set in ProceedToDeployment
	prjOptions := worker.ProjectOptions{
		Project:        project,
		DesiredVersion: v,
	}
	// set a default strategy if none is precised
	if strategy == "" {
		strategy = config.Env.DefaultRolloutStrategy
	}
	strOptions := worker.RoosterOptions{
		Strategy:       strings.ToLower(strategy),
		ClusterID:      clusterID,
		Action:         strings.ToLower(action),
		ManifestPath:   manifestPath,
		CanaryLabel:    canaryLabel,
		TargetLabel:    targetLabel,
		Increment:      increment,
		Decrement:      decrement,
		Canary:         canary,
		TestSuite:      testSuite,
		TestBinary:     testBinary,
		UpdateIfExists: updateIfExists,
		Namespace:      namespace,
		DryRun:         dryRun,
		ProjectOpts:    prjOptions,
	}
	manifestIsIndicated, err := performPreflightCheck(logger, strOptions)
	if err != nil {
		logger.Fatal(err.Error())
	}
	switch manifestIsIndicated {
	case false:
		strOptions.IgnoreResources = true
	case true:
		targetResources, ns, err := getResourcesToDeploy(logger, strOptions)
		if err != nil {
			logger.Fatal(err.Error())
		}
		// Set the target resources & namespace
		strOptions.Resources = targetResources
		// In case the namespace was not given as an option, now we set it
		strOptions.Namespace = ns
	}
	printOptions(strOptions, logger)
	switch strings.ToLower(action) {
	case "rollout":
		err = worker.ProceedToDeployment(kubernetesClientManager, strOptions)
	case "rollback":
		err = worker.RevertDeployment(kubernetesClientManager, strOptions)
	case "update":
		err = worker.UpdateRollout(kubernetesClientManager, strOptions)
	case "scale-down":
		err = worker.ScaleDown(kubernetesClientManager, strOptions)
	default:
		err = errors.New("Unknown action " + action)
	}
	if err != nil {
		logger.Fatal(err.Error())
	}
}

func performPreflightCheck(logger *zap.Logger, opts worker.RoosterOptions) (manifestIndicated bool, err error) {
	action := opts.Action
	manifestPath := opts.ManifestPath
	clusterID := opts.ClusterID
	targetLabel := opts.TargetLabel
	testSuite := opts.TestSuite
	testBinary := opts.TestBinary
	strategy := opts.Strategy
	canUpdate := opts.UpdateIfExists
	canary := opts.Canary
	increment := opts.Increment
	canaryLabel := opts.CanaryLabel
	project := opts.ProjectOpts.Project
	version := opts.ProjectOpts.DesiredVersion
	// Cluster ID
	if clusterID == "" {
		err = errors.New("please indicate the cluster ID")
		return
	}
	currentClusterID, err := getCurrentCluster()
	if err != nil {
		return
	}
	if clusterID != currentClusterID {
		logger.Info("Current cluster ID: " + currentClusterID)
		logger.Info("Indicated cluster: " + clusterID)
		err = errors.New("indicated cluster is different from the current one")
		return
	}
	// Target label
	if targetLabel == "" {
		err = errors.New("please indicate the target label")
		return
	}
	// Canary label
	if canaryLabel == "" {
		err = errors.New("please indicate the canary label")
		return
	}
	// Project
	if project == "" {
		err = errors.New("please indicate the project name")
		return
	}
	// Version
	ok, err := utils.VerifyVersion(version)
	if !ok {
		switch err {
		case nil:
			err = errors.New("invalid version. regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?'")
		}
		return
	}
	// Action
	switch action {
	case "rollout":
		if err = validateOptionsByStrategy(strategy, canary, increment); err != nil {
			return
		}
		// Test options
		if _, err = utils.ValidateTestOptions(testSuite, testBinary); err != nil {
			return
		}
	case "update":
		if !canUpdate && manifestPath != "" {
			err = errors.New("update-if-exists cannot be false")
			return
		}
		if err = verifyIncrementCanary(increment, "increment"); err != nil {
			return
		}
	case "rollback":
		if opts.Decrement > 0 {
			err = errors.New("decrement not supported for rollback action. Use a scale-down instead")
			return
		}
	}
	return manifestPath != "", err
}

func validateOptionsByStrategy(strategy string, canary, increment int) (err error) {
	switch strategy {
	case "canary":
		err = verifyIncrementCanary(canary, "canary")
	case "linear":
		err = verifyIncrementCanary(increment, "increment")
	default:
		err = errors.New("please indicate a valid rollout strategy")
	}
	return
}

func verifyIncrementCanary(sampler int, s string) (err error) {
	samplerType := strings.ToLower(s)
	i := make(map[bool]error)
	i[true] = errors.New("please indicate the rollout increment")
	i[false] = errors.New("the increment cannot be equal or superior to 100")
	c := make(map[bool]error)
	c[true] = errors.New("please indicate the canary size")
	c[false] = errors.New("the canary cannot be equal or superior to 100")
	canaryErr := utils.SamplingErrs{
		Sampler:          "canary",
		ErrorDefinitions: c,
	}
	incrementErr := utils.SamplingErrs{
		Sampler:          "increment",
		ErrorDefinitions: i,
	}
	all := utils.ErrDef{}
	all.SampleErrors = append(all.SampleErrors, incrementErr, canaryErr)
	if sampler == 0 || sampler >= 100 {
		for _, se := range all.SampleErrors {
			if se.Sampler == samplerType {
				return se.ErrorDefinitions[sampler == 0]
			}
		}
	}
	return
}

func getCurrentCluster() (output string, err error) {
	output, err = utils.KubectlEmulator("default", "config", "current-context", "|", "cut", "-d", "'-'", "-f1,2,3")
	output = strings.Replace(output, "\n", "", 1)
	return
}

func getResourcesToDeploy(logger *zap.Logger, opts worker.RoosterOptions) (targetResources []worker.Resource, namespace string, err error) {
	manifestPath := opts.ManifestPath
	namespace = opts.Namespace
	targetResources, err = worker.ReadManifestFiles(logger, manifestPath, namespace)
	if err != nil {
		return
	}
	if len(targetResources) < 1 {
		err = errors.New("target resources could not be determined. Aborting")
		return
	}
	for _, r := range targetResources {
		if r.Kind == "DaemonSet" && r.UpdateStrategy != "OnDelete" {
			err = errors.New("unsupported Rollout strategy. OnDelete is expected for daemonsets")
			return
		}
	}
	for _, r := range targetResources {
		ns, err := utils.DetermineNamespace(r.Namespace, namespace)
		if err != nil {
			logger.Fatal(err.Error())
		}
		// update namespace. useful if the option wasn't indicated
		namespace = ns
		// limitation: Assume all manifest files point towards the same namespace. Will be improved
		break
	}
	return
}
