package cmd

import (
	"fmt"
	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var disableCmd = &cobra.Command{
	Use:   "disable [容器名]",
	Short: "禁用容器开机自启",
	Long:  `取消指定容器的开机自动启动设置。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			listContainers()
			return nil
		}
		name := args[0]
		svc := lxc.NewContainerService(core.GetExecutor())
		out, err := svc.SetAutostart(name, false)
		if err != nil {
			return fmt.Errorf("禁用开机自启失败: %w\n%s", err, out)
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	// 已合并到 set 命令
}
