package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var portCmd = &cobra.Command{
	Use:   "port [容器名] [容器端口] [宿主机端口]",
	Short: "端口映射",
	Long: `将容器的端口映射到宿主机端口。
示例: sb_lxc port mycontainer 80 8080  # 将容器的80端口映射到宿主机的8080端口`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		containerPort := args[1]
		hostPort := args[2]

		// 验证端口是否为数字
		if _, err := strconv.Atoi(containerPort); err != nil {
			return fmt.Errorf("容器端口必须是数字: %s", containerPort)
		}
		if _, err := strconv.Atoi(hostPort); err != nil {
			return fmt.Errorf("宿主机端口必须是数字: %s", hostPort)
		}

		configPath := filepath.Join("/var/lib/lxc", name, "config")

		// 读取现有配置
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("读取容器配置失败: %w", err)
		}

		lines := strings.Split(string(content), "\n")

		// 检查是否已有相同的宿主机端口映射
		newLines := []string{}
		hasType := false
		hasLink := false
		hasFlags := false
		foundPortMapping := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "lxc.net.0.type") {
				hasType = true
			}
			if strings.HasPrefix(trimmed, "lxc.net.0.link") {
				hasLink = true
			}
			if strings.HasPrefix(trimmed, "lxc.net.0.flags") {
				hasFlags = true
			}
			// 检查是否已有相同的端口映射
			if strings.Contains(trimmed, fmt.Sprintf("hostport=%s", hostPort)) {
				foundPortMapping = true
			}
			newLines = append(newLines, line)
		}

		if foundPortMapping {
			fmt.Printf("端口 %s 已经映射，跳过添加。\n", hostPort)
			return nil
		}

		// 添加必要的网络配置
		if !hasType {
			newLines = append(newLines, "lxc.net.0.type = veth")
		}
		if !hasLink {
			newLines = append(newLines, "lxc.net.0.link = lxcbr0")
		}
		if !hasFlags {
			newLines = append(newLines, "lxc.net.0.flags = up")
		}

		// 添加端口映射配置
		// 使用 iptables 进行端口转发
		natRule := fmt.Sprintf("lxc.hook.start = sh -c \"iptables -t nat -A PREROUTING -p tcp --dport %s -j DNAT --to-destination 10.0.3.%s:%s || true\"", hostPort, name, containerPort)
		newLines = append(newLines, natRule)

		// 写入配置
		updated := strings.Join(newLines, "\n")
		if !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}

		if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
			return fmt.Errorf("写入容器配置失败: %w", err)
		}

		fmt.Printf("已将容器 %s 的端口 %s 映射到宿主机端口 %s\n", name, containerPort, hostPort)
		fmt.Println("注意: 需要重启容器使配置生效")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(portCmd)
}
