package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sb_lxc/internal/lxc"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <归档文件>",
	Short: "从归档文件导入容器",
	Long: `从 sb_lxc export 生成的 .tar.gz 归档文件导入容器。
导入会自动重建端口映射、域名映射、开机自启等所有配置。
如果容器名已存在，会引导输入新名称。

示例: sb_lxc import my-container.sb_lxc.tar.gz`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		archivePath := args[0]

		if _, err := os.Stat(archivePath); os.IsNotExist(err) {
			return fmt.Errorf("归档文件 %s 不存在", archivePath)
		}

		// 先读取元数据，检查名称是否冲突
		meta, err := lxc.ParseMeta(archivePath)
		if err != nil {
			return fmt.Errorf("读取归档元数据失败: %w", err)
		}

		newName := ""
		containerDir := filepath.Join("/var/lib/lxc", meta.Name)
		if _, err := os.Stat(containerDir); err == nil {
			// 容器名已存在，引导输入新名称
			fmt.Printf("容器 %s 已存在\n", meta.Name)
			prompt := promptui.Prompt{
				Label: "请输入新容器名称",
				Templates: &promptui.PromptTemplates{
					Prompt:  "{{ . }} ",
					Success: "",
				},
				Validate: func(input string) error {
					input = strings.TrimSpace(input)
					if input == "" {
						return fmt.Errorf("名称不能为空")
					}
					if input == meta.Name {
						return fmt.Errorf("与原名相同，请先卸载或换一个名字")
					}
					if _, err := os.Stat(filepath.Join("/var/lib/lxc", input)); err == nil {
						return fmt.Errorf("容器 %s 也已存在", input)
					}
					return nil
				},
			}
			var err error
			newName, err = prompt.Run()
			if err != nil {
				return nil // ESC 取消
			}
			newName = strings.TrimSpace(newName)
		}

		fmt.Printf("正在导入 %s ...\n", archivePath)
		if err := lxc.ImportContainer(archivePath, newName); err != nil {
			return fmt.Errorf("导入失败: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
