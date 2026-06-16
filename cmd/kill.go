package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var killCmd = &cobra.Command{
	Use:   "kill [容器名]",
	Short: "强制停止容器",
	Long:  `强制停止指定的 LXC 容器，等价于 lxc-stop -n 容器名 -k。`,
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

		out, err := exec.Command("lxc-stop", "-n", name, "-k").CombinedOutput()
		if err != nil {
			return fmt.Errorf("强制停止容器失败: %s", strings.TrimSpace(string(out)))
		}
		fmt.Printf("容器 %s 已强制停止\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
