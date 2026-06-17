package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// hostsMark 用于标记 sb_lxc 管理的 /etc/hosts 行，便于按容器名更新/移除。
func hostsMark(name string) string {
	return "# sb_lxc:" + name
}

// updateHosts 更新 /etc/hosts：将该容器域名的行更新为新 IP，不存在则追加。
// 行格式: "<ip> <domain>  # sb_lxc:<容器名>"
func updateHosts(name, domain, ip string) error {
	mark := hostsMark(name)
	data, _ := os.ReadFile("/etc/hosts")
	lines := strings.Split(string(data), "\n")

	found := false
	for i, line := range lines {
		if strings.Contains(line, mark) {
			lines[i] = fmt.Sprintf("%s\t%s\t%s", ip, domain, mark)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", ip, domain, mark))
	}

	out := strings.Join(lines, "\n")
	return os.WriteFile("/etc/hosts", []byte(out), 0644)
}

// removeHostsLine 从 /etc/hosts 移除该容器的映射行。
func removeHostsLine(name string) error {
	mark := hostsMark(name)
	data, _ := os.ReadFile("/etc/hosts")
	lines := strings.Split(string(data), "\n")

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, mark) {
			out = append(out, line)
		}
	}
	return os.WriteFile("/etc/hosts", []byte(strings.Join(out, "\n")), 0644)
}

// waitForIP 轮询容器 IPv4，最多等待 maxWait 秒。
func waitForIP(client *IncusClient, name string, maxWait int) string {
	for i := 0; i < maxWait; i++ {
		ct, err := client.GetContainer(name)
		if err == nil && ct.IPv4() != "" {
			return ct.IPv4()
		}
		time.Sleep(time.Second)
	}
	return ""
}
