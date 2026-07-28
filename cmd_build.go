package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CmdBuild 从 Incusfile 构建镜像并启动容器 (一键编排，类似 docker build + docker run)。
//
// 用法:
//
//	sb_lxc build [Incusfile]              构建镜像并启动容器 (默认 ./Incusfile)
//	sb_lxc build --image-only [Incusfile] 只构建镜像，不启动容器
//	sb_lxc build --name <name> [Incusfile] 覆盖镜像别名/容器名
//	sb_lxc build --help                   显示帮助
func CmdBuild(args []string) error {
	// 子命令: sb_lxc build show - 列出可用于 FROM 的基础镜像
	if len(args) > 0 && (args[0] == "show" || args[0] == "list" || args[0] == "images") {
		return showBuildBaseImages()
	}

	imageOnly := false
	overrideName := ""
	incusfilePath := ""

	i := 0
	for i < len(args) {
		arg := args[i]
		switch arg {
		case "--image-only", "--no-run":
			imageOnly = true
			i++
		case "--name", "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("--name 需要参数")
			}
			overrideName = args[i+1]
			i += 2
		case "--help", "-h":
			fmt.Print(buildUsage())
			return nil
		default:
			if strings.HasPrefix(arg, "--") {
				return fmt.Errorf("未知参数: %s (使用 --help 查看用法)", arg)
			}
			if incusfilePath != "" {
				return fmt.Errorf("只能指定一个 Incusfile 路径")
			}
			incusfilePath = arg
			i++
		}
	}

	f, err := parseIncusfile(incusfilePath)
	if err != nil {
		return err
	}
	if overrideName != "" {
		f.Name = overrideName
	}

	alias := f.Name
	if alias == "" {
		alias = defaultNameFromImage(f.From) + "-built"
	}

	client := NewIncusClient()

	runCount, copyCount, envCount := 0, 0, 0
	for _, s := range f.Steps {
		switch s.Kind {
		case "RUN":
			runCount++
		case "COPY":
			copyCount++
		case "ENV":
			envCount++
		}
	}

	fmt.Printf("╭─ Incusfile 构建\n")
	fmt.Printf("│ 文件:     %s\n", f.Path)
	fmt.Printf("│ 基础镜像: %s\n", f.From)
	fmt.Printf("│ 目标镜像: %s\n", alias)
	fmt.Printf("│ RUN: %d  COPY: %d  ENV: %d  EXPOSE: %d  步骤: %d\n", runCount, copyCount, envCount, len(f.Exposes), len(f.Steps))
	if f.Domain != "" {
		fmt.Printf("│ DOMAIN:   %s\n", f.Domain)
	}
	if f.Autostart != nil {
		fmt.Printf("│ AUTOSTART: %s\n", strconv.FormatBool(*f.Autostart))
	}
	fmt.Printf("│ 启动容器: %s\n", strconv.FormatBool(!imageOnly))
	fmt.Printf("╰─\n\n")

	if err := buildImage(client, f, alias); err != nil {
		return err
	}

	if imageOnly {
		fmt.Printf("\n✔ 镜像 %s 构建完成 (未启动容器)\n", alias)
		fmt.Printf("  使用 'sb_lxc run %s' 启动容器\n", alias)
		return nil
	}

	if err := runFromBuiltImage(client, alias, f); err != nil {
		return err
	}
	fmt.Printf("\n✔ 一键构建并启动完成! 镜像=%s  容器=%s\n", alias, f.Name)
	return nil
}

func buildUsage() string {
	return `sb_lxc build - 从 Incusfile 构建镜像并启动容器

用法:
  sb_lxc build [Incusfile]                构建镜像并启动容器 (默认 ./Incusfile)
  sb_lxc build --image-only [Incusfile]   只构建镜像
  sb_lxc build --name <name> [Incusfile]  覆盖镜像/容器名
  sb_lxc build show                       列出可用于 FROM 的基础镜像

Incusfile 指令:
  FROM <image>              基础镜像 (如 debian/12 或 debian:12)
  NAME <name>               镜像别名 + 容器名
  RUN <command>             在容器内执行 shell 命令
  COPY <src> <dst>          从宿主机复制文件/目录到容器
  ENV <KEY>=<VALUE>         设置环境变量
  EXPOSE <port>[/<proto>]   声明端口映射 (可多个，空格分隔)
  DOMAIN <domain>           域名映射
  AUTOSTART on|off          开机自启动

示例 Incusfile:
  FROM debian/12
  NAME my-nginx
  RUN apt-get update && apt-get install -y nginx
  RUN echo "daemon off;" >> /etc/nginx/nginx.conf
  COPY ./index.html /var/www/html/index.html
  EXPOSE 80/tcp
  DOMAIN nginx.test
  AUTOSTART on
`
}

// showBuildBaseImages 列出远程镜像源中可用于 FROM 指令的基础镜像。
// 输出按发行版分组，每行一个版本，可直接复制到 Incusfile 的 FROM 行。
func showBuildBaseImages() error {
	fmt.Println("正在从镜像源获取可用基础镜像 ...")
	client := NewIncusClient()
	groups, err := client.ListImages()
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return fmt.Errorf("未找到可用基础镜像")
	}

	arch := archName()
	total := 0
	for _, g := range groups {
		total += len(g.Versions)
	}
	fmt.Printf("╭─ 可用基础镜像 (架构: %s, 共 %d 个发行版 %d 个版本)\n", arch, len(groups), total)
	fmt.Println("│ 可直接用于 Incusfile 的 FROM 指令")
	fmt.Println("│")
	for _, g := range groups {
		fmt.Printf("│ %s\n", g.Distro)
		for _, v := range g.Versions {
			fmt.Printf("│   FROM %s   # %s\n", v.Image, v.Release)
		}
	}
	fmt.Println("│")
	fmt.Println("│ 提示: FROM 同时接受 debian/12 与 debian:12 两种写法")
	fmt.Println("╰─")
	return nil
}

// autoConfigureAptMirror 检测容器内 apt 官方源连通性，失败则自动换为清华镜像源。
// 仅对 Debian/Ubuntu 系容器生效；其他发行版 (Alpine/CentOS 等) 跳过。
// 这样用户无需在 Incusfile 里手动加 sed 换源命令。
func autoConfigureAptMirror(client *IncusClient, name string) error {
	// 检测是否为 Debian/Ubuntu
	osRelease, err := client.ReadFile(name, "/etc/os-release")
	if err != nil {
		return nil // 读不到就跳过，不阻塞构建
	}
	if !strings.Contains(osRelease, "ID=debian") && !strings.Contains(osRelease, "ID=ubuntu") {
		return nil // 非 Debian/Ubuntu，跳过
	}

	// 测试官方源连通性 (8 秒超时，避免阻塞构建)
	// 优先用 curl；没有 curl 则用 wget；都没有则不换源 (无法确认是否为网络问题)
	officialHost := "http://deb.debian.org/"
	if strings.Contains(osRelease, "ID=ubuntu") {
		officialHost = "http://archive.ubuntu.com/"
	}

	// 检测工具可用性：必须有 curl 或 wget 之一才能确认网络问题
	_, hasToolErr := client.execQuiet(name, "sh", "-c", "command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1")
	if hasToolErr != nil {
		// curl 和 wget 都没装，无法确认网络问题，跳过自动换源
		// (用户可在 Incusfile 里 RUN apt-get update 看实际报错)
		return nil
	}

	// 用任意一种工具测试官方源连通性
	testCmd := fmt.Sprintf(`command -v curl >/dev/null 2>&1 && curl -sI --max-time 8 -o /dev/null -w '%%{http_code}' %s 2>/dev/null | grep -qE '^[23][0-9][0-9]$' ||
command -v wget >/dev/null 2>&1 && wget -q --timeout=8 --spider %s 2>/dev/null ||
exit 1`, officialHost, officialHost)
	_, testErr := client.execQuiet(name, "sh", "-c", testCmd)
	if testErr == nil {
		return nil // 官方源可访问，不换源
	}

	// 已确认官方源不可达，自动换为清华镜像源
	fmt.Printf("  ⚠ 检测到 apt 官方源不可达，自动换为清华镜像源\n")

	// Debian 12+ 使用 deb822 格式 (/etc/apt/sources.list.d/debian.sources)
	// Debian 11 及更早使用传统格式 (/etc/apt/sources.list)
	// Ubuntu 使用 /etc/apt/sources.list
	// 用 sh 一次性处理所有可能的源文件格式
	sedScript := `set -e
# Debian deb822 格式 (12+)
if [ -f /etc/apt/sources.list.d/debian.sources ]; then
	sed -i 's|http://deb.debian.org|https://mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list.d/debian.sources
	sed -i 's|http://security.debian.org|https://mirrors.tuna.tsinghua.edu.cn/debian-security|g' /etc/apt/sources.list.d/debian.sources
fi
# Debian 传统格式 (<=11) 与 Ubuntu
if [ -f /etc/apt/sources.list ]; then
	sed -i 's|http://deb.debian.org|https://mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list
	sed -i 's|http://security.debian.org|https://mirrors.tuna.tsinghua.edu.cn/debian-security|g' /etc/apt/sources.list
	sed -i 's|http://archive.ubuntu.com|https://mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list
	sed -i 's|http://security.ubuntu.com|https://mirrors.tuna.tsinghua.edu.cn|g' /etc/apt/sources.list
fi
echo "✔ 镜像源已切换为清华源"`
	if err := client.ExecStreaming(name, sedScript, nil); err != nil {
		return fmt.Errorf("切换镜像源失败: %w", err)
	}
	return nil
}

// buildImage 执行构建流程：启动临时容器 -> 按顺序执行 Steps -> 发布镜像 -> 清理。
// Steps 严格按 Incusfile 中的出现顺序执行 (RUN/COPY/ENV)，确保 COPY 依赖的目录
// 能由前置 RUN 创建。
func buildImage(client *IncusClient, f *Incusfile, alias string) error {
	// 构建容器名加入 PID 与随机后缀，避免并发构建碰撞
	buildName := fmt.Sprintf("sb-lxc-build-%d-%d", os.Getpid(), time.Now().UnixNano())

	// 清理同名旧别名 (含其引用的孤儿镜像)
	if err := client.ReplaceImageAlias(alias); err != nil {
		fmt.Printf("⚠ 清理旧镜像别名失败: %v\n", err)
	}

	// 启动构建容器
	fmt.Printf("▶ [1/4] 启动构建容器 %s (镜像 %s) ...\n", buildName, f.From)
	if err := client.Launch(f.From, buildName); err != nil {
		return fmt.Errorf("启动构建容器失败: %w", err)
	}

	// 确保清理临时容器：先尝试优雅停止，失败则强制停止，确保 Delete 不会因容器仍在运行而失败
	defer func() {
		fmt.Printf("▶ 清理构建容器 %s ...\n", buildName)
		if err := client.Stop(buildName); err != nil {
			// 优雅停止失败（容器挂起/超时），强制停止以保证后续 Delete 成功
			fmt.Fprintf(os.Stderr, "  ⚠ 优雅停止失败 (%v)，强制停止 ...\n", err)
			_ = client.StopForce(buildName)
		}
		if err := client.Delete(buildName); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ 删除构建容器失败: %v\n", err)
		}
	}()

	// 等待网络就绪 (RUN 中常有 apt-get/apk 等需要网络)
	fmt.Printf("▶ [2/4] 等待容器网络就绪 ...\n")
	ip := waitForIP(client, buildName, 30)
	if ip == "" {
		fmt.Printf("⚠ 构建容器未获取 IPv4，RUN 命令可能因网络问题失败\n")
	} else {
		fmt.Printf("  构建容器 IPv4: %s\n", ip)
	}

	// 自动配置镜像源：检测到官方源不可达时，自动换为国内镜像源
	// 适用于 Debian/Ubuntu 系容器 (apt-get 场景)，避免 deb.debian.org 在国内访问超时
	if ip != "" {
		if err := autoConfigureAptMirror(client, buildName); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ 自动配置镜像源失败: %v\n", err)
		}
	}

	// 按顺序执行 Steps (RUN/COPY/ENV 严格按 Incusfile 顺序)
	fmt.Printf("▶ [3/4] 执行 %d 个构建步骤 ...\n", len(f.Steps))
	runEnv := map[string]string{}
	var collectedEnvs []EnvSpec
	contextDir := filepath.Dir(f.Path)
	total := len(f.Steps)
	for i, s := range f.Steps {
		switch s.Kind {
		case "ENV":
			fmt.Printf("  [%d/%d] ENV %s=%s\n", i+1, total, s.Env.Key, s.Env.Value)
			runEnv[s.Env.Key] = s.Env.Value
			collectedEnvs = append(collectedEnvs, s.Env)
		case "COPY":
			fmt.Printf("  [%d/%d] COPY %s -> %s\n", i+1, total, s.Copy.Src, s.Copy.Dst)
			if err := applyCopy(client, buildName, s.Copy, contextDir); err != nil {
				return err
			}
		case "RUN":
			fmt.Printf("  [%d/%d] RUN: %s\n", i+1, total, s.Run)
			if err := client.ExecStreaming(buildName, s.Run, runEnv); err != nil {
				return fmt.Errorf("RUN 失败: %s\n  %w", s.Run, err)
			}
		}
	}

	// 将所有 ENV 持久化到容器文件系统 (供镜像运行时使用)
	if len(collectedEnvs) > 0 {
		if err := applyEnvs(client, buildName, collectedEnvs); err != nil {
			return err
		}
	}

	// 停止构建容器，准备发布镜像
	fmt.Printf("▶ [4/4] 停止构建容器并发布镜像 ...\n")
	if err := client.Stop(buildName); err != nil {
		return fmt.Errorf("停止构建容器失败: %w", err)
	}

	// 发布为本地镜像
	fmt.Printf("▶ 发布镜像 %s ...\n", alias)
	properties := buildImageProperties(f)
	if err := client.PublishImage(buildName, alias, properties); err != nil {
		return fmt.Errorf("发布镜像失败: %w", err)
	}
	fmt.Printf("✔ 镜像已发布: %s\n", alias)
	return nil
}

// buildImageProperties 将 Incusfile 的运行时指令编码为镜像属性，供 sb_lxc run 读取。
func buildImageProperties(f *Incusfile) map[string]string {
	p := map[string]string{}
	if f.Name != "" {
		p["user.sb_lxc.name"] = f.Name
	}
	if len(f.Exposes) > 0 {
		p["user.sb_lxc.expose"] = exposeString(f.Exposes)
	}
	if f.Domain != "" {
		p["user.sb_lxc.domain"] = f.Domain
	}
	if f.Autostart != nil {
		p["user.sb_lxc.autostart"] = strconv.FormatBool(*f.Autostart)
	}
	return p
}

// runFromBuiltImage 从已构建的镜像启动正式容器，并应用 EXPOSE/DOMAIN/AUTOSTART。
func runFromBuiltImage(client *IncusClient, alias string, f *Incusfile) error {
	name := f.Name
	if name == "" {
		name = defaultNameFromImage(alias)
	}

	// 检查同名容器是否已存在
	if existing, _ := client.GetContainer(name); existing != nil {
		return fmt.Errorf("容器 %s 已存在，请先卸载或使用 --name 指定其他名称", name)
	}

	fmt.Printf("▶ 启动容器 %s (镜像 %s) ...\n", name, alias)
	if err := client.LaunchLocalImage(alias, name); err != nil {
		return fmt.Errorf("启动容器失败: %w", err)
	}

	ip := waitForIP(client, name, 30)
	if ip != "" {
		warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))
	} else {
		fmt.Printf("⚠ 容器未获取 IPv4，端口映射与域名映射将跳过\n")
	}

	// AUTOSTART
	if f.Autostart != nil {
		if err := client.SetBootAutostart(name, *f.Autostart); err != nil {
			fmt.Printf("⚠ 设置 AUTOSTART 失败: %v\n", err)
		}
	}

	// DOMAIN
	if f.Domain != "" {
		if err := client.SetDomain(name, f.Domain); err != nil {
			fmt.Printf("⚠ 设置 DOMAIN 失败: %v\n", err)
		} else if ip != "" {
			if err := updateHosts(name, f.Domain, ip); err != nil {
				fmt.Printf("⚠ 更新 /etc/hosts 失败: %v\n", err)
			} else {
				fmt.Printf("✔ 域名映射: %s -> %s\n", f.Domain, ip)
			}
		}
	}

	// EXPOSE -> 端口映射
	if len(f.Exposes) > 0 && ip != "" {
		for _, exp := range f.Exposes {
			if err := client.AddPortMapping(name, exp.Port, exp.Port, exp.Protocol); err != nil {
				fmt.Printf("⚠ 端口映射 %d/%s 失败: %v\n", exp.Port, exp.Protocol, err)
			} else {
				fmt.Printf("✔ 端口映射: %d/%s\n", exp.Port, exp.Protocol)
			}
		}
	}

	return nil
}

// applyEnvs 将 ENV 指令写入容器的 /etc/environment 和 /etc/profile.d/sb_lxc-env.sh。
// /etc/environment 由 PAM 在登录会话中加载，profile.d 脚本由 sh 登录 shell 加载。
// 幂等性：/etc/sb_lxc.env 为覆盖式写入；/etc/environment 先移除 sb_lxc 管理的行再追加，避免重复构建污染。
func applyEnvs(client *IncusClient, name string, envs []EnvSpec) error {
	if len(envs) == 0 {
		return nil
	}
	var envBuf bytes.Buffer
	for _, e := range envs {
		envBuf.WriteString(fmt.Sprintf("%s=%s\n", e.Key, e.Value))
	}
	if err := client.PushFile(name, "/etc/sb_lxc.env", envBuf.Bytes(), "0644"); err != nil {
		return fmt.Errorf("写入 /etc/sb_lxc.env 失败: %w", err)
	}

	// profile.d 脚本: 让登录 shell 自动导出变量
	profile := "#!/bin/sh\n# Generated by sb_lxc build\n[ -f /etc/sb_lxc.env ] && set -a && . /etc/sb_lxc.env && set +a\n"
	if err := client.PushFile(name, "/etc/profile.d/sb_lxc-env.sh", []byte(profile), "0755"); err != nil {
		return fmt.Errorf("写入 /etc/profile.d/sb_lxc-env.sh 失败: %w", err)
	}

	// /etc/environment (PAM 读取)：移除 sb_lxc 已管理的 KEY= 行（幂等），再追加本次的
	envKeys := map[string]bool{}
	for _, e := range envs {
		envKeys[e.Key+"="] = true
	}
	existing, _ := client.ReadFile(name, "/etc/environment")
	var envFile bytes.Buffer
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		skip := false
		for prefix := range envKeys {
			if strings.HasPrefix(trimmed, prefix) {
				skip = true
				break
			}
		}
		if !skip && trimmed != "" {
			envFile.WriteString(line + "\n")
		}
	}
	for _, e := range envs {
		envFile.WriteString(fmt.Sprintf("%s=%s\n", e.Key, e.Value))
	}
	if err := client.PushFile(name, "/etc/environment", envFile.Bytes(), "0644"); err != nil {
		return fmt.Errorf("更新 /etc/environment 失败: %w", err)
	}
	return nil
}

// applyCopy 执行单条 COPY 指令，src 相对于 Incusfile 所在目录。
// 安全约束：解析后的 src 必须位于 contextDir 之内，拒绝路径穿越 (如 ../../etc/passwd)。
func applyCopy(client *IncusClient, name string, cp CopySpec, contextDir string) error {
	src := cp.Src
	if !filepath.IsAbs(src) {
		src = filepath.Join(contextDir, src)
	}
	src = filepath.Clean(src)
	// 路径穿越防护：src 必须位于 contextDir 之内
	// 注意 src 与 contextDir 都必须转为绝对路径，否则 filepath.Rel 会报错误判。
	absContext, err := filepath.Abs(contextDir)
	if err != nil {
		return fmt.Errorf("解析 contextDir 失败: %w", err)
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("解析 src 失败: %w", err)
	}
	rel, err := filepath.Rel(absContext, absSrc)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("COPY 源 %q 位于构建上下文之外 (路径穿越被拒绝)", cp.Src)
	}
	return copyToContainer(client, name, absSrc, cp.Dst)
}

// copyToContainer 将宿主机文件/目录复制到容器。目录会递归复制。
// 不跟随符号链接，避免绕过路径穿越校验。
func copyToContainer(client *IncusClient, name, src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("源文件 %s 不存在: %w", src, err)
	}
	if info.IsDir() {
		return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			// 拒绝符号链接，防止穿越
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("COPY 不支持符号链接: %s", path)
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.ToSlash(filepath.Join(dst, rel))
			return pushFileToContainer(client, name, path, target, fi.Mode())
		})
	}
	// 源是符号链接则拒绝
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("COPY 不支持符号链接: %s", src)
	}
	return pushFileToContainer(client, name, src, dst, info.Mode())
}

func pushFileToContainer(client *IncusClient, name, srcPath, dstPath string, mode os.FileMode) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", srcPath, err)
	}
	modeStr := fmt.Sprintf("%04o", mode.Perm())
	return client.PushFile(name, dstPath, content, modeStr)
}

// CmdRun 从已构建的本地镜像启动容器 (类似 docker run)。
// 读取镜像构建时保存的 EXPOSE/DOMAIN/AUTOSTART 属性并自动应用。
//
// 用法:
//
//	sb_lxc run <镜像别名> [容器名]
func CmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: sb_lxc run <镜像别名> [容器名]")
	}
	alias := args[0]
	name := ""
	if len(args) >= 2 {
		name = args[1]
	}

	client := NewIncusClient()

	// 读取镜像构建时保存的属性
	props, err := client.GetImageProperties(alias)
	if err != nil {
		fmt.Printf("⚠ 读取镜像属性失败: %v (将使用默认配置)\n", err)
		props = map[string]string{}
	}

	f := &Incusfile{
		From:    alias,
		Exposes: parseExposeString(props["user.sb_lxc.expose"]),
		Domain:  props["user.sb_lxc.domain"],
	}
	if name != "" {
		f.Name = name
	} else if v := props["user.sb_lxc.name"]; v != "" {
		f.Name = v
	} else {
		f.Name = defaultNameFromImage(alias)
	}
	if v, ok := props["user.sb_lxc.autostart"]; ok {
		on, _ := strconv.ParseBool(v)
		f.Autostart = &on
	}

	fmt.Printf("▶ 从镜像 %s 启动容器 %s\n", alias, f.Name)
	if len(f.Exposes) > 0 {
		fmt.Printf("  EXPOSE: %s\n", exposeString(f.Exposes))
	}
	if f.Domain != "" {
		fmt.Printf("  DOMAIN:  %s\n", f.Domain)
	}
	if f.Autostart != nil {
		fmt.Printf("  AUTOSTART: %s\n", strconv.FormatBool(*f.Autostart))
	}
	fmt.Println()

	return runFromBuiltImage(client, alias, f)
}

// CmdImageInit 交互式引导生成 Incusfile。
//
// 用法:
//
//	sb_lxc image init [输出路径]   默认输出到 ./Incusfile
func CmdImageInit(args []string) error {
	fmt.Println("╭─ Incusfile 引导生成器")
	fmt.Println("│ 类似 Dockerfile，用于一键构建 Incus 容器镜像")
	fmt.Println("╰─")
	fmt.Println()

	// FROM
	fmt.Println("常用基础镜像:")
	bases := []struct{ Label, Image string }{
		{"Alpine 3.20", "alpine/3.20"},
		{"Alpine 3.21", "alpine/3.21"},
		{"Debian 12 (bookworm)", "debian/12"},
		{"Debian 13 (trixie)", "debian/13"},
		{"Ubuntu 22.04 (jammy)", "ubuntu/22.04"},
		{"Ubuntu 24.04 (noble)", "ubuntu/24.04"},
		{"Rocky Linux 9", "rockylinux/9"},
		{"CentOS Stream 9", "centos/stream9"},
		{"自定义镜像引用", ""},
	}
	labels := make([]string, len(bases))
	for i, b := range bases {
		labels[i] = b.Label
	}
	choice := selectMenu(labels, "选择基础镜像 (↑↓ 选择, Enter 确认, q 退出)")
	if choice < 0 {
		return nil
	}
	from := bases[choice].Image
	if from == "" {
		from = prompt("输入镜像引用 (如 debian/12 或 debian:12): ")
		if from == "" {
			return fmt.Errorf("镜像引用不能为空")
		}
	}

	// NAME
	defaultName := defaultNameFromImage(from)
	name := prompt(fmt.Sprintf("镜像/容器名 (回车默认 %s): ", defaultName))
	if name == "" {
		name = defaultName
	}

	// RUN
	fmt.Println("\nRUN 命令 (在容器内执行的 shell 命令，每行一条，空行结束):")
	fmt.Println("  示例: apt-get update && apt-get install -y nginx")
	var runs []string
	for {
		cmd := prompt("  RUN> ")
		if cmd == "" {
			break
		}
		runs = append(runs, cmd)
	}

	// COPY
	var copies []CopySpec
	fmt.Println("\nCOPY 文件 (从宿主机复制到容器，空行结束):")
	for {
		src := prompt("  COPY 源文件 (回车结束): ")
		if src == "" {
			break
		}
		dst := prompt("  COPY 目标路径: ")
		if dst == "" {
			fmt.Println("  ⚠ 目标路径不能为空，跳过")
			continue
		}
		copies = append(copies, CopySpec{Src: src, Dst: dst})
	}

	// ENV
	var envs []EnvSpec
	fmt.Println("\nENV 环境变量 (空行结束):")
	for {
		kv := prompt("  ENV KEY=VALUE (回车结束): ")
		if kv == "" {
			break
		}
		e, err := parseEnvPayload(kv)
		if err != nil {
			fmt.Printf("  ⚠ %v\n", err)
			continue
		}
		envs = append(envs, e)
	}

	// EXPOSE
	var exposes []PortSpec
	fmt.Println("\nEXPOSE 端口 (空行结束):")
	for {
		p := prompt("  EXPOSE 端口[/协议] (回车结束): ")
		if p == "" {
			break
		}
		ports, err := parseExposePayload(p)
		if err != nil {
			fmt.Printf("  ⚠ %v\n", err)
			continue
		}
		exposes = append(exposes, ports...)
	}

	// DOMAIN
	domain := prompt("\nDOMAIN 域名映射 (回车跳过): ")

	// AUTOSTART
	autostart := ""
	aChoice := selectMenu([]string{"关闭", "开启"}, "开机自启动 (↑↓ 选择, Enter 确认)")
	if aChoice == 1 {
		autostart = "on"
	}

	// 生成 Incusfile 内容
	var buf bytes.Buffer
	buf.WriteString("# Incusfile - sb_lxc 镜像构建描述文件\n")
	buf.WriteString("# 生成时间: " + time.Now().Format("2006-01-02 15:04:05") + "\n")
	buf.WriteString("# 用法: sb_lxc build\n\n")
	buf.WriteString("FROM " + from + "\n")
	buf.WriteString("NAME " + name + "\n")
	for _, r := range runs {
		buf.WriteString("RUN " + r + "\n")
	}
	for _, c := range copies {
		buf.WriteString(fmt.Sprintf("COPY %s %s\n", c.Src, c.Dst))
	}
	for _, e := range envs {
		buf.WriteString(fmt.Sprintf("ENV %s=%s\n", e.Key, e.Value))
	}
	if len(exposes) > 0 {
		parts := make([]string, 0, len(exposes))
		for _, e := range exposes {
			parts = append(parts, fmt.Sprintf("%d/%s", e.Port, e.Protocol))
		}
		buf.WriteString("EXPOSE " + strings.Join(parts, " ") + "\n")
	}
	if domain != "" {
		buf.WriteString("DOMAIN " + domain + "\n")
	}
	if autostart != "" {
		buf.WriteString("AUTOSTART " + autostart + "\n")
	}

	// 写入文件
	path := "Incusfile"
	if len(args) >= 1 {
		path = args[0]
	}
	if _, err := os.Stat(path); err == nil {
		confirm := prompt(fmt.Sprintf("⚠ %s 已存在，覆盖? (y/N): ", path))
		if !strings.EqualFold(confirm, "y") {
			fmt.Println("已取消")
			return nil
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("\n✔ 已生成 %s (%d 字节)\n", path, buf.Len())
	fmt.Println()
	fmt.Println("─── 文件内容预览 ───")
	fmt.Print(buf.String())
	fmt.Println("─── 预览结束 ───")
	fmt.Println()
	fmt.Printf("下一步:\n")
	fmt.Printf("  sb_lxc build %s        # 构建镜像并启动容器 (一键)\n", path)
	fmt.Printf("  sb_lxc build --image-only %s   # 只构建镜像\n", path)
	return nil
}
