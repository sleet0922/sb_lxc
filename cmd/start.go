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

// fixLxcbr0Route 检查并删除 lxc-net 注入的错误默认路由。
// 某些环境下 lxc-net 启动时会在主路由表中插入 default via lxcbr0，
// 导致宿主机和容器都无法访问外网。
func fixLxcbr0Route() {
	out, err := exec.Command("ip", "route", "show", "default").CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "dev lxcbr0") {
			exec.Command("ip", "route", "del", "default", "via", "10.0.3.1", "dev", "lxcbr0").Run()
			fmt.Println("已修复: 删除 lxcbr0 的错误默认路由")
			break
		}
	}
}

var startCmd = &cobra.Command{
	Use:   "start [容器名]",
	Short: "启动容器",
	Long:  `启动一个已创建的 LXC 容器，默认后台运行。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 自动修复 lxc-net 注入的错误默认路由
		fixLxcbr0Route()

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

		out, err := svc.Start(name, true)
		if err != nil {
			return fmt.Errorf("启动容器失败: %w\n%s", err, out)
		}
		fmt.Printf("容器 %s 已启动\n", name)

		// 端口映射：刷新脚本内容，然后在后台子进程中重试（等容器 IP 就绪）
		// 注意：必须用 exec.Command + Start() 启动独立 OS 子进程
		// 不能用 goroutine — 主程序退出后 goroutine 会被立即杀死
		portMappingPath := filepath.Join("/var/lib/lxc", name, "port-mappings")
		portScriptPath := filepath.Join("/var/lib/lxc", name, "port-forward.sh")
		if _, err := os.Stat(portMappingPath); err == nil {
			lxc.CreatePortForwardScript(name) // 确保脚本内容与当前代码一致

			// 确保 systemd 模板服务存在并启用，保证重启后端口转发自动恢复
			if err := lxc.EnsurePortForwardService(); err != nil {
				fmt.Printf("警告: 创建端口转发 systemd 服务失败: %v\n", err)
			}
			exec.Command("systemctl", "enable", "sb-lxc-port@"+name+".service").Run()

			exec.Command("sh", "-c",
				fmt.Sprintf("for i in 1 2 3 4 5; do "+
					"sleep 2; "+
					"if lxc-info -n '%s' -iH 2>/dev/null | grep -q '\\.'; then "+
					"'%s'; break; fi; done", name, portScriptPath)).Start()
		}

		// 域名映射：后台子进程执行脚本（等容器 IP 就绪后再操作 /etc/hosts）
		hookPath := filepath.Join("/var/lib/lxc", name, "domain-hosts.sh")
		if _, err := os.Stat(hookPath); err == nil {
			// 确保 systemd 模板服务存在并启用
			if err := ensureSystemdService(); err != nil {
				fmt.Printf("警告: 创建域名映射 systemd 服务失败: %v\n", err)
			}
			exec.Command("systemctl", "enable", "sb-lxc-domain@"+name+".service").Run()

			exec.Command("sh", "-c",
				fmt.Sprintf("sleep 6 && '"+hookPath+"'")).Start()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
