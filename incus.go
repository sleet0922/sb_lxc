package main

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	incus "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"golang.org/x/term"
)

// IncusClient is a thin application-specific wrapper around the official
// Incus REST client. It connects to the local Unix socket by default.
type IncusClient struct {
	server    incus.InstanceServer
	initErr   error
	imageServ incus.ImageServer
	imageErr  error
}

// NewIncusClient connects to the local daemon unless SB_LXC_INCUS_URL is set.
// The URL form is useful when the tool itself is not running on the daemon.
func NewIncusClient() *IncusClient {
	c := &IncusClient{}
	args := connectionArgsFromEnv()
	if uri := strings.TrimSpace(os.Getenv("SB_LXC_INCUS_URL")); uri != "" {
		c.server, c.initErr = incus.ConnectIncus(uri, args)
	} else {
		c.server, c.initErr = incus.ConnectIncusUnix("", args)
	}
	return c
}

func connectionArgsFromEnv() *incus.ConnectionArgs {
	args := &incus.ConnectionArgs{SkipGetEvents: true}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SB_LXC_INCUS_INSECURE")), "true") {
		args.InsecureSkipVerify = true
	}
	args.TLSServerCert = readOptionalFileEnv("SB_LXC_INCUS_SERVER_CERT")
	args.TLSClientCert = readOptionalFileEnv("SB_LXC_INCUS_CLIENT_CERT")
	args.TLSClientKey = readOptionalFileEnv("SB_LXC_INCUS_CLIENT_KEY")
	args.TLSCA = readOptionalFileEnv("SB_LXC_INCUS_CA")
	return args
}

func readOptionalFileEnv(name string) string {
	path := strings.TrimSpace(os.Getenv(name))
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func (c *IncusClient) ready() error {
	if c == nil {
		return fmt.Errorf("Incus 客户端为空")
	}
	if c.initErr != nil {
		return fmt.Errorf("连接 Incus REST API 失败: %w", c.initErr)
	}
	return nil
}

func (c *IncusClient) imageServer() (incus.ImageServer, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	if c.imageServ != nil || c.imageErr != nil {
		return c.imageServ, c.imageErr
	}
	c.imageServ, c.imageErr = incus.ConnectSimpleStreams(MirrorURL, nil)
	if c.imageErr != nil {
		return nil, fmt.Errorf("连接镜像 SimpleStreams 服务失败: %w", c.imageErr)
	}
	return c.imageServ, nil
}

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

const (
	defaultNICName       = "eth0"
	defaultMacvlanParent = "ens18"
	macvlanParentEnv     = "SB_LXC_MACVLAN_PARENT"
)

func detectMacvlanParent() (string, error) {
	if parent := strings.TrimSpace(os.Getenv(macvlanParentEnv)); parent != "" {
		if !linkExists(parent) {
			return "", fmt.Errorf("%s=%q 指定的网卡不存在或不可用", macvlanParentEnv, parent)
		}
		return parent, nil
	}
	if parent, err := defaultIPv4RouteParent(); err == nil && parent != "" {
		if !linkExists(parent) {
			return "", fmt.Errorf("默认出口网卡 %q 不存在或不可用", parent)
		}
		return parent, nil
	}
	if linkExists(defaultMacvlanParent) {
		return defaultMacvlanParent, nil
	}
	return "", fmt.Errorf("无法自动识别默认出口网卡，请设置 %s=网卡名", macvlanParentEnv)
}

func defaultIPv4RouteParent() (string, error) {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err == nil {
		if dev := firstRouteDev(string(out)); dev != "" {
			return dev, nil
		}
	}
	showErr := err
	out, err = exec.Command("ip", "-4", "route", "get", "1.1.1.1").Output()
	if err == nil {
		if dev := firstRouteDev(string(out)); dev != "" {
			return dev, nil
		}
	}
	if showErr != nil {
		return "", fmt.Errorf("读取默认 IPv4 路由失败: %w", showErr)
	}
	if err != nil {
		return "", fmt.Errorf("探测默认 IPv4 出口失败: %w", err)
	}
	return "", fmt.Errorf("默认 IPv4 路由中没有 dev 字段")
}

func firstRouteDev(output string) string {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field != "dev" || i+1 >= len(fields) {
				continue
			}
			dev := strings.TrimSpace(fields[i+1])
			if dev != "" && dev != "lo" && dev != defaultHostMacvlanName {
				return dev
			}
		}
	}
	return ""
}

func linkExists(name string) bool {
	return exec.Command("ip", "link", "show", name).Run() == nil
}

// Container is the subset of api.InstanceFull used by the application.
type Container struct {
	Name            string
	Status          string
	StatusCode      int
	Type            string
	Config          map[string]string
	Devices         map[string]map[string]string
	ExpandedDevices map[string]map[string]string
	State           *ContainerState
}

type ContainerState struct {
	Network map[string]NICState
	Pid     int64
}

type NICState struct {
	Addresses []NICAddr
	HwAddr    string
	State     string
	Type      string
}

type NICAddr struct {
	Family  string
	Address string
	Scope   string
}

func convertContainer(full *api.InstanceFull) *Container {
	if full == nil {
		return nil
	}
	c := &Container{
		Name:            full.Name,
		Status:          full.Status,
		StatusCode:      int(full.StatusCode),
		Type:            full.Type,
		Config:          cloneConfig(full.Config),
		Devices:         cloneDevices(full.Devices),
		ExpandedDevices: cloneDevices(full.ExpandedDevices),
	}
	if full.State == nil {
		return c
	}
	c.State = &ContainerState{Network: make(map[string]NICState), Pid: full.State.Pid}
	for name, nic := range full.State.Network {
		state := NICState{HwAddr: nic.Hwaddr, State: nic.State, Type: nic.Type}
		for _, addr := range nic.Addresses {
			state.Addresses = append(state.Addresses, NICAddr{Family: addr.Family, Address: addr.Address, Scope: addr.Scope})
		}
		c.State.Network[name] = state
	}
	return c
}

func cloneConfig(in api.ConfigMap) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneDevices(in api.DevicesMap) map[string]map[string]string {
	out := make(map[string]map[string]string, len(in))
	for name, device := range in {
		copyDevice := make(map[string]string, len(device))
		for k, v := range device {
			copyDevice[k] = v
		}
		out[name] = copyDevice
	}
	return out
}

func apiConfig(in map[string]string) api.ConfigMap {
	out := api.ConfigMap{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func apiDevices(in map[string]map[string]string) api.DevicesMap {
	out := api.DevicesMap{}
	for name, device := range in {
		copyDevice := map[string]string{}
		for k, v := range device {
			copyDevice[k] = v
		}
		out[name] = copyDevice
	}
	return out
}

func writableInstance(full *api.InstanceFull) api.InstancePut {
	p := full.Writable()
	p.Config = apiConfig(cloneConfig(full.Config))
	p.Devices = apiDevices(cloneDevices(full.Devices))
	p.Profiles = append([]string(nil), full.Profiles...)
	return p
}

func (c *IncusClient) updateInstance(name string, etag string, put api.InstancePut) error {
	op, err := c.server.UpdateInstance(name, put, etag)
	if err != nil {
		return err
	}
	return op.Wait()
}

// EnsureMirrorRemote is retained for compatibility. REST image requests carry
// the SimpleStreams server directly, so no local Incus remote is needed.
func (c *IncusClient) EnsureMirrorRemote() {}

// AutoCleanupUnusedBridges 自动删除未被任何容器或 profile 引用的 Incus 托管 bridge 网络。
// sb_lxc 使用 macvlan 网络，Incus admin init 默认创建的 incusbr0 在此场景下无用，
// 还会占用 53 端口(dnsmasq)与网段。删除是幂等的：已不存在则跳过。
// 不删除非托管(external) bridge，避免误删用户手动创建的网桥。
// 安全约束：容器/profile 列表查询失败时直接返回错误，绝不执行删除，避免误删在用网桥。
func (c *IncusClient) AutoCleanupUnusedBridges() error {
	if err := c.ready(); err != nil {
		return err
	}
	networks, err := c.server.GetNetworks()
	if err != nil {
		return fmt.Errorf("获取网络列表失败: %w", err)
	}

	// 收集所有容器与 profile 中 NIC 设备引用的 network / parent。
	// 任一查询失败都必须中止：否则 used 为空会误删在用网桥。
	used := map[string]bool{}
	instances, err := c.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return fmt.Errorf("查询容器列表失败，跳过网桥清理以避免误删: %w", err)
	}
	for i := range instances {
		for _, devs := range []map[string]map[string]string{instances[i].Devices, instances[i].ExpandedDevices} {
			for _, dev := range devs {
				if dev["type"] != "nic" {
					continue
				}
				if n := dev["network"]; n != "" {
					used[n] = true
				}
				if p := dev["parent"]; p != "" {
					used[p] = true
				}
			}
		}
	}
	profiles, err := c.server.GetProfiles()
	if err != nil {
		return fmt.Errorf("查询 profile 列表失败，跳过网桥清理以避免误删: %w", err)
	}
	for _, p := range profiles {
		for _, dev := range p.Devices {
			if dev["type"] != "nic" {
				continue
			}
			if n := dev["network"]; n != "" {
				used[n] = true
			}
			if p := dev["parent"]; p != "" {
				used[p] = true
			}
		}
	}

	// 删除所有 Incus 托管且未被引用的 bridge 类型网络
	for _, n := range networks {
		if n.Type != "bridge" || !n.Managed {
			continue
		}
		if used[n.Name] {
			continue
		}
		if err := c.server.DeleteNetwork(n.Name); err != nil {
			fmt.Fprintf(os.Stderr, "⚠ 删除未使用网桥 %s 失败: %v\n", n.Name, err)
			continue
		}
		fmt.Printf("✔ 已删除未使用的 Incus 网桥: %s\n", n.Name)
	}
	return nil
}

func (c *IncusClient) EnsureDefaultMacvlanProfile() error {
	if err := c.ready(); err != nil {
		return err
	}
	parent, err := detectMacvlanParent()
	if err != nil {
		return err
	}
	profile, etag, err := c.server.GetProfile("default")
	if err != nil {
		return fmt.Errorf("获取 default profile 失败: %w", err)
	}
	devices := cloneDevices(profile.Devices)
	eth0 := devices[defaultNICName]
	if eth0 != nil && eth0["type"] == "nic" && eth0["nictype"] == "macvlan" && eth0["parent"] == parent {
		return nil
	}
	devices[defaultNICName] = map[string]string{"type": "nic", "nictype": "macvlan", "parent": parent, "name": defaultNICName}
	put := api.ProfilePut{Config: profile.Config, Description: profile.Description, Devices: apiDevices(devices)}
	if err := c.server.UpdateProfile("default", put, etag); err != nil {
		return fmt.Errorf("配置 default profile 的 %s macvlan 失败: %w", defaultNICName, err)
	}
	return nil
}

func (c *IncusClient) Start(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	if err := c.configureMacvlanNIC(name, false); err != nil {
		return fmt.Errorf("配置 %s macvlan 网络失败: %w", defaultNICName, err)
	}
	op, err := c.server.UpdateInstanceState(name, api.InstanceStatePut{Action: "start", Timeout: -1}, "")
	if err != nil {
		return err
	}
	return op.Wait()
}

func (c *IncusClient) configureMacvlanNIC(name string, forceNewMAC bool) error {
	if err := c.ready(); err != nil {
		return err
	}
	full, etag, err := c.server.GetInstanceFull(name)
	if err != nil {
		return err
	}
	parent, err := detectMacvlanParent()
	if err != nil {
		return err
	}
	mac := ""
	if !forceNewMAC {
		mac = convertContainer(full).NICMAC(defaultNICName)
	}
	if mac == "" {
		mac, err = randomMAC()
		if err != nil {
			return err
		}
	}
	put := writableInstance(full)
	devices := cloneDevices(full.Devices)
	if devices == nil {
		devices = map[string]map[string]string{}
	}
	devices[defaultNICName] = map[string]string{
		"type": "nic", "name": defaultNICName, "nictype": "macvlan", "parent": parent, "hwaddr": mac,
	}
	put.Devices = apiDevices(devices)
	if err := c.updateInstance(name, etag, put); err != nil {
		return err
	}
	return nil
}

func randomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[0] = (buf[0] | 0x02) & 0xfe
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5]), nil
}

func (c *IncusClient) Stop(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	op, err := c.server.UpdateInstanceState(name, api.InstanceStatePut{Action: "stop", Timeout: 30}, "")
	if err != nil {
		return err
	}
	return op.Wait()
}

func (c *IncusClient) Delete(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	op, err := c.server.DeleteInstance(name)
	if err != nil {
		return err
	}
	return op.Wait()
}

func defaultExecEnv() map[string]string {
	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}
	return map[string]string{
		"HOME": "/root",
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"TERM": term,
		"USER": "root",
	}
}

func (c *IncusClient) Exec(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	shell := "/bin/sh"
	if out, _ := c.execQuiet(name, "/bin/sh", "-c", "if test -x /bin/bash; then echo yes; fi"); out == "yes" {
		shell = "/bin/bash"
	}
	fd := int(os.Stdin.Fd())
	if old, err := makeRaw(fd); err == nil {
		defer restoreTerm(fd, old)
	}
	width, height := 120, 40
	if w, h, err := term.GetSize(fd); err == nil && w > 0 && h > 0 {
		width, height = w, h
	}
	req := api.InstanceExecPost{
		Command:     []string{shell},
		Environment: defaultExecEnv(),
		WaitForWS:   true,
		Interactive: true,
		Width:       width,
		Height:      height,
	}
	op, err := c.server.ExecInstance(name, req, &incus.InstanceExecArgs{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	if err != nil {
		return fmt.Errorf("无法进入容器 %s 的 shell: %w", name, err)
	}
	if err := op.Wait(); err != nil {
		return fmt.Errorf("进入容器 %s 的 shell 断开或执行失败: %w", name, err)
	}
	return nil
}

func (c *IncusClient) execQuiet(name string, args ...string) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	req := api.InstanceExecPost{
		Command:     args,
		Environment: defaultExecEnv(),
		WaitForWS:   true,
		Interactive: false,
	}
	var out bytes.Buffer
	op, err := c.server.ExecInstance(name, req, &incus.InstanceExecArgs{
		Stdin: strings.NewReader(""), Stdout: &out, Stderr: io.Discard,
	})
	if err != nil {
		return "", err
	}
	err = op.Wait()
	return strings.TrimSpace(out.String()), err
}

func (c *IncusClient) Launch(imageRef, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	// 规范化镜像引用：debian:12 -> debian/12，与镜像源 alias 一致
	alias := normalizeImageRef(imageRef)
	// 去掉可能的 remote 前缀（如 mirror-images:debian/12 -> debian/12）
	if idx := strings.IndexByte(alias, ':'); idx >= 0 {
		alias = alias[idx+1:]
	}
	req := api.InstancesPost{
		Name: name,
		Type: api.InstanceTypeContainer,
		InstancePut: api.InstancePut{
			Config: map[string]string{
				"security.privileged": "true", // 默认高权限：便于 systemd/网络/设备访问
			},
		},
		Source: api.InstanceSource{
			Type: "image", Alias: alias, Server: MirrorURL, Protocol: "simplestreams",
		},
		Start: false,
	}
	op, err := c.server.CreateInstance(req)
	if err != nil {
		return err
	}
	if err := op.Wait(); err != nil {
		return err
	}
	return c.Start(name)
}

func (c *IncusClient) ListContainers() ([]Container, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	instances, err := c.server.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return nil, fmt.Errorf("获取容器列表失败: %w", err)
	}
	result := make([]Container, 0, len(instances))
	for i := range instances {
		result = append(result, *convertContainer(&instances[i]))
	}
	return result, nil
}

func (c *IncusClient) GetContainer(name string) (*Container, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	instance, _, err := c.server.GetInstanceFull(name)
	if err != nil {
		return nil, fmt.Errorf("获取容器 %q 失败: %w", name, err)
	}
	return convertContainer(instance), nil
}

func (c *IncusClient) SetBootAutostart(name string, on bool) error {
	if err := c.ready(); err != nil {
		return err
	}
	full, etag, err := c.server.GetInstanceFull(name)
	if err != nil {
		return err
	}
	put := writableInstance(full)
	if put.Config == nil {
		put.Config = api.ConfigMap{}
	}
	put.Config["boot.autostart"] = strconv.FormatBool(on)
	return c.updateInstance(name, etag, put)
}

func (c *IncusClient) SetDomain(name, domain string) error {
	if err := c.ready(); err != nil {
		return err
	}
	full, etag, err := c.server.GetInstanceFull(name)
	if err != nil {
		return err
	}
	put := writableInstance(full)
	if put.Config == nil {
		put.Config = api.ConfigMap{}
	}
	put.Config["user.sb_lxc.domain"] = domain
	return c.updateInstance(name, etag, put)
}

func (c *IncusClient) UnsetDomain(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	full, etag, err := c.server.GetInstanceFull(name)
	if err != nil {
		return err
	}
	put := writableInstance(full)
	delete(put.Config, "user.sb_lxc.domain")
	return c.updateInstance(name, etag, put)
}

func (c *IncusClient) Export(name, path string) error {
	if err := c.ready(); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	backup := api.InstanceBackupsPost{CompressionAlgorithm: "gzip"}
	err = c.server.CreateInstanceBackupStream(name, backup, &incus.BackupFileRequest{BackupFile: f})
	if err == nil {
		return nil
	}
	// Older servers may not expose direct_backup. Fall back to a temporary
	// persisted backup and download it through the standard REST endpoint.
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr == nil {
		_ = f.Truncate(0)
	}
	backup.Name = fmt.Sprintf("sb-lxc-export-%d", time.Now().UnixNano())
	op, createErr := c.server.CreateInstanceBackup(name, backup)
	if createErr != nil {
		return fmt.Errorf("导出备份失败: %w (direct backup: %v)", createErr, err)
	}
	if createErr = op.Wait(); createErr != nil {
		return createErr
	}
	defer func() {
		if deleteOp, e := c.server.DeleteInstanceBackup(name, backup.Name); e == nil {
			_ = deleteOp.Wait()
		}
	}()
	_, downloadErr := c.server.GetInstanceBackupFile(name, backup.Name, &incus.BackupFileRequest{BackupFile: f})
	return downloadErr
}

func (c *IncusClient) Import(path, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	op, err := c.server.CreateInstanceFromBackup(incus.InstanceBackupArgs{BackupFile: f, Name: name})
	if err != nil {
		return err
	}
	if err := op.Wait(); err != nil {
		return err
	}
	// 导入的容器也强制高权限，保持策略一致
	_ = c.EnsurePrivileged(name)
	return nil
}

// EnsurePrivileged 确保容器以高权限运行 (security.privileged=true)。
// 已是高权限则跳过。用于导入/迁移场景保持策略一致。
func (c *IncusClient) EnsurePrivileged(name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	full, etag, err := c.server.GetInstanceFull(name)
	if err != nil {
		return err
	}
	if full.Config["security.privileged"] == "true" {
		return nil
	}
	put := writableInstance(full)
	if put.Config == nil {
		put.Config = api.ConfigMap{}
	}
	put.Config["security.privileged"] = "true"
	return c.updateInstance(name, etag, put)
}

func (c *IncusClient) ConfigureImportedNetwork(name string) error {
	return c.configureMacvlanNIC(name, true)
}

func (ct *Container) NICMAC(nic string) string {
	for _, devs := range []map[string]map[string]string{ct.Devices, ct.ExpandedDevices} {
		if dev := devs[nic]; dev != nil && strings.TrimSpace(dev["hwaddr"]) != "" {
			return strings.TrimSpace(dev["hwaddr"])
		}
	}
	if ct.Config != nil {
		if mac := strings.TrimSpace(ct.Config["volatile."+nic+".hwaddr"]); mac != "" {
			return mac
		}
	}
	if ct.State != nil {
		if state, ok := ct.State.Network[nic]; ok {
			return strings.TrimSpace(state.HwAddr)
		}
	}
	return ""
}

func (ct *Container) UsesMacvlanNIC(nic string) bool {
	dev := ct.ExpandedDevices[nic]
	if dev == nil {
		dev = ct.Devices[nic]
	}
	return dev != nil && dev["type"] == "nic" && dev["nictype"] == "macvlan"
}

func (ct *Container) IPv4() string {
	if ct.State == nil {
		return ""
	}
	for name, nic := range ct.State.Network {
		if name == "lo" || nic.Type == "loopback" {
			continue
		}
		for _, addr := range nic.Addresses {
			if addr.Family == "inet" && addr.Scope == "global" {
				return addr.Address
			}
		}
	}
	return ""
}

func (ct *Container) Autostart() string { return ct.Config["boot.autostart"] }
func (ct *Container) Domain() string    { return ct.Config["user.sb_lxc.domain"] }

type Image struct {
	Architecture string
	Type         string
	Aliases      []ImageAlias
	Properties   map[string]string
	Size         int64
}

type ImageAlias struct{ Name string }

type ImageVersion struct {
	Release string
	Image   string
}

type DistroGroup struct {
	Distro   string
	Versions []ImageVersion
}

func (c *IncusClient) ListImages() ([]DistroGroup, error) {
	remote, err := c.imageServer()
	if err != nil {
		return nil, err
	}
	images, err := remote.GetImages()
	if err != nil {
		return nil, fmt.Errorf("获取镜像列表失败: %w", err)
	}
	arch := archName() // 动态获取当前主机架构，支持 amd64/arm64
	allowedDistros := map[string]bool{"alpine": true, "centos": true, "debian": true, "nixos": true, "ubuntu": true, "oracle": true, "rockylinux": true}
	grouped := map[string]map[string]string{}
	distroOrder := []string{}
	for _, img := range images {
		if img.Type != "container" || img.Architecture != arch || img.Properties["variant"] == "cloud" {
			continue
		}
		osName, release := img.Properties["os"], img.Properties["release"]
		if osName == "" || release == "" {
			continue
		}
		osKey, relKey := strings.ToLower(osName), strings.ToLower(release)
		if !allowedDistros[osKey] {
			continue
		}
		shortest := ""
		for _, alias := range img.Aliases {
			if shortest == "" || len(alias.Name) < len(shortest) {
				shortest = alias.Name
			}
		}
		if shortest == "" {
			continue
		}
		if grouped[osKey] == nil {
			grouped[osKey] = map[string]string{}
			distroOrder = append(distroOrder, osKey)
		}
		if current := grouped[osKey][relKey]; current == "" || len(shortest) < len(current) {
			grouped[osKey][relKey] = shortest
		}
	}
	sort.Strings(distroOrder)
	result := make([]DistroGroup, 0, len(distroOrder))
	for _, osKey := range distroOrder {
		rels := grouped[osKey]
		relKeys := make([]string, 0, len(rels))
		for release := range rels {
			relKeys = append(relKeys, release)
		}
		sort.Strings(relKeys)
		versions := make([]ImageVersion, 0, len(relKeys))
		for _, release := range relKeys {
			versions = append(versions, ImageVersion{Release: release, Image: strings.TrimSuffix(rels[release], "/default")})
		}
		result = append(result, DistroGroup{Distro: titleCase(osKey), Versions: versions})
	}
	return result, nil
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// LaunchLocalImage 从本地镜像存储启动容器 (不从远程镜像服务器拉取)。
// 用于 sb_lxc build/run 启动已构建的本地镜像。
func (c *IncusClient) LaunchLocalImage(alias, name string) error {
	if err := c.ready(); err != nil {
		return err
	}
	req := api.InstancesPost{
		Name: name,
		Type: api.InstanceTypeContainer,
		InstancePut: api.InstancePut{
			Config: map[string]string{
				"security.privileged": "true", // 默认高权限：便于 systemd/网络/设备访问
			},
		},
		Source: api.InstanceSource{
			Type:  "image",
			Alias: alias,
		},
		Start: false,
	}
	op, err := c.server.CreateInstance(req)
	if err != nil {
		return err
	}
	if err := op.Wait(); err != nil {
		return err
	}
	return c.Start(name)
}

// PushFile 将字节数据写入容器的指定路径 (覆盖模式)。
// mode 为八进制权限字符串如 "0644"，为空时默认 0644。UID/GID 保持不变。
func (c *IncusClient) PushFile(name, path string, content []byte, mode string) error {
	if err := c.ready(); err != nil {
		return err
	}
	if mode == "" {
		mode = "0644"
	}
	modeInt, err := strconv.ParseInt(mode, 8, 64)
	if err != nil {
		return fmt.Errorf("权限 %q 不是合法的八进制数: %w", mode, err)
	}
	args := incus.InstanceFileArgs{
		Content:   bytes.NewReader(content),
		UID:       -1,
		GID:       -1,
		Mode:      int(modeInt),
		Type:      "file",
		WriteMode: "overwrite",
	}
	return c.server.CreateInstanceFile(name, path, args)
}

// ReadFile 读取容器内指定路径的文件内容。
func (c *IncusClient) ReadFile(name, path string) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	reader, _, err := c.server.GetInstanceFile(name, path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExecStreaming 在容器内通过 /bin/sh -c 执行命令，stdout/stderr 实时输出到当前进程。
// extraEnv 会与默认环境合并，使 Incusfile 的 ENV 指令对后续 RUN 生效。
func (c *IncusClient) ExecStreaming(name, command string, extraEnv map[string]string) error {
	if err := c.ready(); err != nil {
		return err
	}
	env := defaultExecEnv()
	for k, v := range extraEnv {
		env[k] = v
	}
	req := api.InstanceExecPost{
		Command:     []string{"/bin/sh", "-c", command},
		Environment: env,
		WaitForWS:   true,
		Interactive: false,
	}
	op, err := c.server.ExecInstance(name, req, &incus.InstanceExecArgs{
		Stdin:  strings.NewReader(""),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	return op.Wait()
}

// PublishImage 将容器发布为本地 Incus 镜像，并设置别名。
// properties 会作为镜像属性存储，供 sb_lxc run 读取以恢复 EXPOSE/DOMAIN/AUTOSTART。
func (c *IncusClient) PublishImage(containerName, alias string, properties map[string]string) error {
	if err := c.ready(); err != nil {
		return err
	}
	req := api.ImagesPost{
		Source: &api.ImagesPostSource{
			Type: "container",
			Name: containerName,
		},
	}
	if properties != nil {
		req.Properties = properties
	}
	op, err := c.server.CreateImage(req, nil)
	if err != nil {
		return err
	}
	if err := op.Wait(); err != nil {
		return err
	}
	// 从操作元数据获取 fingerprint
	fingerprint := ""
	opAPI := op.Get()
	if opAPI.Metadata != nil {
		if fp, ok := opAPI.Metadata["fingerprint"].(string); ok {
			fingerprint = fp
		}
	}
	if fingerprint == "" {
		// 回退：列出所有镜像，取最新创建的
		images, err := c.server.GetImages()
		if err != nil {
			return fmt.Errorf("发布成功但无法获取镜像 fingerprint: %w", err)
		}
		if len(images) == 0 {
			return fmt.Errorf("发布成功但未找到镜像")
		}
		latest := images[0]
		for _, img := range images {
			if img.CreatedAt.After(latest.CreatedAt) {
				latest = img
			}
		}
		fingerprint = latest.Fingerprint
	}
	// 创建别名
	aliasReq := api.ImageAliasesPost{
		ImageAliasesEntry: api.ImageAliasesEntry{
			Name: alias,
			ImageAliasesEntryPut: api.ImageAliasesEntryPut{
				Target: fingerprint,
			},
		},
	}
	return c.server.CreateImageAlias(aliasReq)
}

// ReplaceImageAlias 删除已存在的镜像别名。若旧镜像无其他别名引用，则一并删除孤儿镜像。
// 用于重新构建同名镜像时保持幂等。别名不存在时静默返回 nil。
func (c *IncusClient) ReplaceImageAlias(alias string) error {
	if err := c.ready(); err != nil {
		return err
	}
	entry, _, err := c.server.GetImageAlias(alias)
	if err != nil {
		return nil // 别名不存在，无需处理
	}
	// 删除旧别名
	if err := c.server.DeleteImageAlias(alias); err != nil {
		return err
	}
	// 检查旧镜像是否还有其他别名引用
	aliases, err := c.server.GetImageAliases()
	if err != nil {
		return nil
	}
	for _, a := range aliases {
		if a.Target == entry.Target {
			return nil // 仍有其他别名引用，保留镜像
		}
	}
	// 无其他引用，删除孤儿镜像
	op, err := c.server.DeleteImage(entry.Target)
	if err != nil {
		return nil
	}
	_ = op.Wait()
	return nil
}

// GetImageProperties 读取本地镜像 (通过别名) 的属性。
func (c *IncusClient) GetImageProperties(alias string) (map[string]string, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	entry, _, err := c.server.GetImageAlias(alias)
	if err != nil {
		return nil, err
	}
	image, _, err := c.server.GetImage(entry.Target)
	if err != nil {
		return nil, err
	}
	return image.Properties, nil
}
