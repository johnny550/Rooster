package config

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Config struct {
	DeployerVersion string `default:"1.0.0" split_words:"true"`
	BackupDirectory string `default:"/tmp/rooster_backup"`
}

var Env Config

func init() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	if err := envconfig.Process("", &Env); err != nil {
		logger.Error(err.Error())
	}
}
