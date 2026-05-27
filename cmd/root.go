package cmd

import (
	"fmt"
	"os"
	"strings"

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

	visible := []*cobra.Command{}
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			visible = append(visible, c)
		}
	}

	if len(visible) > 0 {
		fmt.Println("  可用命令:")

		maxLen := 0
		for _, c := range visible {
			if len(c.Name()) > maxLen {
				maxLen = len(c.Name())
			}
		}
		cellW := maxLen + 2
		totalW := cellW*2 + 3
		horiz := strings.Repeat("─", totalW)

		printRow := func(left, right string) {
			left = " " + left + strings.Repeat(" ", cellW-1-len(left))
			if right != "" {
				right = " " + right + strings.Repeat(" ", cellW-1-len(right))
				fmt.Printf("  │%s│%s│\n", left, right)
			} else {
				empty := strings.Repeat(" ", cellW)
				fmt.Printf("  │%s│%s│\n", left, empty)
			}
		}

		fmt.Println("  ┌" + horiz + "┐")
		for i := 0; i < len(visible); i += 2 {
			left := visible[i].Name()
			right := ""
			if i+1 < len(visible) {
				right = visible[i+1].Name()
			}
			printRow(left, right)
			if i+2 < len(visible) {
				fmt.Println("  ├" + horiz + "┤")
			}
		}
		fmt.Println("  └" + horiz + "┘")
	}

	fmt.Println()
}