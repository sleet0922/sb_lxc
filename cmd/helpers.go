package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// containerState 获取容器运行状态: "RUNNING", "STOPPED", "FROZEN", 或空串
func containerState(name string) string {
	out, _ := exec.Command("lxc-info", "-n", name, "-s").Output()
	// 输出格式: State: RUNNING
	s := strings.TrimSpace(string(out))
	if strings.HasPrefix(s, "State:") {
		return strings.TrimSpace(strings.TrimPrefix(s, "State:"))
	}
	return ""
}

// containerExists 检查容器是否存在
func containerExists(name string) bool {
	out, _ := exec.Command("lxc-ls").Output()
	for _, n := range strings.Fields(string(out)) {
		if n == name {
			return true
		}
	}
	return false
}

// requireContainer 检查容器是否存在，不存在则返回友好错误
func requireContainer(name string) error {
	if !containerExists(name) {
		return fmt.Errorf("容器 %s 不存在", name)
	}
	return nil
}

// requireRunning 检查容器是否在运行，不在运行则返回友好错误
func requireRunning(name string) error {
	if err := requireContainer(name); err != nil {
		return err
	}
	state := containerState(name)
	if state != "RUNNING" {
		return fmt.Errorf("容器 %s 未运行 (当前状态: %s)", name, stateText(state))
	}
	return nil
}

// requireStopped 检查容器是否已停止，未停止则返回友好错误
func requireStopped(name string) error {
	if err := requireContainer(name); err != nil {
		return err
	}
	state := containerState(name)
	if state == "RUNNING" {
		return fmt.Errorf("容器 %s 正在运行，请先停止容器", name)
	}
	return nil
}

// stateText 将英文状态转为中文
func stateText(state string) string {
	switch state {
	case "RUNNING":
		return "运行中"
	case "STOPPED":
		return "已停止"
	case "FROZEN":
		return "已冻结"
	default:
		return state
	}
}
