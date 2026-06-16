package cmd

import (
	"fmt"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [容器名]",
	Short: "关停容器",
	Long:  `关停一个正在运行的 LXC 容器。`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			name = promptSelectContainer()
			if name == "" {
				return nil
			}
		}

		if err := requireContainer(name); err != nil {
			return err
		}
		state := containerState(name)
		if state != "RUNNING" {
			return fmt.Errorf("容器 %s 未运行 (当前状态: %s)", name, stateText(state))
		}

		svc := lxc.NewContainerService(core.GetExecutor())
		out, err := svc.Stop(name, false)
		if err != nil {
			return fmt.Errorf("关停容器失败: %s", out)
		}
		fmt.Printf("容器 %s 已关停\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
