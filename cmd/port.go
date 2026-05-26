package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

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

		if _, err := strconv.Atoi(containerPort); err != nil {
			return fmt.Errorf("容器端口必须是数字: %s", containerPort)
		}
		if _, err := strconv.Atoi(hostPort); err != nil {
			return fmt.Errorf("宿主机端口必须是数字: %s", hostPort)
		}

		// 获取容器 IP
		svc := lxc.NewContainerService(core.GetExecutor())
		containerIP, err := svc.GetIP(name)
		if err != nil {
			return fmt.Errorf("获取容器 IP 失败，请先启动容器: %w", err)
		}

		configPath := filepath.Join("/var/lib/lxc", name, "config")
		hookDir := filepath.Join("/var/lib/lxc", name)
		hookPath := filepath.Join(hookDir, "port-forward.sh")

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
		hasHook := false
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
			if strings.HasPrefix(trimmed, "lxc.hook.pre-start") {
				hasHook = true
			}
			if strings.Contains(trimmed, fmt.Sprintf("--dport %s", hostPort)) {
				foundPortMapping = true
			}
			newLines = append(newLines, line)
		}

		if foundPortMapping {
			fmt.Printf("端口 %s 已经映射，跳过添加。\n", hostPort)
			return nil
		}

		// 创建端口转发脚本
		scriptContent := fmt.Sprintf(`#!/bin/sh
iptables -t nat -A PREROUTING -p tcp --dport %s -j DNAT --to-destination %s:%s || true
`, hostPort, containerIP, containerPort)

		if err := os.WriteFile(hookPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("创建端口转发脚本失败: %w", err)
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

		// 替换或添加 hook
		if hasHook {
			for i, line := range newLines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "lxc.hook.pre-start") && strings.Contains(line, "port-forward.sh") {
					newLines[i] = fmt.Sprintf("lxc.hook.pre-start = %s", hookPath)
				}
			}
		} else {
			newLines = append(newLines, fmt.Sprintf("lxc.hook.pre-start = %s", hookPath))
		}

		// 写入配置
		updated := strings.Join(newLines, "\n")
		if !strings.HasSuffix(updated, "\n") {
			updated += "\n"
		}

		if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
			return fmt.Errorf("写入容器配置失败: %w", err)
		}

		// 立即添加 iptables 规则，不用等重启
		addRule := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
			"-p", "tcp", "--dport", hostPort,
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%s", containerIP, containerPort))
		addRule.Run()

		fmt.Printf("已将容器 %s 的端口 %s 映射到宿主机端口 %s\n", name, containerPort, hostPort)
		fmt.Printf("容器 IP: %s\n", containerIP)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(portCmd)
}