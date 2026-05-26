package cmd

import (
	"fmt"
	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出容器",
	Long:  `显示所有 LXC 容器的详细信息，包括名称、状态、IP 地址等。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := lxc.NewContainerService(core.GetExecutor())
		out, err := svc.ListDetailed()
		if err != nil {
			return fmt.Errorf("列出容器失败: %w\n%s", err, out)
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
