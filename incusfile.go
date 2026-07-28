package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Incusfile 是 Dockerfile 风格的 Incus 镜像构建描述文件。
// sb_lxc build 读取该文件，启动一个临时构建容器，按顺序执行指令，
// 最后将容器发布为本地 Incus 镜像，可选地直接启动正式容器。
//
// 支持的指令：
//
//	FROM <image>          基础镜像 (如 debian/12, alpine/3.20, 也兼容 debian:12)
//	NAME <name>           镜像别名 + 容器名
//	RUN <command>         在容器内执行 shell 命令 (通过 /bin/sh -c)
//	COPY <src> <dst>      从宿主机复制文件/目录到容器
//	ENV <KEY>=<VALUE>     设置环境变量 (写入 /etc/environment + profile.d)
//	EXPOSE <port>[/<proto>] ...  声明端口映射 (运行时自动创建)
//	DOMAIN <domain>       域名映射 (运行时写入 /etc/hosts)
//	AUTOSTART on|off      开机自启动
type Incusfile struct {
	Path       string
	From       string
	Name       string
	Steps      []BuildStep // RUN/COPY/ENV 按出现顺序执行
	Exposes    []PortSpec
	Domain     string
	Autostart  *bool
}

// BuildStep 是一个有序的构建步骤 (RUN/COPY/ENV)。
type BuildStep struct {
	Kind string // "RUN", "COPY", "ENV"
	Run  string
	Copy CopySpec
	Env  EnvSpec
}

type CopySpec struct {
	Src string
	Dst string
}

type EnvSpec struct {
	Key   string
	Value string
}

type PortSpec struct {
	Port     int
	Protocol string
}

// parseIncusfile 从指定路径解析 Incusfile。path 为空时默认 ./Incusfile。
// 支持行尾反斜杠续行 (与 Dockerfile 一致)：行尾 \ 会把下一行拼接到当前行。
func parseIncusfile(path string) (*Incusfile, error) {
	if path == "" {
		path = "Incusfile"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	f := &Incusfile{Path: path}
	// 规范化换行：去掉 \r (兼容 Windows CRLF)，再按 \n 切分
	cleaned := strings.ReplaceAll(string(data), "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")
	// 预处理：合并以 \ 结尾的续行，记录每条逻辑行起始的物理行号
	rawLines := strings.Split(cleaned, "\n")
	type logicalLine struct {
		no   int
		text string
	}
	var logical []logicalLine
	for i := 0; i < len(rawLines); i++ {
		startNo := i + 1
		cur := rawLines[i]
		// 注释 / 空行不参与续行
		trimmed := strings.TrimSpace(cur)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			logical = append(logical, logicalLine{no: startNo, text: cur})
			continue
		}
		// 合并后续以 \ 结尾的行 (容忍反斜杠后的尾随空白，跳过续行中的注释/空行)
		for {
			trimmedCur := strings.TrimRight(cur, " \t")
			if !strings.HasSuffix(trimmedCur, "\\") || i+1 >= len(rawLines) {
				break
			}
			cur = strings.TrimSuffix(trimmedCur, "\\")
			i++
			// 跳过续行中的注释行和空行 (与 Dockerfile 行为一致：注释在续行中被移除)
			for i < len(rawLines) {
				next := strings.TrimSpace(rawLines[i])
				if next == "" || strings.HasPrefix(next, "#") {
					i++
					continue
				}
				break
			}
			if i >= len(rawLines) {
				break
			}
			cur += " " + strings.TrimLeft(rawLines[i], " \t")
		}
		logical = append(logical, logicalLine{no: startNo, text: cur})
	}
	for _, ll := range logical {
		lineNo := ll.no
		raw := ll.text
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		directive := strings.ToUpper(fields[0])
		payload := directivePayload(raw)
		switch directive {
		case "FROM":
			if payload == "" {
				return nil, fmt.Errorf("line %d: FROM 需要镜像引用", lineNo)
			}
			f.From = normalizeImageRef(payload)
		case "NAME":
			if payload == "" {
				return nil, fmt.Errorf("line %d: NAME 需要名称", lineNo)
			}
			f.Name = payload
		case "RUN":
			if payload == "" {
				return nil, fmt.Errorf("line %d: RUN 需要命令", lineNo)
			}
			f.Steps = append(f.Steps, BuildStep{Kind: "RUN", Run: payload})
		case "COPY":
			src, dst, err := parseCopyPayload(payload)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			f.Steps = append(f.Steps, BuildStep{Kind: "COPY", Copy: CopySpec{Src: src, Dst: dst}})
		case "ENV":
			kv, err := parseEnvPayload(payload)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			f.Steps = append(f.Steps, BuildStep{Kind: "ENV", Env: kv})
		case "EXPOSE":
			ports, err := parseExposePayload(payload)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			f.Exposes = append(f.Exposes, ports...)
		case "DOMAIN":
			if payload == "" {
				return nil, fmt.Errorf("line %d: DOMAIN 需要域名", lineNo)
			}
			f.Domain = payload
		case "AUTOSTART":
			on, err := parseBoolPayload(payload)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			f.Autostart = &on
		default:
			return nil, fmt.Errorf("line %d: 未知指令 %s (支持: FROM NAME RUN COPY ENV EXPOSE DOMAIN AUTOSTART)", lineNo, directive)
		}
	}
	if f.From == "" {
		return nil, fmt.Errorf("%s 缺少 FROM 指令", path)
	}
	return f, nil
}

// directivePayload 提取指令关键字后的全部内容，保留空格、引号等。
func directivePayload(rawLine string) string {
	trimmed := strings.TrimLeft(rawLine, " \t")
	idx := strings.IndexAny(trimmed, " \t")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[idx+1:])
}

// normalizeImageRef 兼容 Docker 风格 "debian:12" 与 Incus 风格 "debian/12"。
// 形如 "debian:12" -> "debian/12"；"debian/12" 不变。
func normalizeImageRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, ":") && !strings.Contains(ref, "/") {
		return strings.Replace(ref, ":", "/", 1)
	}
	return ref
}

// parseCopyPayload 解析 COPY 指令的 payload，支持引号包裹的含空格路径。
// COPY ./index.html /var/www/html/index.html
// COPY "my file.txt" "/path with spaces/file.txt"
func parseCopyPayload(payload string) (src, dst string, err error) {
	fields, err := shellSplit(payload)
	if err != nil {
		return "", "", fmt.Errorf("COPY 用法: COPY <src> <dst>: %w", err)
	}
	if len(fields) != 2 {
		return "", "", fmt.Errorf("COPY 用法: COPY <src> <dst> (得到 %d 个参数)", len(fields))
	}
	return fields[0], fields[1], nil
}

// shellSplit 按空白切分字符串，但支持双引号包裹的含空格字段。
// 例如: shellSplit(`a "b c" d`) -> ["a", "b c", "d"]
func shellSplit(s string) ([]string, error) {
	var result []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case (c == ' ' || c == '\t') && !inQuote:
			if cur.Len() > 0 {
				result = append(result, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("未闭合的引号")
	}
	if cur.Len() > 0 {
		result = append(result, cur.String())
	}
	return result, nil
}

func parseEnvPayload(payload string) (EnvSpec, error) {
	if idx := strings.IndexByte(payload, '='); idx > 0 {
		return EnvSpec{Key: strings.TrimSpace(payload[:idx]), Value: strings.TrimSpace(payload[idx+1:])}, nil
	}
	parts := strings.Fields(payload)
	if len(parts) < 2 {
		return EnvSpec{}, fmt.Errorf("ENV 用法: ENV KEY=VALUE 或 ENV KEY VALUE")
	}
	return EnvSpec{Key: parts[0], Value: strings.Join(parts[1:], " ")}, nil
}

func parseExposePayload(payload string) ([]PortSpec, error) {
	fields := strings.Fields(payload)
	if len(fields) == 0 {
		return nil, fmt.Errorf("EXPOSE 需要至少一个端口")
	}
	var result []PortSpec
	for _, f := range fields {
		proto := "tcp"
		portStr := f
		if idx := strings.LastIndex(f, "/"); idx > 0 {
			proto = strings.ToLower(strings.TrimSpace(f[idx+1:]))
			if proto != "tcp" && proto != "udp" {
				return nil, fmt.Errorf("协议必须是 tcp 或 udp, 得到 %q", proto)
			}
			portStr = f[:idx]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("端口 %q 无效", portStr)
		}
		result = append(result, PortSpec{Port: port, Protocol: proto})
	}
	return result, nil
}

func parseBoolPayload(payload string) (bool, error) {
	p := strings.ToLower(strings.TrimSpace(payload))
	switch p {
	case "on", "true", "yes", "1":
		return true, nil
	case "off", "false", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("期望 on/off, 得到 %q", payload)
}

// exposeString 序列化端口列表为紧凑字符串 (用于镜像属性)。
func exposeString(ports []PortSpec) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
	}
	return strings.Join(parts, ",")
}

// parseExposeString 从镜像属性恢复端口列表。
func parseExposeString(s string) []PortSpec {
	if s == "" {
		return nil
	}
	var result []PortSpec
	for _, part := range strings.Split(s, ",") {
		proto := "tcp"
		portStr := part
		if idx := strings.LastIndex(part, "/"); idx > 0 {
			proto = strings.ToLower(part[idx+1:])
			portStr = part[:idx]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		result = append(result, PortSpec{Port: port, Protocol: proto})
	}
	return result
}
