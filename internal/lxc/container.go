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
	out, err := s.exec.Run("lxc-ls", "-f")
	if err != nil {
		return out, err
	}

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) <= 1 {
		return "", nil
	}

	stateMap := map[string]string{
		"RUNNING": "run",
		"STOPPED": "stop",
		"FROZEN":  "frozen",
	}

	result := []string{}
	// 自定义表头，去掉 GROUPS 列
	result = append(result, fmt.Sprintf("%-16s %-6s %-5s %-16s", "NAME", "STATE", "AUTO", "IPV4"))

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		name := fields[0]

		state := stateMap[strings.ToUpper(fields[1])]
		if state == "" {
			state = strings.ToLower(fields[1])
		}

		autostart := "no"
		if fields[2] == "1" {
			autostart = "yes"
		}

		ipv4 := fields[4]

		result = append(result, fmt.Sprintf("%-16s %-6s %-5s %-16s", name, state, autostart, ipv4))
	}

	return strings.Join(result, "\n") + "\n", nil
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
	Autostart string
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
	}

	return status, nil
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
