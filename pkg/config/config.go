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

package config

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Config struct {
	ApiVersionCoreV1       string `default:"v1"`
	BackupDirectory        string `default:"/tmp/streamliner_backup"`
	DefaultNamespace       string `default:"default"`
	CmName                 string `default:"str-versioning-cache"`
	CmKind                 string `default:"ConfigMap"`
	CmOwnerTag             string `default:"responsible.unit=streamliner"`
	DefaultRolloutStrategy string `default:"linear"`
	Delimiter              string `default:"__"`
	DeployerVersion        string `default:"1.0.0" split_words:"true"`
	LabelPrefix            string `default:"deploy.streamliner"`
	NodeKind               string `default:"Node"`
}

var Env Config

func init() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	if err := envconfig.Process("", &Env); err != nil {
		logger.Error(err.Error())
	}
}
