package cmd

import (
	"fmt"
	"os"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sb_lxc",
	Short: "LXC 容器管理",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	core.InitConfig()
	core.InitLogger()

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SilenceUsage = true

	// 隐藏 help 命令（不带参数已显示帮助）
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	rootCmd.SetHelpFunc(customHelp)
}

func listContainers() {
	svc := lxc.NewContainerService(core.GetExecutor())
	out, err := svc.ListDetailed()
	if err != nil {
		fmt.Printf("获取容器列表失败: %s\n", err)
		return
	}
	fmt.Print(out)
}

func customHelp(cmd *cobra.Command, args []string) {
	visible := []*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.Hidden && c.Name() != "help" {
			visible = append(visible, c)
		}
	}

	if len(visible) > 0 {
		fmt.Println()
		for _, c := range visible {
			fmt.Printf("  %s\n", c.Use)
		}
	}

	fmt.Println()
}