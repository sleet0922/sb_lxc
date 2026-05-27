package cmd

import (
	"fmt"
	"os"

	"sb_lxc/internal/core"

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

	rootCmd.SetHelpFunc(customHelp)
}

func customHelp(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println("  LXC 容器管理")
	fmt.Println()

	visible := []*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			visible = append(visible, c)
		}
	}

	if len(visible) > 0 {
		fmt.Println("  可用命令:")
		fmt.Println()
		for _, c := range visible {
			fmt.Printf("    %s\n", c.Use)
		}
	}

	fmt.Println()
}