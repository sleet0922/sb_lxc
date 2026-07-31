package main

import (
	"fmt"
	"os"
	"sync"
)

// Version 工具版本
const Version = "1.9.2"

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

	// 启动期副作用按命令分层执行，避免只读命令承担不必要的网络探测开销。
	// - needsStartupSideEffects: 仅容器创建/启动类命令需要确保 default profile 的
	//   macvlan 配置就绪并清理遗留的 incusbr0 网桥；只读命令 (list/in/exec/set/stop/...)
	//   无需这些副作用，直接连接 Incus 即可，显著降低延迟。
	// - AutoConfigureHostMacvlan 不在 main 中调用：start/install/import/create 等命令
	//   会在容器获取 IPv4 后自行调用；main 提前调用既冗余（被它启动的容器此时还未运行、
	//   不在目标列表里），又会触发额外的 ping 探测，是命令延迟的主要来源。
	if needsStartupSideEffects(cmd) {
		client := NewIncusClient()
		client.EnsureMirrorRemote()
		// 两个维护任务相互独立，并行执行以缩短启动延迟。
		// Incus REST 客户端底层基于 http.Client，可安全并发使用。
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); warnAutoHostMacvlan(client.EnsureDefaultMacvlanProfile()) }()
		go func() { defer wg.Done(); _ = client.AutoCleanupUnusedBridges() }()
		wg.Wait()
	}

	if err := dispatch(cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "✘ %v\n", err)
		os.Exit(1)
	}
}

// isLightweightCommand 判断命令是否为轻量级（无需连接 Incus）。
// help/version 等纯本地命令跳过所有启动期副作用，保证即时响应。
func isLightweightCommand(cmd string) bool {
	switch cmd {
	case "help", "-h", "--help", "--version", "-v", "version":
		return true
	}
	return false
}

// needsStartupSideEffects 判断命令是否需要执行启动期维护副作用
// (EnsureDefaultMacvlanProfile + AutoCleanupUnusedBridges)。
// 只有创建/启动容器的命令需要 default profile 的 macvlan 配置就绪；
// 只读或纯配置命令 (list/images/in/exec/set/stop/remove/export) 跳过，
// 直接连接 Incus 执行，避免每次都做网络探测与网桥扫描带来的延迟。
func needsStartupSideEffects(cmd string) bool {
	switch cmd {
	case "install", "i", "create", "run", "build", "import", "start", "restart":
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
	case "remove", "rm":
		return CmdRemove(args)
	case "uninstall":
		// 旧别名: 行为同 remove container (兼容老用户)
		fmt.Println("提示: uninstall 已改名为 remove, 用法: sb_lxc remove container [名]")
		return CmdRemove(append([]string{"container"}, args...))
	case "build":
		return CmdBuild(args)
	case "create":
		return CmdCreate(args)
	case "run":
		// 旧别名兼容
		fmt.Println("提示: run 已改名为 create, 用法: sb_lxc create [容器名]")
		return CmdCreate(args)
	case "images", "image":
		return CmdImages(args)
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
  sb_lxc remove [container|image] [名] | 删除容器或镜像 (交互式选择)
  sb_lxc uninstall            | 删除容器 (旧别名, 等同 remove container)
  sb_lxc start   [容器名]     | 启动容器
  sb_lxc stop    [容器名]     | 停止容器
  sb_lxc restart [容器名]     | 重启容器
  sb_lxc in      [容器名]     | 进入容器 shell
  sb_lxc exec    <容器名> <命令...> | 在容器内执行命令 (非交互)
  sb_lxc export  [容器名]     | 导出容器为 tar.gz
  sb_lxc import  [文件] [名]  | 从 tar.gz 导入容器

容器设置 (sb_lxc set <容器名> ...):
  sb_lxc set <容器名> port [规格]      | 端口映射 (规格如 8080:80/tcp)
  sb_lxc set <容器名> port rm <规格>   | 取消端口映射
  sb_lxc set <容器名> port list        | 查看端口映射
  sb_lxc set <容器名> domain <域名>    | 域名映射 (写入 /etc/hosts)
  sb_lxc set <容器名> autostart [on|off] | 开机自启动

镜像构建 (类似 Dockerfile):
  sb_lxc build [Incusfile]               | 构建镜像 (默认 ./Incusfile)
  sb_lxc build --name <名> [Incusfile]   | 覆盖镜像别名
  sb_lxc build show                      | 列出可用的基础镜像
  sb_lxc create [容器名]                 | 从 ./Incusfile 读取镜像名并创建+启动容器
  sb_lxc images                          | 列出本地镜像别名 (build 产物)

  Incusfile 指令:
    FROM <镜像>   NAME <名称>     RUN <命令>
    COPY <源> <目标>   ENV <K=V>
    EXPOSE <端口>   DOMAIN <域名>   AUTOSTART on|off
    TEMP <名称> ... END   临时构建块 (隔离编译工具链)

其他:
  sb_lxc help                 | 显示此帮助

提示: [容器名] 省略时进入交互式选择菜单
`, Version)
}

// CmdImages 列出本地镜像别名 (sb_lxc build 的产物)。
// 显示别名、镜像指纹(短)、大小、创建时间。
func CmdImages(args []string) error {
	client := NewIncusClient()
	infos, err := client.ListLocalImageAliasesWithDetails()
	if err != nil {
		return fmt.Errorf("读取本地镜像列表失败: %w", err)
	}
	if len(infos) == 0 {
		fmt.Println("本地无镜像别名。用 'sb_lxc build' 构建镜像。")
		return nil
	}
	fmt.Printf("╭─ 本地镜像 (共 %d 个)\n", len(infos))
	fmt.Println("│ 别名                          大小      创建时间          指纹(短)")
	fmt.Println("│ ──────────────────────────── ──────── ──────────────── ────────────")
	for _, info := range infos {
		fp := info.Target
		if len(fp) > 12 {
			fp = fp[:12]
		}
		fmt.Printf("│ %-29s %-8s %-16s %s\n", truncName(info.Name, 29), humanSize(info.Size), info.CreatedAt.Format("2006-01-02 15:04"), fp)
	}
	fmt.Println("╰─")
	return nil
}

func truncName(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	switch exp {
	case 0:
		return fmt.Sprintf("%.1fK", float64(bytes)/float64(div))
	case 1:
		return fmt.Sprintf("%.1fM", float64(bytes)/float64(div))
	case 2:
		return fmt.Sprintf("%.1fG", float64(bytes)/float64(div))
	default:
		return fmt.Sprintf("%.1fT", float64(bytes)/float64(div))
	}
}
