package cmd

import (
	"fmt"
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

		// 如果有域名映射脚本，执行它
		hookPath := filepath.Join("/var/lib/lxc", name, "domain-hosts.sh")
		if _, err := exec.LookPath(hookPath); err == nil {
			exec.Command("sh", "-c", "nohup '"+hookPath+"' >/dev/null 2>&1 &").Run()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}