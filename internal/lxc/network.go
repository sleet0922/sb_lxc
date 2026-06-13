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

	f, err := os.OpenFile(mappingPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开端口映射文件: %w", err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%d %d\n", hostPort, containerPort); err != nil {
		return fmt.Errorf("写入端口映射失败: %w", err)
	}

	createPortForwardScript(name)
	return applyPortForwards(name)
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
	createPortForwardScript(name)
	return applyPortForwards(name)
}

// applyPortForwards 清除旧规则并应用当前所有端口映射
func applyPortForwards(name string) error {
	clearContainerRules(name)

	containerIP, err := getContainerIP(name)
	if err != nil {
		// 容器未运行，跳过即时应用（脚本会在容器启动时执行）
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

		comment := fmt.Sprintf("sb-lxc-%s-%d", name, hostPort)

		// DNAT 规则
		exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
			"-p", "tcp", "--dport", strconv.Itoa(hostPort),
			"-j", "DNAT", "--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
			"-m", "comment", "--comment", comment).Run()

		// FORWARD 规则
		exec.Command("iptables", "-A", "FORWARD",
			"-p", "tcp", "-d", containerIP, "--dport", strconv.Itoa(containerPort),
			"-j", "ACCEPT",
			"-m", "comment", "--comment", comment).Run()
	}

	return nil
}

// clearContainerRules 清除指定容器的所有 iptables 规则
func clearContainerRules(name string) {
	marker := "sb-lxc-" + name
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t nat 2>/dev/null | grep -v '%s' | iptables-restore -T nat 2>/dev/null", marker)).Run()
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t filter 2>/dev/null | grep -v '%s' | iptables-restore -T filter 2>/dev/null", marker)).Run()
}

// removeIptableRule 移除单条 iptables 规则（端口级别）
func removeIptableRule(name string, hostPort int) {
	comment := fmt.Sprintf("sb-lxc-%s-%d", name, hostPort)
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t nat 2>/dev/null | grep -v '%s' | iptables-restore -T nat 2>/dev/null", comment)).Run()
	exec.Command("sh", "-c",
		fmt.Sprintf("iptables-save -t filter 2>/dev/null | grep -v '%s' | iptables-restore -T filter 2>/dev/null", comment)).Run()
}

func getContainerIP(name string) (string, error) {
	svc := NewContainerService(&core.ShellExecutor{})
	return svc.GetIP(name)
}

// createPortForwardScript 创建端口转发脚本，用于容器启动时自动应用规则
func createPortForwardScript(name string) {
	scriptPath := filepath.Join("/var/lib/lxc", name, "port-forward.sh")
	content := fmt.Sprintf(`#!/bin/sh
NAME="%s"
# 清除该容器的旧规则
iptables-save -t nat 2>/dev/null | grep -v "sb-lxc-$NAME" | iptables-restore -T nat 2>/dev/null
iptables-save -t filter 2>/dev/null | grep -v "sb-lxc-$NAME" | iptables-restore -T filter 2>/dev/null

# 获取容器 IP
IP=$(lxc-info -n "$NAME" -iH 2>/dev/null | head -1 | tr -d ' \n\t')
if [ -z "$IP" ]; then
  exit 0
fi

# 读取映射并应用规则
MAPPING_FILE="/var/lib/lxc/$NAME/port-mappings"
if [ ! -f "$MAPPING_FILE" ]; then
  exit 0
fi

while read -r host_port container_port; do
  [ -z "$host_port" ] && continue
  iptables -t nat -A PREROUTING -p tcp --dport "$host_port" -j DNAT --to-destination "$IP:$container_port" -m comment --comment "sb-lxc-$NAME-$host_port" 2>/dev/null
  iptables -A FORWARD -p tcp -d "$IP" --dport "$container_port" -j ACCEPT -m comment --comment "sb-lxc-$NAME-$host_port" 2>/dev/null
done < "$MAPPING_FILE"
`, name)

	os.WriteFile(scriptPath, []byte(content), 0755)
}

// EnsurePortForwardService 确保 systemd 服务文件存在
func EnsurePortForwardService() error {
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