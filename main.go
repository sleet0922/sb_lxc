package main

import (
	"fmt"
	"os"
)

// Version 工具版本
const Version = "1.0.0"

// MirrorRemote 镜像源在本地的 remote 名称
const MirrorRemote = "mirror-images"

// MirrorURL 清华大学 LXC 镜像源地址
const MirrorURL = "https://mirrors.tuna.tsinghua.edu.cn/lxc-images/"

func main() {
	// 每次启动都先确保只保留清华镜像源（移除官方 images 源与旧 mirror-images）
	client := NewIncusClient()
	client.EnsureMirrorRemote()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	if err := dispatch(cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "✘ %v\n", err)
		os.Exit(1)
	}
}

// dispatch 命令分发
func dispatch(cmd string, args []string) error {
	switch cmd {
	case "list", "ls":
		return CmdList()
	case "start":
		return withContainer(args, "选择要启动的容器", CmdStart)
	case "stop":
		return withContainer(args, "选择要停止的容器", CmdStop)
	case "in":
		return withContainer(args, "选择要进入的容器", CmdIn)
	case "set":
		return withContainer(args, "选择要设置的容器", CmdSet)
	case "export":
		return withContainer(args, "选择要导出的容器", CmdExport)
	case "import":
		return CmdImport(args)
	case "install", "i":
		return CmdInstall()
	case "uninstall", "rm":
		return CmdUninstall()
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("未知命令: %s", cmd)
	}
}

// withContainer 若 args 中有容器名则直接使用，否则弹出交互式选择菜单。
func withContainer(args []string, label string, fn func(string) error) error {
	if len(args) >= 1 {
		return fn(args[0])
	}
	name, err := selectContainer(label)
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	return fn(name)
}

// selectContainer 列出所有容器供用户选择，返回选中容器名。
func selectContainer(label string) (string, error) {
	client := NewIncusClient()
	cs, err := client.ListContainers()
	if err != nil {
		return "", err
	}
	if len(cs) == 0 {
		fmt.Println("暂无容器。")
		return "", nil
	}
	names := make([]string, len(cs))
	for i, c := range cs {
		names[i] = c.Name
	}
	choice := selectMenu(names, label+" (↑↓ 选择, Enter 确认, q 退出)")
	if choice < 0 {
		return "", nil
	}
	return names[choice], nil
}

func printUsage() {
	fmt.Printf(`sb_lxc - Incus 容器管理工具 v%s

用法:
  sb_lxc list               列出已安装容器
  sb_lxc start  [容器名]     启动容器 (无参数则交互选择)
  sb_lxc stop   [容器名]     停止容器 (无参数则交互选择)
  sb_lxc in     [容器名]     进入容器 (无参数则交互选择)
  sb_lxc set    [容器名]     容器设置 (无参数则交互选择)
  sb_lxc export [容器名]     导出容器 (无参数则交互选择)
  sb_lxc import [文件路径] [新容器名]  导入容器 (无参数则选择本地 tar.gz)
  sb_lxc install            安装新容器 (交互式选择发行版)
  sb_lxc uninstall           删除容器 (交互式选择)
  sb_lxc help               显示此帮助
`, Version)
}
