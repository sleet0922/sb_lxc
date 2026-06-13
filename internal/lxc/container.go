package lxc

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	State     string
	Autostart string
	CPU       string
	Memory    string
}

func (s *ContainerService) Status(name string) (*ContainerStatus, error) {
	status := &ContainerStatus{
		Name: name,
	}

	// 从配置文件读取开机自启状态
	configPath := filepath.Join("/var/lib/lxc", name, "config")
	if content, err := os.ReadFile(configPath); err == nil {
		status.Autostart = "not_set"
		for _, line := range strings.Split(string(content), "\n") {
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
	}

	// 从 lxc-info 获取运行状态
	info, err := s.exec.Run("lxc-info", "-n", name)
	if err == nil {
		for _, line := range strings.Split(info, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "State:") {
				status.State = strings.TrimSpace(strings.TrimPrefix(line, "State:"))
			}
		}
	}

	// 从 cgroup v2 读取 CPU 和内存（仅在容器运行时）
	if status.State == "RUNNING" {
		cgPath := "/sys/fs/cgroup/lxc.payload." + name
		memPath := filepath.Join(cgPath, "memory.current")

		// CPU: 采样两次 cpu.stat，计算实时 CPU%
		cpuStatPath := filepath.Join(cgPath, "cpu.stat")
		sample1 := readUsageUsec(cpuStatPath)
		if sample1 > 0 {
			time.Sleep(500 * time.Millisecond)
			sample2 := readUsageUsec(cpuStatPath)
			if sample2 > sample1 {
				delta := sample2 - sample1
				cpuPct := float64(delta) / 5_000 // 500ms = 500000us, so delta/500000*100 = delta/5000
				status.CPU = fmt.Sprintf("%.1f%%", cpuPct)
			} else {
				status.CPU = "< 0.1%"
			}
		}

		// 内存: 读取 memory.current
		if data, err := os.ReadFile(memPath); err == nil {
			if bytes, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
				mib := float64(bytes) / 1024 / 1024
				if mib < 1 {
					kib := math.Round(float64(bytes)/1024*10) / 10
					status.Memory = fmt.Sprintf("%.1f KiB", kib)
				} else {
					status.Memory = fmt.Sprintf("%.1f MiB", math.Round(mib*10)/10)
				}
			}
		}
	}

	return status, nil
}

// readUsageUsec 从 cpu.stat 读取 usage_usec 值
func readUsageUsec(path string) uint64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "usage_usec") {
			fields := strings.Fields(line)
			if len(fields) == 2 {
				val, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return val
				}
			}
			break
		}
	}
	return 0
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
