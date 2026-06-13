package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [容器名]",
	Short: "启动容器",
	Long:  `启动一个已创建的 LXC 容器，默认后台运行。`,
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
			exec.Command("sh", "-c",
				fmt.Sprintf("for i in 1 2 3 4 5; do "+
					"sleep 2; "+
					"if lxc-info -n '%s' -iH 2>/dev/null | grep -q '\\.'; then "+
					"'%s'; break; fi; done", name, portScriptPath)).Start()
		}

		// 域名映射：后台子进程执行脚本（等容器 IP 就绪后再操作 /etc/hosts）
		hookPath := filepath.Join("/var/lib/lxc", name, "domain-hosts.sh")
		if _, err := os.Stat(hookPath); err == nil {
			exec.Command("sh", "-c",
				fmt.Sprintf("sleep 6 && '"+hookPath+"'")).Start()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
