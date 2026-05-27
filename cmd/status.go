package cmd

import (
	"fmt"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [容器名]",
	Short: "查看容器状态",
	Long: `查看容器的各项配置是否生效，包括开机自启。
会读取容器配置文件并检测相关设置。`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		svc := lxc.NewContainerService(core.GetExecutor())

		status, err := svc.Status(name)
		if err != nil {
			return fmt.Errorf("获取容器状态失败: %w", err)
		}

		fmt.Printf("容器: %s\n\n", status.Name)

		fmt.Println("【开机自启】")
		switch status.Autostart {
		case "enabled":
			fmt.Println("  状态: 已启用 ✔")
		case "disabled":
			fmt.Println("  状态: 已禁用")
		default:
			fmt.Println("  状态: 未设置")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
