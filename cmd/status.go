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
	Args: cobra.MaximumNArgs(1),
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
		svc := lxc.NewContainerService(core.GetExecutor())

		status, err := svc.Status(name)
		if err != nil {
			return fmt.Errorf("获取容器状态失败: %w", err)
		}

		fmt.Printf("容器: %s\n", status.Name)
		fmt.Println()

		if status.State != "" {
			fmt.Printf("  状态: %s\n", status.State)
		}
		if status.CPU != "" {
			fmt.Printf("  CPU:  %s\n", status.CPU)
		}
		if status.Memory != "" {
			fmt.Printf("  内存: %s\n", status.Memory)
		}
		if status.State == "" && status.CPU == "" && status.Memory == "" {
			fmt.Println("  容器未运行")
		}

		switch status.Autostart {
		case "enabled":
			fmt.Println("  开机自启: 已启用")
		case "disabled":
			fmt.Println("  开机自启: 已禁用")
		default:
			fmt.Println("  开机自启: 未设置")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
