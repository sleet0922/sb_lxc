package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sb_lxc/internal/core"
	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall [容器名]",
	Short: "删除一个容器",
	Long:  `永久删除指定的 LXC 容器及其所有数据。`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		force, _ := cmd.Flags().GetBool("force")

		if !force {
			reader := bufio.NewReader(os.Stdin)
			fmt.Printf("确认删除容器 %s? 输入容器名以确认: ", name)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
			if strings.TrimSpace(input) != name {
				fmt.Println("容器名不匹配，已取消删除。")
				return nil
			}
		}

		svc := lxc.NewContainerService(core.GetExecutor())
		out, err := svc.Destroy(name)
		if err != nil {
			return fmt.Errorf("删除容器失败: %w\n%s", err, out)
		}
		fmt.Println(out)
		fmt.Printf("容器 %s 已删除。\n", name)
		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolP("force", "f", false, "强制删除，不提示确认")
	rootCmd.AddCommand(uninstallCmd)
}
