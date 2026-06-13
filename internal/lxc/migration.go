package lxc

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sb_lxc/internal/core"
)

// ContainerMeta 导出导入用的容器元数据
type ContainerMeta struct {
	Version       int            `json:"version"`
	Name          string         `json:"name"`
	Autostart     bool           `json:"autostart"`
	PortForwards  []PortForward  `json:"port_forwards"`
	DomainMappings []string      `json:"domain_mappings"` // 域名列表
}

// GatherMeta 收集容器的所有元数据（端口映射、域名映射、自启状态）
func GatherMeta(name string) (*ContainerMeta, error) {
	meta := &ContainerMeta{
		Version: 1,
		Name:    name,
	}

	// 读取自启配置
	configPath := filepath.Join("/var/lib/lxc", name, "config")
	if data, err := os.ReadFile(configPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "lxc.start.auto") {
				if strings.Contains(line, "= 1") || strings.HasSuffix(strings.TrimSpace(line), "=1") {
					meta.Autostart = true
				}
			}
		}
	}

	// 读取端口映射
	netSvc := NewNetworkService(&core.ShellExecutor{})
	forwards, _ := netSvc.ListPortForwards(name)
	meta.PortForwards = forwards

	// 读取域名映射（从 domain-hosts.sh 脚本中提取域名）
	hookPath := filepath.Join("/var/lib/lxc", name, "domain-hosts.sh")
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			// 从 "sed -i '/[[:space:]]example.com$/d' /etc/hosts" 中提取域名
			if strings.Contains(line, "sed -i") && strings.Contains(line, "/etc/hosts") {
				// 提取 [[:space:]] 和 $ 之间的域名
				start := strings.Index(line, "[[:space:]]")
				if start > 0 {
					rest := line[start+len("[[:space:]}"):]
					if end := strings.Index(rest, "$"); end > 0 {
						domain := rest[:end]
						domain = strings.TrimRight(domain, `/\`)
						if domain != "" {
							meta.DomainMappings = append(meta.DomainMappings, domain)
						}
					}
				}
			}
		}
	}

	return meta, nil
}

// ExportContainer 导出容器为 tar.gz 文件。
// 流程：收集元数据 → 停容器 → tar rootfs → 启容器 → 打包
func ExportContainer(name, outputPath string) error {
	meta, err := GatherMeta(name)
	if err != nil {
		return fmt.Errorf("收集元数据失败: %w", err)
	}

	svc := NewContainerService(&core.ShellExecutor{})

	// 检查容器是否在运行，如运行则先停止
	wasRunning := false
	info, _ := svc.Info(name)
	if strings.Contains(info, "RUNNING") {
		wasRunning = true
		fmt.Printf("容器 %s 正在运行，暂时停止以导出...\n", name)
		if out, err := svc.Stop(name, false); err != nil {
			return fmt.Errorf("停止容器失败: %w\n%s", err, out)
		}
	}

	// 停止后的恢复保证
	defer func() {
		if wasRunning {
			if out, err := svc.Start(name, true); err != nil {
				fmt.Printf("警告: 恢复启动容器失败: %v\n%s\n", err, out)
			} else {
				fmt.Printf("容器 %s 已恢复运行\n", name)
			}
		}
	}()

	rootfsDir := filepath.Join("/var/lib/lxc", name, "rootfs")
	if _, err := os.Stat(rootfsDir); err != nil {
		return fmt.Errorf("rootfs 目录不存在: %s", rootfsDir)
	}

	// 创建输出文件
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// 写入 metadata.json
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	if err := writeTarEntry(tarWriter, "metadata.json", metaJSON); err != nil {
		return err
	}

	// 直接打包 rootfs 目录
	fmt.Printf("正在打包 rootfs ...\n")
	err = filepath.Walk(rootfsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(rootfsDir, path)
		if rel == "." {
			return nil
		}
		tarPath := "rootfs/" + rel

		// 处理符号链接 — filepath.Walk 不跟踪符号链接，需单独处理
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("读取符号链接 %s 失败: %w", path, err)
			}
			return tarWriter.WriteHeader(&tar.Header{
				Name:     tarPath,
				Linkname: link,
				Mode:     int64(info.Mode()),
				Typeflag: tar.TypeSymlink,
			})
		}

		if info.IsDir() {
			return tarWriter.WriteHeader(&tar.Header{
				Name:     tarPath + "/",
				Mode:     0755,
				Typeflag: tar.TypeDir,
			})
		}

		// 跳过设备文件、socket 等特殊文件
		if !info.Mode().IsRegular() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", path, err)
		}

		if err := tarWriter.WriteHeader(&tar.Header{
			Name: tarPath,
			Mode: int64(info.Mode()),
			Size: info.Size(),
		}); err != nil {
			return err
		}
		_, err = tarWriter.Write(data)
		return err
	})
	if err != nil {
		return fmt.Errorf("打包 rootfs 失败: %w", err)
	}

	fmt.Printf("已导出容器 %s 到 %s\n", name, outputPath)
	fmt.Printf("  端口映射: %d 条\n", len(meta.PortForwards))
	fmt.Printf("  域名映射: %d 条\n", len(meta.DomainMappings))
	fmt.Printf("  开机自启: %v\n", meta.Autostart)
	return nil
}

// ParseMeta 从导出的 tar.gz 归档中仅读取元数据（不执行导入）。
func ParseMeta(archivePath string) (*ContainerMeta, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("打开归档文件失败: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("解压失败: %w", err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取归档失败: %w", err)
		}
		if hdr.Name == "metadata.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("读取元数据失败: %w", err)
			}
			meta := &ContainerMeta{}
			if err := json.Unmarshal(data, meta); err != nil {
				return nil, fmt.Errorf("解析元数据失败: %w", err)
			}
			return meta, nil
		}
	}
	return nil, fmt.Errorf("归档中未找到元数据")
}

// ImportContainer 从 tar.gz 文件导入容器。
// newName 为空则使用元数据中的原名。
// 流程：解包 → 写入 rootfs → 生成清洁 config → 应用端口/域名映射
func ImportContainer(archivePath, newName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("打开归档文件失败: %w", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)

	var meta *ContainerMeta
	type rootfsEntry struct {
		name     string
		mode     int64
		linkname string // 符号链接目标
		isDir    bool
		isLink   bool
		content  []byte
	}
	var rootfsFiles []rootfsEntry

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取归档失败: %w", err)
		}

		if hdr.Name == "metadata.json" {
			data, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("读取元数据失败: %w", err)
			}
			meta = &ContainerMeta{}
			if err := json.Unmarshal(data, meta); err != nil {
				return fmt.Errorf("解析元数据失败: %w", err)
			}
		} else if strings.HasPrefix(hdr.Name, "rootfs/") {
			rel := strings.TrimPrefix(hdr.Name, "rootfs/")
			entry := rootfsEntry{
				name:     rel,
				mode:     hdr.Mode,
				linkname: hdr.Linkname,
				isDir:    hdr.Typeflag == tar.TypeDir,
				isLink:   hdr.Typeflag == tar.TypeSymlink,
			}
			if !entry.isDir && !entry.isLink {
				data, err := io.ReadAll(tarReader)
				if err != nil {
					return fmt.Errorf("读取 rootfs 文件 %s 失败: %w", hdr.Name, err)
				}
				entry.content = data
			}
			rootfsFiles = append(rootfsFiles, entry)
		}
	}

	if meta == nil {
		return fmt.Errorf("归档中未找到元数据")
	}

	// 支持重命名
	name := meta.Name
	if newName != "" {
		fmt.Printf("以新名称 %s 导入 (原名: %s)\n", newName, name)
		name = newName
	}
	lxcDir := filepath.Join("/var/lib/lxc", name)

	// 检查容器是否已存在
	if _, err := os.Stat(lxcDir); err == nil {
		return fmt.Errorf("容器 %s 已存在，请先卸载或使用 --name 指定新名称", name)
	}

	// 提前清理旧残留（防止上次未正常卸载的 iptables 规则、hosts 记录等干扰）
	fmt.Printf("正在清理 %s 的残留配置...\n", name)
	clearContainerRules(name)
	for _, domain := range meta.DomainMappings {
		exec.Command("sed", "-i", "/[[:space:]]"+domain+"$/d", "/etc/hosts").Run()
	}
	exec.Command("systemctl", "disable", "sb-lxc-port@"+name+".service", "sb-lxc-domain@"+name+".service").Run()

	// 写入 rootfs
	rootfsDir := filepath.Join(lxcDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("创建 rootfs 目录失败: %w", err)
	}

	fmt.Printf("正在恢复 rootfs ...\n")
	for _, f := range rootfsFiles {
		targetPath := filepath.Join(rootfsDir, f.name)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(targetPath), err)
		}

		if f.isDir {
			if err := os.MkdirAll(targetPath, os.FileMode(f.mode)); err != nil {
				return fmt.Errorf("创建目录 %s 失败: %w", f.name, err)
			}
		} else if f.isLink {
			if err := os.Symlink(f.linkname, targetPath); err != nil {
				return fmt.Errorf("创建符号链接 %s -> %s 失败: %w", f.name, f.linkname, err)
			}
		} else {
			if err := os.WriteFile(targetPath, f.content, os.FileMode(f.mode)); err != nil {
				return fmt.Errorf("写入 %s 失败: %w", f.name, err)
			}
		}
	}

	// 生成清洁的 LXC 配置（避免 set_config_unsupported_key）
	configPath := filepath.Join(lxcDir, "config")
	cleanConfig := generateCleanConfig(name, meta.Autostart)
	if err := os.WriteFile(configPath, []byte(cleanConfig), 0644); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}

	// 应用端口映射
	if len(meta.PortForwards) > 0 {
		EnsurePortForwardService()
		netSvc := NewNetworkService(&core.ShellExecutor{})
		for _, pf := range meta.PortForwards {
			if err := netSvc.AddPortForward(name, pf.ContainerPort, pf.HostPort); err != nil {
				fmt.Printf("  警告: 端口映射 %d->%d 应用失败: %v\n", pf.HostPort, pf.ContainerPort, err)
			}
		}
		exec.Command("systemctl", "enable", "sb-lxc-port@"+name+".service").Run()
	}

	// 应用域名映射
	for _, domain := range meta.DomainMappings {
		applyDomainMapping(name, domain)
	}

	fmt.Printf("已导入容器 %s\n", name)
	fmt.Printf("  端口映射: %d 条\n", len(meta.PortForwards))
	fmt.Printf("  域名映射: %d 条\n", len(meta.DomainMappings))
	fmt.Printf("  开机自启: %v\n", meta.Autostart)
	fmt.Printf("使用 sb_lxc start %s 启动容器\n", name)
	return nil
}

// generateCleanConfig 生成不含不支持键的清洁 LXC 配置。
// IP 地址根据容器名哈希生成，无需容器内运行 DHCP 客户端。
func generateCleanConfig(name string, autostart bool) string {
	autostartVal := "0"
	if autostart {
		autostartVal = "1"
	}

	// 根据容器名生成确定性 IP 尾号 (2-253)，避免每次手动配 IP
	h := fnv.New32a()
	h.Write([]byte(name))
	ipSuffix := int(h.Sum32()%252) + 2

	return fmt.Sprintf(`# Distribution configuration
lxc.include = /usr/share/lxc/config/common.conf

# Container specific configuration
lxc.rootfs.path = dir:/var/lib/lxc/%s/rootfs
lxc.uts.name = %s

# /dev setup — 自动创建 /dev/null, /dev/zero 等设备
lxc.autodev = 1

# Mount /proc, /sys
lxc.mount.auto = proc:rw sys:rw

# Network configuration — 宿主机侧直接分配 IPv4，不依赖容器内 DHCP
lxc.net.0.type = veth
lxc.net.0.link = lxcbr0
lxc.net.0.flags = up
lxc.net.0.ipv4.address = 10.0.3.%d/24
lxc.net.0.ipv4.gateway = 10.0.3.1

# Autostart
lxc.start.auto = %s
`, name, name, ipSuffix, autostartVal)
}

// applyDomainMapping 为容器创建域名映射（从导出数据恢复）
func applyDomainMapping(name, domain string) {
	hookDir := filepath.Join("/var/lib/lxc", name)
	hookPath := filepath.Join(hookDir, "domain-hosts.sh")

	// 生成脚本
	scriptContent := fmt.Sprintf(`#!/bin/sh
NAME="%s"
DOMAIN="%s"
for i in 1 2 3; do
  IP=$(lxc-info -n "$NAME" -iH 2>/dev/null | head -1 | tr -d ' \n\t')
  if [ -n "$IP" ] && [ "$IP" != " " ]; then
    break
  fi
  sleep 1
done
if [ -n "$IP" ]; then
  sed -i "/[[:space:]]%s$/d" /etc/hosts
  echo "$IP  %s" >> /etc/hosts
fi
`, name, domain, domain, domain)

	os.WriteFile(hookPath, []byte(scriptContent), 0755)

	// 确保并启用 systemd 服务
	ensureDomainService()
	exec.Command("systemctl", "enable", "sb-lxc-domain@"+name+".service").Run()
	fmt.Printf("  域名映射已恢复: %s -> %s\n", domain, name)
}

func ensureDomainService() {
	svcPath := "/etc/systemd/system/sb-lxc-domain@.service"
	if _, err := os.Stat(svcPath); err == nil {
		return
	}
	content := `[Unit]
Description=Domain hosts update for LXC container %i
After=lxc.service
Wants=lxc.service

[Service]
Type=oneshot
ExecStart=/var/lib/lxc/%i/domain-hosts.sh

[Install]
WantedBy=multi-user.target`
	os.WriteFile(svcPath, []byte(content), 0644)
	exec.Command("systemctl", "daemon-reload").Run()
}

func writeTarEntry(tw *tar.Writer, name string, content []byte) error {
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}
