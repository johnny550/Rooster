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
	"os"
	"os/exec"

	"rooster/pkg/utils"

	"go.uber.org/zap"
)

func runTests(logger *zap.Logger, testPackage string, testBinary string) (err error) {
	skip, err := utils.ValidateTestOptions(testPackage, testBinary)
	if err != nil {
		return
	}
	if skip {
		logger.Info("Skipping test phase. Only basic resource checks will be performed.")
		return
	}
	logger.Info("Running tests...")
	testExecutable, err := exec.LookPath("pkg/test-binaries/" + testBinary)
	if err != nil {
		return
	}
	if testExecutable == "" {
		err = errors.New("test binary not found")
		return
	}
	// exec command
	cmd := &exec.Cmd{
		Path:   testExecutable,
		Args:   []string{testExecutable, "-test.v", "-test.run", testPackage},
		Stdout: os.Stdout,
		Stderr: os.Stdout,
	}
	logger.Info("Command: " + cmd.String())
	err = cmd.Run()
	return
}
