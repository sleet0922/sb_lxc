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

		// 使用 syscall.Exec 来替换当前进程，这样用户可以像在容器中一样操作
		command := exec.Command("lxc-attach", "-n", name)
		command.Stdin = os.Stdin
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr

		if err := command.Run(); err != nil {
			return fmt.Errorf("进入容器失败: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(inCmd)
}
