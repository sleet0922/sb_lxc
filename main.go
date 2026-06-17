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
		return withContainer(args, CmdStart)
	case "stop":
		return withContainer(args, CmdStop)
	case "in":
		return withContainer(args, CmdIn)
	case "set":
		return withContainer(args, CmdSet)
	case "export":
		return withContainer(args, CmdExport)
	case "import":
		return CmdImport(args)
	case "install", "i":
		return CmdInstall()
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("未知命令: %s", cmd)
	}
}

// withContainer 校验并提取容器名参数后调用处理函数
func withContainer(args []string, fn func(string) error) error {
	if len(args) < 1 {
		return fmt.Errorf("缺少容器名参数")
	}
	return fn(args[0])
}

func printUsage() {
	fmt.Printf(`sb_lxc - Incus 容器管理工具 v%s

用法:
  sb_lxc list               列出已安装容器
  sb_lxc start  <容器名>     启动容器
  sb_lxc stop   <容器名>     停止容器
  sb_lxc in     <容器名>     进入容器 (sh)
  sb_lxc set    <容器名>     容器设置 (端口映射 / 开机自启动)
  sb_lxc export <容器名>     导出容器为 ./容器名.tar.gz
  sb_lxc import <文件路径> [新容器名]  导入容器
  sb_lxc install            安装新容器 (交互式选择发行版)
  sb_lxc help               显示此帮助
`, Version)
}
