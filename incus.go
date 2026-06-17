package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
)

// archName 将 Go 架构名映射为 incus 镜像架构名。
func archName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

// IncusClient 封装对 incus 命令的调用与输出解析。
type IncusClient struct{}

// NewIncusClient 创建客户端实例。
func NewIncusClient() *IncusClient { return &IncusClient{} }

// ──────────────────── 数据结构（对应 incus list --format json） ────────────────────

// Container 对应 incus list --format json 的单个元素。
type Container struct {
	Name            string                       `json:"name"`
	Status          string                       `json:"status"`
	StatusCode      int                          `json:"status_code"`
	Type            string                       `json:"type"`
	Config          map[string]string            `json:"config"`
	Devices         map[string]map[string]string `json:"devices"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
	State           *ContainerState              `json:"state"`
}

// ContainerState 容器运行态。
type ContainerState struct {
	Network map[string]NICState `json:"network"`
	Pid     int64               `json:"pid"`
}

// NICState 网卡状态。
type NICState struct {
	Addresses []NICAddr `json:"addresses"`
	HwAddr    string    `json:"hwaddr"`
	State     string    `json:"state"`
	Type      string    `json:"type"`
}

// NICAddr 网卡地址。
type NICAddr struct {
	Family  string `json:"family"`
	Address string `json:"address"`
	Scope   string `json:"scope"`
}

// ──────────────────── 基础执行 ────────────────────

// run 执行 incus 子命令并接管 stdio（交互式透传）。
func (c *IncusClient) run(args ...string) error {
	cmd := exec.Command("incus", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// capture 执行 incus 子命令并返回 stdout（不接管 stdio）。
func (c *IncusClient) capture(args ...string) ([]byte, error) {
	cmd := exec.Command("incus", args...)
	return cmd.Output()
}

// EnsureMirrorRemote 添加清华镜像源，已存在则忽略。
func (c *IncusClient) EnsureMirrorRemote() {
	cmd := exec.Command("incus", "remote", "add", MirrorRemote, MirrorURL,
		"--protocol=simplestreams", "--public")
	_ = cmd.Run() // 已存在会报错，忽略
}

// ──────────────────── 生命周期 ────────────────────

func (c *IncusClient) Start(name string) error { return c.run("start", name) }
func (c *IncusClient) Stop(name string) error  { return c.run("stop", name) }

// Exec 进入容器，依次尝试 sh / bash。
func (c *IncusClient) Exec(name string) error {
	for _, sh := range []string{"/bin/sh", "/bin/bash"} {
		if err := c.run("exec", name, "--", sh); err == nil {
			return nil
		}
	}
	return fmt.Errorf("无法进入容器 %s", name)
}

// Launch 从镜像源启动新容器。
func (c *IncusClient) Launch(imageRef, name string) error {
	return c.run("launch", imageRef, name)
}

// ──────────────────── 查询 ────────────────────

// ListContainers 解析 incus list --format json。
func (c *IncusClient) ListContainers() ([]Container, error) {
	out, err := c.capture("list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("获取容器列表失败: %w", err)
	}
	var cs []Container
	if err := json.Unmarshal(out, &cs); err != nil {
		return nil, fmt.Errorf("解析容器列表失败: %w", err)
	}
	return cs, nil
}

// GetContainer 获取单个容器信息。
func (c *IncusClient) GetContainer(name string) (*Container, error) {
	cs, err := c.ListContainers()
	if err != nil {
		return nil, err
	}
	for i := range cs {
		if cs[i].Name == name {
			return &cs[i], nil
		}
	}
	return nil, fmt.Errorf("容器 %q 不存在", name)
}

// ──────────────────── 配置 ────────────────────

// SetBootAutostart 设置开机自启。
func (c *IncusClient) SetBootAutostart(name string, on bool) error {
	val := "false"
	if on {
		val = "true"
	}
	return c.run("config", "set", name, "boot.autostart="+val)
}

// SetDomain 设置域名映射（存于 user.sb_lxc.domain 配置项）。
func (c *IncusClient) SetDomain(name, domain string) error {
	return c.run("config", "set", name, "user.sb_lxc.domain="+domain)
}

// UnsetDomain 取消域名映射。
func (c *IncusClient) UnsetDomain(name string) error {
	return c.run("config", "unset", name, "user.sb_lxc.domain")
}

// AddProxyDevice 添加端口映射代理设备。
func (c *IncusClient) AddProxyDevice(name, device, listen, connect string) error {
	return c.run("config", "device", "add", name, device, "proxy",
		"listen="+listen, "connect="+connect)
}

// RemoveDevice 移除设备。
func (c *IncusClient) RemoveDevice(name, device string) error {
	return c.run("config", "device", "remove", name, device)
}

// Export 导出容器为 tar.gz 文件。
func (c *IncusClient) Export(name, path string) error {
	return c.run("export", name, path)
}

// Import 从 tar.gz 文件导入容器。
func (c *IncusClient) Import(path, name string) error {
	return c.run("import", path, name)
}

// ──────────────────── Container 便捷方法 ────────────────────

// ProxyDevices 从展开设备中提取所有 proxy 设备。
func (ct *Container) ProxyDevices() map[string]map[string]string {
	result := map[string]map[string]string{}
	devs := ct.ExpandedDevices
	if devs == nil {
		devs = ct.Devices
	}
	for k, v := range devs {
		if v["type"] == "proxy" {
			result[k] = v
		}
	}
	return result
}

// IPv4 返回首个全局 IPv4 地址。
func (ct *Container) IPv4() string {
	if ct.State == nil || ct.State.Network == nil {
		return ""
	}
	for name, nic := range ct.State.Network {
		if name == "lo" || nic.Type == "loopback" {
			continue
		}
		for _, a := range nic.Addresses {
			if a.Family == "inet" && a.Scope == "global" {
				return a.Address
			}
		}
	}
	return ""
}

// IPv6 返回首个全局 IPv6 地址。
func (ct *Container) IPv6() string {
	if ct.State == nil || ct.State.Network == nil {
		return ""
	}
	for name, nic := range ct.State.Network {
		if name == "lo" || nic.Type == "loopback" {
			continue
		}
		for _, a := range nic.Addresses {
			if a.Family == "inet6" && a.Scope == "global" {
				return a.Address
			}
		}
	}
	return ""
}

// Autostart 返回 boot.autostart 配置值（未设置则为空）。
func (ct *Container) Autostart() string {
	return ct.Config["boot.autostart"]
}

// Domain 返回域名映射配置值（未设置则为空）。
func (ct *Container) Domain() string {
	return ct.Config["user.sb_lxc.domain"]
}

// shortAddr 将 tcp:0.0.0.0:8080 简化为 8080。
func shortAddr(addr string) string {
	parts := strings.Split(addr, ":")
	if len(parts) == 3 {
		return parts[2]
	}
	return addr
}

// ──────────────────── 镜像查询 ────────────────────

// Image 对应 incus image list --format json 的单个元素。
type Image struct {
	Architecture string              `json:"architecture"`
	Type         string              `json:"type"`
	Aliases      []ImageAlias        `json:"aliases"`
	Properties   map[string]string  `json:"properties"`
	Size         int64               `json:"size"`
}

// ImageAlias 镜像别名。
type ImageAlias struct {
	Name string `json:"name"`
}

// ImageVersion 镜像的某个具体版本。
type ImageVersion struct {
	Release string // 版本号/代号，如 "bookworm"
	Image   string // 镜像引用，如 "debian/bookworm"
}

// DistroGroup 发行版分组：发行版名 → 该发行版下所有可选版本。
type DistroGroup struct {
	Distro   string        // 发行版名，如 "Debian"
	Versions []ImageVersion
}

// ListImages 从镜像源拉取 x86_64 容器镜像，按发行版分组、版本聚合返回。
// 排除 cloud 变体，按 os(一级) + release(二级) 去重，取最短 alias 作为引用。
func (c *IncusClient) ListImages() ([]DistroGroup, error) {
	out, err := c.capture("image", "list", MirrorRemote+":", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("获取镜像列表失败: %w", err)
	}
	var imgs []Image
	if err := json.Unmarshal(out, &imgs); err != nil {
		return nil, fmt.Errorf("解析镜像列表失败: %w", err)
	}

	const arch = "x86_64"
	// 只保留这些发行版
	allowedDistros := map[string]bool{
		"alpine":     true,
		"centos":     true,
		"debian":     true,
		"nixos":      true,
		"ubuntu":     true,
		"oracle":     true,
		"rockylinux": true,
	}
	// distro(lower) -> {release(lower) -> shortest alias}
	grouped := map[string]map[string]string{}
	distroOrder := []string{}

	for _, img := range imgs {
		if img.Type != "container" || img.Architecture != arch {
			continue
		}
		p := img.Properties
		if p["variant"] == "cloud" {
			continue
		}
		osName := p["os"]
		rel := p["release"]
		if osName == "" || rel == "" {
			continue
		}
		osKey := strings.ToLower(osName)
		if !allowedDistros[osKey] {
			continue
		}
		relKey := strings.ToLower(rel)

		// 取最短 alias
		shortest := ""
		for _, a := range img.Aliases {
			n := a.Name
			if shortest == "" || len(n) < len(shortest) {
				shortest = n
			}
		}
		if shortest == "" {
			continue
		}

		if _, ok := grouped[osKey]; !ok {
			grouped[osKey] = map[string]string{}
			distroOrder = append(distroOrder, osKey)
		}
		if cur, ok := grouped[osKey][relKey]; !ok || len(shortest) < len(cur) {
			grouped[osKey][relKey] = shortest
		}
	}

	sort.Strings(distroOrder)

	result := make([]DistroGroup, 0, len(distroOrder))
	for _, osKey := range distroOrder {
		rels := grouped[osKey]
		relKeys := make([]string, 0, len(rels))
		for k := range rels {
			relKeys = append(relKeys, k)
		}
		sort.Strings(relKeys)

		versions := make([]ImageVersion, 0, len(relKeys))
		for _, rk := range relKeys {
			ref := strings.TrimSuffix(rels[rk], "/default")
			versions = append(versions, ImageVersion{Release: rk, Image: ref})
		}
		result = append(result, DistroGroup{
			Distro:   titleCase(osKey),
			Versions: versions,
		})
	}
	return result, nil
}

// titleCase 首字母大写。
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
