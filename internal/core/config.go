package core

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	Cfg *viper.Viper
	Log *zap.Logger
)

func InitConfig() error {
	Cfg = viper.New()
	Cfg.SetConfigName("config")
	Cfg.SetConfigType("yaml")

	home, _ := os.UserHomeDir()
	cfgDir := filepath.Join(home, ".sb_lxc")
	Cfg.AddConfigPath(cfgDir)

	os.MkdirAll(cfgDir, 0755)

	Cfg.SetDefault("default.distro", "ubuntu")
	Cfg.SetDefault("default.release", "jammy")
	Cfg.SetDefault("default.arch", "amd64")

	if err := Cfg.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			Cfg.SafeWriteConfig()
		}
	}
	return nil
}

func InitLogger() error {
	if os.Getenv("SB_LXC_DEBUG") == "1" {
		var err error
		Log, err = zap.NewDevelopment()
		return err
	}

	Log = zap.NewNop()
	return nil
}

func GetExecutor() Executor {
	return &ShellExecutor{}
}
