package lxc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sb_lxc/internal/core"
)

type ContainerService struct {
	exec core.Executor
}

func NewContainerService(exec core.Executor) *ContainerService {
	return &ContainerService{exec: exec}
}

func (s *ContainerService) List() (string, error) {
	return s.exec.Run("lxc-ls")
}

func (s *ContainerService) ListDetailed() (string, error) {
	return s.exec.Run("lxc-ls", "-f")
}

func (s *ContainerService) Info(name string) (string, error) {
	return s.exec.Run("lxc-info", "-n", name)
}

func (s *ContainerService) Start(name string, detach bool) (string, error) {
	args := []string{"-n", name}
	if detach {
		args = append(args, "-d")
	}
	return s.exec.Run("lxc-start", args...)
}

func (s *ContainerService) Stop(name string, force bool) (string, error) {
	args := []string{"-n", name}
	if force {
		args = append(args, "-k")
	}
	return s.exec.Run("lxc-stop", args...)
}

func (s *ContainerService) Destroy(name string) (string, error) {
	return s.exec.Run("lxc-destroy", "-n", name)
}

func (s *ContainerService) Freeze(name string) (string, error) {
	return s.exec.Run("lxc-freeze", "-n", name)
}

func (s *ContainerService) Unfreeze(name string) (string, error) {
	return s.exec.Run("lxc-unfreeze", "-n", name)
}

func (s *ContainerService) Autostart() (string, error) {
	return s.exec.Run("lxc-autostart")
}

type ContainerStatus struct {
	Name      string
	Autostart string // "enabled", "disabled", "not_set"
	PortMaps  []PortMapInfo
}

type PortMapInfo struct {
	ContainerPort string
	HostPort      string
}

func (s *ContainerService) Status(name string) (*ContainerStatus, error) {
	configPath := filepath.Join("/var/lib/lxc", name, "config")

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取容器配置失败: %w", err)
	}

	status := &ContainerStatus{
		Name:      name,
		Autostart: "not_set",
		PortMaps:  []PortMapInfo{},
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "lxc.start.auto") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 {
				if fields[2] == "1" {
					status.Autostart = "enabled"
				} else {
					status.Autostart = "disabled"
				}
			}
		}

		if strings.HasPrefix(trimmed, "lxc.hook.pre-start") && strings.Contains(trimmed, "iptables") {
			// 旧格式: lxc.hook.pre-start = sh -c "iptables ..."
			dportMatch := extractPort(trimmed, "--dport")
			destMatch := extractPortAfterColon(trimmed, "--to-destination")
			if dportMatch != "" && destMatch != "" {
				status.PortMaps = append(status.PortMaps, PortMapInfo{
					HostPort:      dportMatch,
					ContainerPort: destMatch,
				})
			}
		}

		// 新格式: lxc.hook.pre-start = /var/lib/lxc/<name>/port-forward.sh
		if strings.HasPrefix(trimmed, "lxc.hook.pre-start") && strings.Contains(trimmed, "port-forward.sh") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 {
				scriptPath := fields[2]
				scriptContent, err := os.ReadFile(scriptPath)
				if err == nil {
					dportMatch := extractPort(string(scriptContent), "--dport")
					destMatch := extractPortAfterColon(string(scriptContent), "--to-destination")
					if dportMatch != "" && destMatch != "" {
						status.PortMaps = append(status.PortMaps, PortMapInfo{
							HostPort:      dportMatch,
							ContainerPort: destMatch,
						})
					}
				}
			}
		}
	}

	return status, nil
}

func extractPort(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	after := s[idx+len(prefix):]
	after = strings.TrimSpace(after)
	parts := strings.Fields(after)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func extractPortAfterColon(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	after := s[idx+len(prefix):]
	after = strings.TrimSpace(after)
	parts := strings.Fields(after)
	if len(parts) > 0 {
		// format: 10.0.3.X:PORT
		colonIdx := strings.LastIndex(parts[0], ":")
		if colonIdx != -1 && colonIdx+1 < len(parts[0]) {
			return parts[0][colonIdx+1:]
		}
	}
	return ""
}

func (s *ContainerService) GetIP(name string) (string, error) {
	ip, err := s.exec.Run("lxc-info", "-n", name, "-iH")
	if err != nil {
		return "", err
	}
	ip = strings.TrimSpace(ip)
	lines := strings.Split(ip, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ".") {
			return line, nil
		}
	}
	return "", fmt.Errorf("未能获取容器 %s 的 IPv4 地址", name)
}

func (s *ContainerService) SetAutostart(name string, enabled bool) (string, error) {
	configPath := filepath.Join("/var/lib/lxc", name, "config")

	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	value := "0"
	if enabled {
		value = "1"
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "lxc.start.auto") {
			lines[i] = "lxc.start.auto = " + value
			found = true
		}
	}

	if !found {
		lines = append(lines, "lxc.start.auto = "+value)
	}

	updated := strings.Join(lines, "\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}

	if err := os.WriteFile(configPath, []byte(updated), 0644); err != nil {
		return "", err
	}

	if enabled {
		return fmt.Sprintf("已启用容器 %s 的开机自启。", name), nil
	}

	return fmt.Sprintf("已禁用容器 %s 的开机自启。", name), nil
}
