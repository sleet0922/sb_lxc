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
