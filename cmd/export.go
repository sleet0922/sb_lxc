package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"sb_lxc/internal/lxc"

	"github.com/spf13/cobra"
)

var exportOutput string

var exportCmd = &cobra.Command{
	Use:   "export [容器名]",
	Short: "导出容器为可移植的归档文件",
	Long: `将容器及其所有配置（端口映射、域名映射、开机自启等）导出为 tar.gz 归档，
可在其他机器上用 sb_lxc import 导入。

导出内容包括：
  - 容器完整快照（rootfs）
  - 端口映射配置
  - 域名映射配置
  - 开机自启设置`,
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

		if err := requireContainer(name); err != nil {
			return err
		}

		output := exportOutput
		if output == "" {
			output = name + ".sb_lxc.tar.gz"
		}

		// 确保输出路径是绝对路径或相对于当前目录
		if !filepath.IsAbs(output) && !strings.HasPrefix(output, "./") && !strings.HasPrefix(output, "../") {
			output = "./" + output
		}

		fmt.Printf("正在导出容器 %s ...\n", name)
		if err := lxc.ExportContainer(name, output); err != nil {
			return fmt.Errorf("导出失败: %w", err)
		}

		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "输出文件路径 (默认: <容器名>.sb_lxc.tar.gz)")
	rootCmd.AddCommand(exportCmd)
}
