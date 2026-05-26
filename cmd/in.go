package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var inCmd = &cobra.Command{
	Use:   "in [容器名]",
	Short: "进入容器",
	Long:  `使用 lxc-attach 进入指定的 LXC 容器。`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		command := exec.Command("lxc-attach", "-n", name)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr

		// lxc-attach 会继承容器内最后一个命令的退出码，这不是进入容器失败
		// 用户可能执行了任何命令（包括不存在的命令），不应被视为错误
		if err := command.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				return nil
			}
			return fmt.Errorf("进入容器失败: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(inCmd)
}
