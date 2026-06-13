package lxc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"sb_lxc/internal/core"
)

type NetworkService struct {
	exec core.Executor
}

func NewNetworkService(exec core.Executor) *NetworkService {
	return &NetworkService{exec: exec}
}

type PortForward struct {
	HostPort      int
	ContainerPort int
}

// hostPortInUse 检查宿主机端口是否已被其他容器占用
func hostPortInUse(hostPort int, excludeContainer string) (string, error) {
	entries, err := os.ReadDir("/var/lib/lxc")
	if err != nil {
		return "", nil
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == excludeContainer {
			continue
		}
		mappingPath := filepath.Join("/var/lib/lxc", e.Name(), "port-mappings")
		data, err := os.ReadFile(mappingPath)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				hp, _ := strconv.Atoi(parts[0])
				if hp == hostPort {
					return e.Name(), nil
				}
			}
		}
	}
	return "", nil
}

func (s *NetworkService) AddPortForward(name string, containerPort, hostPort int) error {
	if hostPort < 1 || hostPort > 65535 || containerPort < 1 || containerPort > 65535 {
		return fmt.Errorf("端口号必须在 1-65535 之间")
	}

	mappingPath := filepath.Join("/var/lib/lxc", name, "port-mappings")

	existing, _ := s.ListPortForwards(name)
	for _, pf := range existing {
		if pf.HostPort == hostPort {
			return fmt.Errorf("宿主机端口 %d 已经映射到容器 %s 的 %d 端口", hostPort, name, pf.ContainerPort)
		}
	}

	if conflict, err := hostPortInUse(hostPort, name); err == nil && conflict != "" {
		return fmt.Errorf("宿主机端口 %d 已被容器 %s 占用", hostPort, conflict)
	}

	f, err := os.OpenFile(mappingPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开端口映射文件: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%d %d\n", hostPort, containerPort); err != nil {
		return fmt.Errorf("写入端口映射失败: %w", err)
	}

	CreatePortForwardScript(name)
	return ApplyPortForwards(name)
}

func (s *NetworkService) ListPortForwards(name string) ([]PortForward, error) {
	mappingPath := filepath.Join("/var/lib/lxc", name, "port-mappings")
	f, err := os.Open(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var forwards []PortForward
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hp, _ := strconv.Atoi(parts[0])
			cp, _ := strconv.Atoi(parts[1])
			if hp > 0 && cp > 0 {
				forwards = append(forwards, PortForward{HostPort: hp, ContainerPort: cp})
			}
		}
	}
	return forwards, scanner.Err()
}

func (s *NetworkService) RemovePortForward(name string, hostPort int) error {
	forwards, err := s.ListPortForwards(name)
	if err != nil {
		return err
	}

	var remaining []PortForward
	found := false
	for _, pf := range forwards {
		if pf.HostPort == hostPort {
			found = true
		} else {
			remaining = append(remaining, pf)
		}
	}
	if !found {
		return fmt.Errorf("未找到宿主机端口 %d 的映射", hostPort)
	}

	mappingPath := filepath.Join("/var/lib/lxc", name, "port-mappings")
	if len(remaining) == 0 {
		os.Remove(mappingPath)
	} else {
		var sb strings.Builder
		for _, pf := range remaining {
			sb.WriteString(fmt.Sprintf("%d %d\n", pf.HostPort, pf.ContainerPort))
		}
		if err := os.WriteFile(mappingPath, []byte(sb.String()), 0644); err != nil {
			return err
		}
	}

	removeIptableRule(name, hostPort)
	CreatePortForwardScript(name)
	return ApplyPortForwards(name)
}

func addIptablesDNATRules(name, containerIP, comment string, hostPort, containerPort int) {
	// PREROUTING DNAT — 外部流量
	exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", "tcp", "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
		"-m", "comment", "--comment", comment).Run()

	// POSTROUTING MASQUERADE — 回包伪装
	exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-p", "tcp", "-d", containerIP, "--dport", strconv.Itoa(containerPort),
		"-j", "MASQUERADE",
		"-m", "comment", "--comment", comment).Run()

	// OUTPUT DNAT — 宿主机自身访问（127.0.0.1 或 本机IP）
	exec.Command("iptables", "-t", "nat", "-A", "OUTPUT",
		"-p", "tcp", "--dport", strconv.Itoa(hostPort),
		"-m", "addrtype", "--dst-type", "LOCAL",
		"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
		"-m", "comment", "--comment", comment).Run()

	// FORWARD ACCEPT — 允许转发
	exec.Command("iptables", "-A", "FORWARD",
		"-p", "tcp", "-d", containerIP, "--dport", strconv.Itoa(containerPort),
		"-j", "ACCEPT",
		"-m", "comment", "--comment", comment).Run()
}

// clearAllPortRules 清除所有容器中指定宿主机端口的 iptables 规则（防止旧容器残留规则抢占）
func clearAllPortRules(hostPort int) {
	marker := fmt.Sprintf("sb-lxc-port-.*-%d[^0-9]", hostPort)
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t nat 2>/dev/null | grep -vE '%s' | iptables-restore -T nat 2>/dev/null", marker)).Run()
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t filter 2>/dev/null | grep -vE '%s' | iptables-restore -T filter 2>/dev/null", marker)).Run()
}

func ApplyPortForwards(name string) error {
	containerIP, err := getContainerIP(name)
	if err != nil {
		return nil
	}

	mappingPath := filepath.Join("/var/lib/lxc", name, "port-mappings")
	data, err := os.ReadFile(mappingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// 先清除该容器的所有旧规则，再逐个端口清除跨容器残留规则
	clearContainerRules(name)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])
		if hostPort < 1 || containerPort < 1 {
			continue
		}

		// 清除其他容器中该宿主机端口的残留规则
		clearAllPortRules(hostPort)

		comment := fmt.Sprintf("sb-lxc-port-%s-%d", name, hostPort)
		addIptablesDNATRules(name, containerIP, comment, hostPort, containerPort)
	}

	return nil
}

func clearContainerRules(name string) {
	marker := "sb-lxc-port-" + name
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t nat 2>/dev/null | grep -v '%s' | iptables-restore -T nat 2>/dev/null", marker)).Run()
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t filter 2>/dev/null | grep -v '%s' | iptables-restore -T filter 2>/dev/null", marker)).Run()
}

func removeIptableRule(name string, hostPort int) {
	comment := fmt.Sprintf("sb-lxc-port-%s-%d", name, hostPort)
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t nat 2>/dev/null | grep -v '%s' | iptables-restore -T nat 2>/dev/null", comment)).Run()
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t filter 2>/dev/null | grep -v '%s' | iptables-restore -T filter 2>/dev/null", comment)).Run()
}

func getContainerIP(name string) (string, error) {
	svc := NewContainerService(&core.ShellExecutor{})
	return svc.GetIP(name)
}

func CreatePortForwardScript(name string) {
	scriptPath := filepath.Join("/var/lib/lxc", name, "port-forward.sh")
	content := fmt.Sprintf(`#!/bin/sh
NAME="%s"

# 先清除该容器的所有旧规则
iptables-save -t nat 2>/dev/null | grep -v "sb-lxc-port-$NAME" | iptables-restore -T nat 2>/dev/null
iptables-save -t filter 2>/dev/null | grep -v "sb-lxc-port-$NAME" | iptables-restore -T filter 2>/dev/null

IP=$(lxc-info -n "$NAME" -iH 2>/dev/null | grep '\.' | head -1 | tr -d ' \n\t')
if [ -z "$IP" ]; then
  exit 0
fi

MAPPING_FILE="/var/lib/lxc/$NAME/port-mappings"
if [ ! -f "$MAPPING_FILE" ]; then
  exit 0
fi

while read -r host_port container_port; do
  [ -z "$host_port" ] && continue

  # 清除其他容器中该宿主机端口的残留规则
  iptables-save -t nat 2>/dev/null | grep -vE "sb-lxc-port-.*-$host_port[^0-9]" | iptables-restore -T nat 2>/dev/null
  iptables-save -t filter 2>/dev/null | grep -vE "sb-lxc-port-.*-$host_port[^0-9]" | iptables-restore -T filter 2>/dev/null

  iptables -t nat -A PREROUTING -p tcp --dport "$host_port" -j DNAT --to-destination "$IP:$container_port" -m comment --comment "sb-lxc-port-$NAME-$host_port" 2>/dev/null
  iptables -t nat -A POSTROUTING -p tcp -d "$IP" --dport "$container_port" -j MASQUERADE -m comment --comment "sb-lxc-port-$NAME-$host_port" 2>/dev/null
  iptables -t nat -A OUTPUT -p tcp --dport "$host_port" -m addrtype --dst-type LOCAL -j DNAT --to-destination "$IP:$container_port" -m comment --comment "sb-lxc-port-$NAME-$host_port" 2>/dev/null
  iptables -A FORWARD -p tcp -d "$IP" --dport "$container_port" -j ACCEPT -m comment --comment "sb-lxc-port-$NAME-$host_port" 2>/dev/null
done < "$MAPPING_FILE"
`, name)

	os.WriteFile(scriptPath, []byte(content), 0755)
}

func ensureRouteLocalnet() {
	sysctlPath := "/etc/sysctl.d/99-sb-lxc-port.conf"
	content := "net.ipv4.conf.all.route_localnet=1\n"

	// 写入 sysctl 配置文件（持久化）
	if _, err := os.Stat(sysctlPath); os.IsNotExist(err) {
		os.WriteFile(sysctlPath, []byte(content), 0644)
	}

	// 立即生效
	exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run()
}

func EnsurePortForwardService() error {
	ensureRouteLocalnet()

	svcPath := "/etc/systemd/system/sb-lxc-port@.service"
	if _, err := os.Stat(svcPath); err == nil {
		return nil
	}

	svcContent := `[Unit]
Description=Port forwarding for LXC container %i
After=lxc.service
Wants=lxc.service

[Service]
Type=oneshot
ExecStart=/var/lib/lxc/%i/port-forward.sh

[Install]
WantedBy=multi-user.target`

	if err := os.WriteFile(svcPath, []byte(svcContent), 0644); err != nil {
		return fmt.Errorf("创建 systemd 服务文件失败: %w", err)
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("重载 systemd 失败: %w\n%s", err, string(out))
	}
	return nil
}
