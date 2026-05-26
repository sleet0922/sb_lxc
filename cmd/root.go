package cmd

import (
	"os"

	"sb_lxc/internal/core"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sb_lxc",
	Short: "简化 LXC 容器管理操作",
	Long:  `sb_lxc 是一个 LXC 管理 CLI 工具，提供简单的命令行接口管理 LXC 容器。`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	core.InitConfig()
	core.InitLogger()
}
