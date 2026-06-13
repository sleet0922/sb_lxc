package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

const systemdServiceTemplate = `[Unit]
Description=Domain hosts update for LXC container %i
After=lxc.service
Wants=lxc.service

[Service]
Type=oneshot
ExecStart=/var/lib/lxc/%i/domain-hosts.sh

[Install]
WantedBy=multi-user.target`

const systemdServicePath = "/etc/systemd/system/sb-lxc-domain@.service"

func ensureSystemdService() error {
	if _, err := os.Stat(systemdServicePath); err == nil {
		return nil
	}
	if err := os.WriteFile(systemdServicePath, []byte(systemdServiceTemplate), 0644); err != nil {
		return fmt.Errorf("创建 systemd 服务文件失败: %w", err)
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("重载 systemd 失败: %w\n%s", err, string(out))
	}
	return nil
}

var domainNameCmd = &cobra.Command{
	Use:   "domain_name [容器名] [域名]",
	Short: "域名映射",
	Long: `将容器 IP 动态映射到宿主机的 hosts 文件。
每次容器启动（手动或开机自启）都会自动更新 IP。
示例: sb_lxc domain_name mycontainer test.com`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		domain := args[1]

		svc := lxc.NewContainerService(core.GetExecutor())

		containerIP, err := svc.GetIP(name)
		if err != nil {
			return fmt.Errorf("获取容器 IP 失败，请先启动容器: %w", err)
		}

		configPath := filepath.Join("/var/lib/lxc", name, "config")
		hookDir := filepath.Join("/var/lib/lxc", name)
		hookPath := filepath.Join(hookDir, "domain-hosts.sh")

		// 检查是否已配置
		sysdOut, _ := exec.Command("systemctl", "is-enabled", "sb-lxc-domain@"+name+".service").CombinedOutput()
		if strings.TrimSpace(string(sysdOut)) == "enabled" {
			if data, err := os.ReadFile(hookPath); err == nil {
				if strings.Contains(string(data), domain) {
					fmt.Printf("域名 %s 已经映射到容器 %s\n", domain, name)
					return nil
				}
			}
		}

		// 清理容器配置中残留的 lxc.hook.post-start（如果有）
		if content, err := os.ReadFile(configPath); err == nil {
			lines := strings.Split(string(content), "\n")
			newLines := []string{}
			changed := false
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "lxc.hook.post-start") && strings.Contains(trimmed, "domain-hosts.sh") {
					changed = true
					continue
				}
				newLines = append(newLines, line)
			}
			if changed {
				updated := strings.Join(newLines, "\n")
				if !strings.HasSuffix(updated, "\n") {
					updated += "\n"
				}
				os.WriteFile(configPath, []byte(updated), 0644)
			}
		}

		// 创建域名更新脚本
		scriptContent := fmt.Sprintf(`#!/bin/sh
NAME="%s"
DOMAIN="%s"
for i in 1 2 3; do
  IP=$(lxc-info -n "$NAME" -iH 2>/dev/null | head -1 | tr -d ' \n\t')
  if [ -n "$IP" ] && [ "$IP" != " " ]; then
    break
  fi
  sleep 1
done
if [ -n "$IP" ]; then
  sed -i "/[[:space:]]%s$/d" /etc/hosts
  echo "$IP  %s" >> /etc/hosts
fi
`, name, domain, domain, domain)

		if err := os.WriteFile(hookPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("创建域名脚本失败: %w", err)
		}

		// 确保 systemd 服务文件存在
		if err := ensureSystemdService(); err != nil {
			fmt.Printf("警告: %v\n", err)
		}

		// 启用 systemd 服务（用于开机自启场景）
		enableCmd := exec.Command("systemctl", "enable", "sb-lxc-domain@"+name+".service")
		if out, err := enableCmd.CombinedOutput(); err != nil {
			fmt.Printf("警告: 启用 systemd 服务失败: %s\n%s\n", err, string(out))
		}

		// 立即更新 hosts 文件
		updateCmd := exec.Command("sh", "-c", fmt.Sprintf(
			"sed -i '/[[:space:]]%s$/d' /etc/hosts && echo '%s  %s' >> /etc/hosts",
			domain, containerIP, domain))
		if out, err := updateCmd.CombinedOutput(); err != nil {
			fmt.Printf("警告: 更新 hosts 文件失败: %s\n%s\n", err, string(out))
		}

		fmt.Printf("已将容器 %s (%s) 映射到域名 %s\n", name, containerIP, domain)
		fmt.Printf("容器启动时自动更新 hosts 文件\n")
		return nil
	},
}

func init() {
	// 已合并到 set 命令
}