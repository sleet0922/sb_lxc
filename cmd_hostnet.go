package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
)

const (
	defaultHostMacvlanName = "sb-lxc-mv"
	hostShimCIDREnv        = "SB_LXC_HOST_SHIM_CIDR"
)

// AutoConfigureHostMacvlan 自动配置宿主机侧 macvlan shim，让宿主机可以访问
// 当前已运行的 macvlan 容器。该函数是幂等的：重复调用只会刷新地址和路由。
//
// 背景：Linux macvlan 默认隔离 parent 物理网卡与其 macvlan 子接口，容器能访问
// 路由器/局域网其他机器，但宿主机与容器之间不会经由物理网卡“折返”。解决办法是在
// 宿主机上也创建一个 macvlan 子接口，并把容器 IP 路由到该子接口。
func AutoConfigureHostMacvlan(client *IncusClient) error {
	targets := runningContainerIPv4Routes(client)
	if len(targets) == 0 {
		return nil
	}
	return ensureHostMacvlanRoutes(targets)
}

// AutoConfigureHostMacvlanForIP 为单个容器 IPv4 自动补齐宿主机侧 macvlan 路由。
func AutoConfigureHostMacvlanForIP(ip string) error {
	target, err := normalizeRouteTarget(ip)
	if err != nil {
		return err
	}
	return ensureHostMacvlanRoutes([]string{target})
}

func warnAutoHostMacvlan(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 自动配置宿主机 macvlan 互通失败: %v\n", err)
	}
}

func ensureHostMacvlanRoutes(targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	if err := ensureCommand("ip"); err != nil {
		return err
	}

	parent := defaultMacvlanParent
	normalizedTargets := make([]string, 0, len(targets))
	reservedIPs := make([]net.IP, 0, len(targets))
	for _, target := range targets {
		normalized, err := normalizeRouteTarget(target)
		if err != nil {
			return fmt.Errorf("路由目标 %q 无效: %w", target, err)
		}
		normalizedTargets = append(normalizedTargets, normalized)
		if ip := routeTargetIPv4(normalized); ip != nil {
			reservedIPs = append(reservedIPs, ip)
		}
	}

	shimCIDR, err := autoHostShimCIDR(parent, reservedIPs)
	if err != nil {
		return err
	}
	shimIP, _, err := net.ParseCIDR(shimCIDR)
	if err != nil || shimIP == nil || shimIP.To4() == nil {
		return fmt.Errorf("shim 地址 %q 无效", shimCIDR)
	}
	shimIP = shimIP.To4()

	if err := ensureHostMacvlan(parent, defaultHostMacvlanName); err != nil {
		return err
	}
	if err := configureHostMacvlanIsolation(parent, defaultHostMacvlanName); err != nil {
		return err
	}
	if err := replaceAddr(defaultHostMacvlanName, shimCIDR); err != nil {
		return err
	}
	if err := linkUp(defaultHostMacvlanName); err != nil {
		return err
	}

	for _, target := range normalizedTargets {
		if err := replaceRoute(target, defaultHostMacvlanName, shimIP.String()); err != nil {
			return err
		}
	}
	return nil
}

func autoHostShimCIDR(parent string, reserved []net.IP) (string, error) {
	if cidr := strings.TrimSpace(os.Getenv(hostShimCIDREnv)); cidr != "" {
		if err := validateHostShimCIDR(cidr); err != nil {
			return "", fmt.Errorf("%s=%q 无效: %w", hostShimCIDREnv, cidr, err)
		}
		if ipConflictsWithReservedCIDR(cidr, reserved) {
			return "", fmt.Errorf("%s=%q 与容器 IP 冲突", hostShimCIDREnv, cidr)
		}
		return cidr, nil
	}

	hostIP, ipNet, err := firstGlobalIPv4CIDR(parent)
	if err != nil {
		return "", err
	}

	gateway := defaultGatewayFor(parent)
	candidate, err := pickHostShimIP(parent, hostIP, gateway, ipNet, reserved)
	if err != nil {
		return "", err
	}
	return candidate.String() + "/32", nil
}

func firstGlobalIPv4CIDR(parent string) (net.IP, *net.IPNet, error) {
	out, err := exec.Command("ip", "-4", "-o", "addr", "show", "dev", parent, "scope", "global").Output()
	if err != nil {
		return nil, nil, fmt.Errorf("读取父网卡 %s IPv4 地址失败: %w", parent, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field != "inet" || i+1 >= len(fields) {
				continue
			}
			ip, ipNet, err := net.ParseCIDR(fields[i+1])
			if err != nil || ip == nil || ip.To4() == nil {
				continue
			}
			return ip.To4(), ipNet, nil
		}
	}
	return nil, nil, fmt.Errorf("父网卡 %s 没有可用的全局 IPv4 地址", parent)
}

func defaultGatewayFor(parent string) net.IP {
	out, err := exec.Command("ip", "-4", "route", "show", "default", "dev", parent).Output()
	if err != nil {
		return nil
	}
	fields := strings.Fields(string(out))
	for i, field := range fields {
		if field == "via" && i+1 < len(fields) {
			if ip := net.ParseIP(fields[i+1]); ip != nil {
				return ip.To4()
			}
		}
	}
	return nil
}

func pickHostShimIP(parent string, hostIP, gateway net.IP, ipNet *net.IPNet, reserved []net.IP) (net.IP, error) {
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones < 24 {
		return nil, fmt.Errorf("无法自动为 %s/%d 安全选择 shim IP；请设置 %s，例如 %s=192.168.3.254/32", ipNet.IP, ones, hostShimCIDREnv, hostShimCIDREnv)
	}

	network := ipv4ToUint32(ipNet.IP)
	mask := ipv4ToUint32(net.IP(ipNet.Mask))
	broadcast := network | ^mask

	host := ipv4ToUint32(hostIP)
	gw := uint32(0)
	if gateway != nil {
		gw = ipv4ToUint32(gateway)
	}

	// 从网段末尾向前选择，常见家庭/办公网络一般网关在 .1，宿主机在中间地址。
	for n := broadcast - 1; n > network; n-- {
		if n == host || (gw != 0 && n == gw) || containsIPv4Uint32(reserved, n) {
			continue
		}
		ip := uint32ToIPv4(n)
		if ipv4ProbablyUsed(parent, ip) {
			continue
		}
		return ip, nil
	}

	return nil, fmt.Errorf("无法在 %s 中自动选择未占用的 shim IP；请设置 %s", ipNet.String(), hostShimCIDREnv)
}

func routeTargetIPv4(target string) net.IP {
	ip, ipNet, err := net.ParseCIDR(target)
	if err != nil || ip == nil || ip.To4() == nil {
		return nil
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones != 32 {
		return nil
	}
	return ip.To4()
}

func containsIPv4Uint32(ips []net.IP, n uint32) bool {
	for _, ip := range ips {
		if ip == nil || ip.To4() == nil {
			continue
		}
		if ipv4ToUint32(ip) == n {
			return true
		}
	}
	return false
}

func ipConflictsWithReservedCIDR(cidr string, reserved []net.IP) bool {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil || ip == nil || ip.To4() == nil {
		return false
	}
	return containsIPv4Uint32(reserved, ipv4ToUint32(ip))
}

func ipv4ProbablyUsed(parent string, ip net.IP) bool {
	if _, err := exec.LookPath("ping"); err != nil {
		return false
	}

	ipStr := ip.String()
	_ = exec.Command("ip", "neigh", "flush", ipStr, "dev", parent).Run()
	_ = exec.Command("ping", "-4", "-c", "1", "-W", "1", "-I", parent, ipStr).Run()

	out, err := exec.Command("ip", "neigh", "show", ipStr, "dev", parent).Output()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, "lladdr") &&
		!strings.Contains(s, "FAILED") &&
		!strings.Contains(s, "INCOMPLETE")
}

func ipv4ToUint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32(ip.To4())
}

func uint32ToIPv4(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

func ensureCommand(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("找不到 %s 命令: %w", name, err)
	}
	return nil
}

func ensureHostMacvlan(parent, ifname string) error {
	if parent == "" {
		return fmt.Errorf("父网卡不能为空")
	}
	if ifname == "" {
		return fmt.Errorf("shim 接口名不能为空")
	}

	if err := exec.Command("ip", "link", "show", parent).Run(); err != nil {
		return fmt.Errorf("父网卡 %s 不存在或不可用: %w", parent, err)
	}

	if err := exec.Command("ip", "link", "show", ifname).Run(); err == nil {
		return nil
	}

	cmd := exec.Command("ip", "link", "add", ifname, "link", parent, "type", "macvlan", "mode", "bridge")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建 macvlan shim 失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// configureHostMacvlanIsolation 避免宿主机物理网卡 ens18 和 macvlan shim 在同一二层
// 网络内互相替对方的 IPv4 地址响应 ARP。否则路由器可能会看到同一个宿主机 IP
// 同时对应 ens18 的物理 MAC 和 sb-lxc-mv 的虚拟 MAC（ARP flux）。
//
// 该函数只调整运行时 sysctl，不会修改 ens18 的物理 MAC，也不会修改 ens18 的 IPv4。
func configureHostMacvlanIsolation(parent, ifname string) error {
	settings := []struct {
		path  string
		value string
		desc  string
	}{
		{"/proc/sys/net/ipv4/conf/" + parent + "/arp_ignore", "1", parent + " arp_ignore"},
		{"/proc/sys/net/ipv4/conf/" + parent + "/arp_announce", "2", parent + " arp_announce"},
		{"/proc/sys/net/ipv4/conf/" + ifname + "/arp_ignore", "1", ifname + " arp_ignore"},
		{"/proc/sys/net/ipv4/conf/" + ifname + "/arp_announce", "2", ifname + " arp_announce"},
		// shim 只承担 IPv4 /32 路由用途，关闭它的 IPv6 RA/自动地址可避免路由器
		// 把 sb-lxc-mv 当成一台额外 IPv6 终端展示。
		{"/proc/sys/net/ipv6/conf/" + ifname + "/accept_ra", "0", ifname + " accept_ra"},
		{"/proc/sys/net/ipv6/conf/" + ifname + "/autoconf", "0", ifname + " autoconf"},
		{"/proc/sys/net/ipv6/conf/" + ifname + "/disable_ipv6", "1", ifname + " disable_ipv6"},
	}

	for _, setting := range settings {
		if err := writeProcSysIfExists(setting.path, setting.value); err != nil {
			return fmt.Errorf("配置 %s 失败: %w", setting.desc, err)
		}
	}

	// 如果 shim 之前已经通过 RA 获得了 IPv6 地址，尽量清掉；失败不影响 IPv4 互通。
	_ = exec.Command("ip", "-6", "addr", "flush", "dev", ifname).Run()
	return nil
}

func writeProcSysIfExists(path, value string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o644)
}

func replaceAddr(ifname, cidr string) error {
	cmd := exec.Command("ip", "addr", "replace", cidr, "dev", ifname)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("配置 shim 地址失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func linkUp(ifname string) error {
	cmd := exec.Command("ip", "link", "set", ifname, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("启用 shim 接口失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func replaceRoute(target, ifname, src string) error {
	args := []string{"route", "replace", target, "dev", ifname}
	if src != "" {
		args = append(args, "src", src)
	}
	cmd := exec.Command("ip", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("配置路由失败: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runningContainerIPv4Routes(client *IncusClient) []string {
	cs, err := client.ListContainers()
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	routes := []string{}
	for _, ct := range cs {
		if !strings.EqualFold(ct.Status, "Running") {
			continue
		}
		ip := ct.IPv4()
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		routes = append(routes, ip+"/32")
	}
	return routes
}

func validateCIDR(cidr string) error {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	if ip == nil || ip.To4() == nil {
		return errors.New("仅支持 IPv4 CIDR")
	}
	return nil
}

func validateHostShimCIDR(cidr string) error {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	if ip == nil || ip.To4() == nil {
		return errors.New("仅支持 IPv4 CIDR")
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones != 32 {
		return errors.New("shim 地址必须使用 /32，避免宿主机整段局域网路由被改走 shim")
	}
	return nil
}

func normalizeRouteTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", errors.New("不能为空")
	}

	if strings.Contains(target, "/") {
		if err := validateCIDR(target); err != nil {
			return "", err
		}
		return target, nil
	}

	ip := net.ParseIP(target)
	if ip == nil || ip.To4() == nil {
		return "", errors.New("仅支持 IPv4 地址或 IPv4 CIDR")
	}
	return target + "/32", nil
}
