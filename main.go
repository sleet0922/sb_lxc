package main

import (
	"fmt"
	"os"
)

// Version 工具版本
const Version = "1.6.5"

// MirrorRemote 镜像源在本地的 remote 名称
const MirrorRemote = "mirror-images"

// MirrorURL LXC 镜像源地址（清华源已失效，改用官方源）
const MirrorURL = "https://images.linuxcontainers.org/"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// 轻量命令：跳过启动期网络探测，保证 help/version 等即时响应
	if isLightweightCommand(cmd) {
		if err := dispatch(cmd, args); err != nil {
			fmt.Fprintf(os.Stderr, "✘ %v\n", err)
			os.Exit(1)
		}
		return
	}

	// 重量级命令：执行启动期副作用（网络探测、网桥清理、macvlan shim 维护）
	client := NewIncusClient()
	client.EnsureMirrorRemote()
	warnAutoHostMacvlan(client.EnsureDefaultMacvlanProfile())
	_ = client.AutoCleanupUnusedBridges()
	warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))

	if err := dispatch(cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "✘ %v\n", err)
		os.Exit(1)
	}
}

// isLightweightCommand 判断命令是否为轻量级（无需网络探测）。
// help/version 等只读命令跳过启动期副作用，保证即时响应。
func isLightweightCommand(cmd string) bool {
	switch cmd {
	case "help", "-h", "--help", "--version", "-v", "version":
		return true
	}
	return false
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
	case "restart":
		return withContainer(args, "选择要重启的容器", CmdRestart)
	case "in":
		return withContainer(args, "选择要进入的容器", CmdIn)
	case "exec":
		return CmdExec(args)
	case "set":
		return CmdSet(args)
	case "export":
		return withContainer(args, "选择要导出的容器", CmdExport)
	case "import":
		return CmdImport(args)
	case "install", "i":
		return CmdInstall(args)
	case "uninstall", "rm":
		return CmdUninstall()
	case "build":
		return CmdBuild(args)
	case "run":
		return CmdRun(args)
	case "image", "img":
		return CmdImage(args)
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "--version", "-v", "version":
		fmt.Printf("sb_lxc v%s\n", Version)
		return nil
	default:
		return fmt.Errorf("未知命令: %s (使用 'sb_lxc help' 查看可用命令)", cmd)
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

容器管理:
  sb_lxc list                 | 列出已安装容器
  sb_lxc install              | 安装新容器 (交互式选择发行版)
  sb_lxc uninstall            | 删除容器
  sb_lxc start   [容器名]     | 启动容器
  sb_lxc stop    [容器名]     | 停止容器
  sb_lxc restart [容器名]     | 重启容器
  sb_lxc in      [容器名]     | 进入容器 shell
  sb_lxc exec    <容器名> <命令...> | 在容器内执行命令 (非交互)
  sb_lxc export  [容器名]     | 导出容器为 tar.gz
  sb_lxc import  [文件] [名]  | 从 tar.gz 导入容器

容器设置 (sb_lxc set <容器名> ...):
  port [规格]                 | 端口映射 (规格如 8080:80/tcp)
  port --rm <规格>            | 取消端口映射
  port --list                 | 查看端口映射
  domain <域名>               | 域名映射 (写入 /etc/hosts)
  autostart [on|off]          | 开机自启动

镜像构建 (类似 Dockerfile):
  sb_lxc build [Incusfile]               | 构建镜像并启动容器 (一键)
  sb_lxc build --image-only [Incusfile]  | 只构建镜像不启动
  sb_lxc build --name <名> [Incusfile]   | 覆盖镜像/容器名
  sb_lxc build show                      | 列出可用的基础镜像
  sb_lxc run <镜像> [容器名]             | 从已构建镜像启动容器
  sb_lxc image init [路径]               | 交互式生成 Incusfile

  Incusfile 指令:
    FROM <镜像>   NAME <名称>     RUN <命令>
    COPY <源> <目标>   ENV <K=V>
    EXPOSE <端口>   DOMAIN <域名>   AUTOSTART on|off

其他:
  sb_lxc help                 | 显示此帮助

提示: [容器名] 省略时进入交互式选择菜单
`, Version)
}

// CmdImage 镜像子命令分发 (目前仅 init)。
func CmdImage(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: sb_lxc image init [路径]")
	}
	switch args[0] {
	case "init", "new", "create":
		return CmdImageInit(args[1:])
	case "build":
		return CmdBuild(args[1:])
	default:
		return fmt.Errorf("未知子命令: sb_lxc image %s (支持: init, build)", args[0])
	}
}
