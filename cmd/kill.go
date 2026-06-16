package cmd

import (
	"fmt"
	"os/exec"

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

		out, err := exec.Command("lxc-stop", "-n", name, "-k").CombinedOutput()
		if err != nil {
			return fmt.Errorf("强制停止容器失败: %w\n%s", err, out)
		}
		fmt.Printf("容器 %s 已强制停止\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(killCmd)
}
