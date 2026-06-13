package cmd

import (
	"fmt"
	"os"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/manifoldco/promptui"
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
	// 按指定顺序显示命令
	order := []string{"in", "start", "stop", "list", "status", "set", "install", "uninstall"}

	// 建立 name -> command 的映射
	cmdMap := make(map[string]*cobra.Command)
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			cmdMap[c.Name()] = c
		}
	}

	fmt.Println()
	for _, name := range order {
		if c, ok := cmdMap[name]; ok {
			fmt.Printf("  %s\n", c.Use)
		}
	}
	fmt.Println()
}

func promptSelectContainer() string {
	svc := lxc.NewContainerService(core.GetExecutor())
	out, err := svc.List()
	if err != nil {
		fmt.Printf("获取容器列表失败: %v\n", err)
		return ""
	}
	names := strings.Fields(out)
	if len(names) == 0 {
		fmt.Println("没有可用的容器")
		return ""
	}

	selTemplate := &promptui.SelectTemplates{
		Label: "{{ . }}",
	}

	prompt := promptui.Select{
		Label:        "请选择容器",
		Items:        names,
		Templates:    selTemplate,
		HideHelp:     true,
		HideSelected: true,
	}

	_, name, err := prompt.Run()
	if err != nil {
		return "" // ESC
	}
	return name
}