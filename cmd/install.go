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
	Short: "安装/创建一个新容器",
	Long: `从可用系统镜像创建一个新的 LXC 容器。
如果没有指定容器名，会提示输入。
可以使用 --distro, --version, --arch 参数指定镜像。`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exec := core.GetExecutor()
		svc := lxc.NewImageService(exec)

		// 获取容器名
		name := ""
		if len(args) > 0 {
			name = args[0]
		} else {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("请输入容器名: ")
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("读取输入失败: %w", err)
			}
			name = strings.TrimSpace(input)
			if name == "" {
				return fmt.Errorf("容器名不能为空")
			}
		}

		// 获取参数或使用默认值
		distro, _ := cmd.Flags().GetString("distro")
		version, _ := cmd.Flags().GetString("version")
		arch, _ := cmd.Flags().GetString("arch")

		if distro == "" {
			distro = core.Cfg.GetString("default.distro")
		}
		if version == "" {
			version = core.Cfg.GetString("default.release")
		}
		if arch == "" {
			arch = core.Cfg.GetString("default.arch")
		}

		// 如果没有指定参数，提示用户
		reader := bufio.NewReader(os.Stdin)

		if distro == "" || !cmd.Flags().Changed("distro") {
			fmt.Printf("发行版 [%s]: ", core.Cfg.GetString("default.distro"))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				distro = input
			} else {
				distro = core.Cfg.GetString("default.distro")
			}
		}

		if version == "" || !cmd.Flags().Changed("version") {
			fmt.Printf("版本 [%s]: ", core.Cfg.GetString("default.release"))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				version = input
			} else {
				version = core.Cfg.GetString("default.release")
			}
		}

		if arch == "" || !cmd.Flags().Changed("arch") {
			fmt.Printf("架构 [%s]: ", core.Cfg.GetString("default.arch"))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				arch = input
			} else {
				arch = core.Cfg.GetString("default.arch")
			}
		}

		fmt.Printf("\n正在创建容器: %s / %s / %s / %s\n", name, distro, version, arch)
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
