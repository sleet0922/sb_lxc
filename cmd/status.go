package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [容器名]",
	Short: "查看容器状态",
	Long: `查看容器的各项配置是否生效，包括开机自启和端口映射。
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

		// 开机自启状态
		fmt.Println("【开机自启】")
		switch status.Autostart {
		case "enabled":
			fmt.Println("  状态: 已启用 ✔")
		case "disabled":
			fmt.Println("  状态: 已禁用")
		default:
			fmt.Println("  状态: 未设置")
		}

		// 端口映射状态
		fmt.Println("\n【端口映射】")
		if len(status.PortMaps) == 0 {
			fmt.Println("  未配置端口映射")
		} else {
			for _, pm := range status.PortMaps {
				fmt.Printf("  宿主机 %s -> 容器 %s\n", pm.HostPort, pm.ContainerPort)
			}
			// 检测 iptables 规则是否已生效
			fmt.Println("\n  iptables 规则生效状态:")
			for _, pm := range status.PortMaps {
				checkIptablesRule(pm.HostPort)
			}
		}

		return nil
	},
}

func checkIptablesRule(hostPort string) {
	out, _ := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n").CombinedOutput()
	if strings.Contains(string(out), fmt.Sprintf("dpt:%s", hostPort)) {
		fmt.Printf("  端口 %s: 规则已生效 ✔\n", hostPort)
	} else {
		fmt.Printf("  端口 %s: 规则未生效（需重启容器） ✘\n", hostPort)
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}