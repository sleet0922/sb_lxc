package cmd

import (
	"fmt"
	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var enableCmd = &cobra.Command{
	Use:   "enable [容器名]",
	Short: "启用容器开机自启",
	Long:  `设置指定容器在系统启动时自动启动。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			listContainers()
			return nil
		}
		name := args[0]
		svc := lxc.NewContainerService(core.GetExecutor())
		out, err := svc.SetAutostart(name, true)
		if err != nil {
			return fmt.Errorf("启用开机自启失败: %w\n%s", err, out)
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(enableCmd)
}
