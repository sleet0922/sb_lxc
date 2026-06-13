package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "容器配置",
	Long: `交互式配置容器，支持开机自启和域名映射。
使用上下键选择，回车确认。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := lxc.NewContainerService(core.GetExecutor())

		// 获取容器列表
		containers, err := getContainerNames()
		if err != nil {
			return fmt.Errorf("获取容器列表失败: %w", err)
		}
		if len(containers) == 0 {
			fmt.Println("没有可用的容器")
			return nil
		}

		fmt.Print("\033[A") // 吸收 promptui 遗留空行

		selTemplate := &promptui.SelectTemplates{
			Label: "{{ . }}",
		}

		actions := []string{"启用开机自启", "禁用开机自启", "域名映射"}

		for {
			// 选择操作
			actionPrompt := promptui.Select{
				Label:        "请选择操作",
				Items:        actions,
				Templates:    selTemplate,
				HideHelp:     true,
				HideSelected: true,
			}
			_, action, err := actionPrompt.Run()
			if err != nil {
				// ESC 退出
				return nil
			}

		chooseContainer:
			for {
				// 选择容器
				containerPrompt := promptui.Select{
					Label:        "请选择容器",
					Items:        containers,
					Templates:    selTemplate,
					HideHelp:     true,
					HideSelected: true,
				}
				_, name, err := containerPrompt.Run()
				if err != nil {
					// ESC 返回上一级
					break
				}

				switch action {
				case "启用开机自启":
					out, err := svc.SetAutostart(name, true)
					if err != nil {
						return fmt.Errorf("启用开机自启失败: %w\n%s", err, out)
					}
					fmt.Println(out)
					return nil

				case "禁用开机自启":
					out, err := svc.SetAutostart(name, false)
					if err != nil {
						return fmt.Errorf("禁用开机自启失败: %w\n%s", err, out)
					}
					fmt.Println(out)
					return nil

				case "域名映射":
					// 输入域名
					domainPrompt := promptui.Prompt{
						Label: "请输入域名",
						Templates: &promptui.PromptTemplates{
							Prompt:  "{{ . }} ",
							Success: "",
						},
					}
					domain, err := domainPrompt.Run()
					if err != nil {
						// ESC 返回容器选择
						continue chooseContainer
					}
					domain = strings.TrimSpace(domain)
					if domain == "" {
						fmt.Println("域名不能为空")
						continue chooseContainer
					}

					configPath := filepath.Join("/var/lib/lxc", name, "config")
					hookPath := filepath.Join("/var/lib/lxc", name, "domain-hosts.sh")

					// 检查是否已配置相同域名
					sysdOut, _ := exec.Command("systemctl", "is-enabled", "sb-lxc-domain@"+name+".service").CombinedOutput()
					if strings.TrimSpace(string(sysdOut)) == "enabled" {
						if data, err := os.ReadFile(hookPath); err == nil {
							if strings.Contains(string(data), domain) {
								fmt.Printf("域名 %s 已经映射到容器 %s\n", domain, name)
								return nil
							}
						}
					}

					// 清理容器配置中残留的 lxc.hook.post-start
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

					// 启用 systemd 服务
					enableCmd := exec.Command("systemctl", "enable", "sb-lxc-domain@"+name+".service")
					if out, err := enableCmd.CombinedOutput(); err != nil {
						fmt.Printf("警告: 启用 systemd 服务失败: %s\n%s\n", err, string(out))
					}

					// 尝试获取 IP 并立即更新 hosts（容器可能在运行）
					containerIP, ipErr := svc.GetIP(name)
					if ipErr == nil && containerIP != "" {
						updateCmd := exec.Command("sh", "-c", fmt.Sprintf(
							"sed -i '/[[:space:]]%s$/d' /etc/hosts && echo '%s  %s' >> /etc/hosts",
							domain, containerIP, domain))
						if out, err := updateCmd.CombinedOutput(); err != nil {
							fmt.Printf("警告: 更新 hosts 文件失败: %s\n%s\n", err, string(out))
						}
						fmt.Printf("已将容器 %s (%s) 映射到域名 %s\n", name, containerIP, domain)
					} else {
						fmt.Printf("已将容器 %s 映射到域名 %s（容器启动后将自动更新 hosts）\n", name, domain)
					}
					return nil
				}
			}
		}
	},
}

func getContainerNames() ([]string, error) {
	svc := lxc.NewContainerService(core.GetExecutor())
	out, err := svc.List()
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	return names, nil
}

func init() {
	rootCmd.AddCommand(setCmd)
}