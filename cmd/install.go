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

var installCmd = &cobra.Command{
	Use:   "install [容器名]",
	Short: "创建容器",
	Long: `从可用系统镜像创建一个新的 LXC 容器。
如果没有指定容器名，会提示输入。
可以使用 --distro, --version, --arch 参数指定镜像。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exec := core.GetExecutor()
		svc := lxc.NewImageService(exec)

		distro, _ := cmd.Flags().GetString("distro")
		version, _ := cmd.Flags().GetString("version")
		arch, _ := cmd.Flags().GetString("arch")

		if distro == "" || version == "" || arch == "" {
			selectedDistro, selectedVersion, selectedArch, err := svc.SelectImageInteractive()
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：%v\n", err)
				return nil
			}
			if distro == "" {
				distro = selectedDistro
			}
			if version == "" {
				version = selectedVersion
			}
			if arch == "" {
				arch = selectedArch
			}
		}

		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("请输入容器名：")
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取输入失败：%w", err)
			}
			name = strings.TrimSpace(input)
			if name == "" {
				return fmt.Errorf("容器名不能为空")
			}
		}

		fmt.Printf("\n正在创建容器：%s / %s / %s / %s\n", name, distro, version, arch)
		out, err := svc.CreateFromDownload(name, distro, version, arch)
		if err != nil {
			return fmt.Errorf("创建容器失败: %w\n%s", err, out)
		}
		fmt.Println(out)
		fmt.Printf("\n容器 %s 创建成功!\n", name)
		return nil
	},
}

func init() {
	installCmd.Flags().StringP("distro", "d", "", "发行版 (如 ubuntu, debian, alpine)")
	installCmd.Flags().StringP("version", "v", "", "版本 (如 jammy, bookworm, latest)")
	installCmd.Flags().StringP("arch", "a", "", "架构 (如 amd64, arm64)")
	rootCmd.AddCommand(installCmd)
}
