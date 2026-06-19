package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// CmdSet 容器设置：交互式菜单选择端口映射 / 取消端口映射 / 开关开机自启。
func CmdSet(name string) error {
	client := NewIncusClient()
	ct, err := client.GetContainer(name)
	if err != nil {
		return err
	}

	options := []string{
		"端口映射",
		"取消端口映射",
		"域名映射",
		"取消域名映射",
		"挂载文件夹",
		"取消挂载",
		"开机自启动",
		"关闭开机自启动",
	}

	fmt.Printf("容器: %s  (状态: %s, 自启: %s, 域名: %s)\n",
		name, strings.ToLower(ct.Status), autostartBadge(ct.Autostart()), orNA(ct.Domain()))
	choice := selectMenu(options, "选择操作 (↑↓ 选择, Enter 确认, q 退出)")
	if choice < 0 {
		return nil
	}

	switch choice {
	case 0:
		return addPortMapping(client, name)
	case 1:
		return removePortMapping(client, ct)
	case 2:
		return setDomain(client, ct)
	case 3:
		return removeDomain(client, ct)
	case 4:
		return addMount(client, ct)
	case 5:
		return removeMount(client, ct)
	case 6:
		if err := client.SetBootAutostart(name, true); err != nil {
			return err
		}
		fmt.Printf("✔ 容器 %s 已开启开机自启动\n", name)
	case 7:
		if err := client.SetBootAutostart(name, false); err != nil {
			return err
		}
		fmt.Printf("✔ 容器 %s 已关闭开机自启动\n", name)
	}
	return nil
}

// orNA 空值显示为 N/A。
func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// addPortMapping 交互式添加端口映射，全监听 0.0.0.0。
func addPortMapping(client *IncusClient, name string) error {
	r := bufio.NewReader(os.Stdin)
	hostPort := prompt(r, "宿主机端口: ")
	if hostPort == "" {
		return fmt.Errorf("宿主机端口不能为空")
	}
	containerPort := prompt(r, "容器端口: ")
	if containerPort == "" {
		return fmt.Errorf("容器端口不能为空")
	}

	device := "port" + hostPort
	listen := fmt.Sprintf("tcp:0.0.0.0:%s", hostPort)
	connect := fmt.Sprintf("tcp:0.0.0.0:%s", containerPort)

	if err := client.AddProxyDevice(name, device, listen, connect); err != nil {
		return err
	}
	fmt.Printf("✔ 端口映射已添加: 0.0.0.0:%s -> 0.0.0.0:%s (设备 %s)\n", hostPort, containerPort, device)
	return nil
}

// removePortMapping 列出已有端口映射供选择移除。
func removePortMapping(client *IncusClient, ct *Container) error {
	devs := ct.ProxyDevices()
	if len(devs) == 0 {
		fmt.Println("该容器没有端口映射设备。")
		return nil
	}

	names := make([]string, 0, len(devs))
	for k := range devs {
		names = append(names, k)
	}
	sort.Strings(names)

	labels := make([]string, len(names))
	for i, n := range names {
		d := devs[n]
		labels[i] = fmt.Sprintf("%s  (%s -> %s)", n, shortAddr(d["listen"]), shortAddr(d["connect"]))
	}

	choice := selectMenu(labels, "选择要移除的端口映射 (↑↓ 选择, Enter 确认, q 退出)")
	if choice < 0 {
		return nil
	}
	target := names[choice]
	if err := client.RemoveDevice(ct.Name, target); err != nil {
		return err
	}
	fmt.Printf("✔ 端口映射 %s 已移除\n", target)
	return nil
}

// setDomain 设置域名映射，若容器已运行则立即更新 /etc/hosts。
func setDomain(client *IncusClient, ct *Container) error {
	r := bufio.NewReader(os.Stdin)
	domain := prompt(r, "域名 (如 alpine.test): ")
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}
	if err := client.SetDomain(ct.Name, domain); err != nil {
		return err
	}
	fmt.Printf("✔ 域名映射已保存: %s\n", domain)

	// 容器运行中则立即写入 /etc/hosts
	if strings.EqualFold(ct.Status, "Running") {
		ip := ct.IPv4()
		if ip == "" {
			ip = waitForIP(client, ct.Name, 5)
		}
		if ip != "" {
			if err := updateHosts(ct.Name, domain, ip); err != nil {
				return fmt.Errorf("更新 /etc/hosts 失败: %w", err)
			}
			fmt.Printf("✔ 已更新 /etc/hosts: %s -> %s\n", domain, ip)
		} else {
			fmt.Printf("⚠ 容器未获取到 IPv4，将在下次启动时写入\n")
		}
	} else {
		fmt.Printf("ℹ 容器未运行，将在启动时自动写入 /etc/hosts\n")
	}
	return nil
}

// removeDomain 取消域名映射并移除 /etc/hosts 中的对应行。
func removeDomain(client *IncusClient, ct *Container) error {
	if ct.Domain() == "" {
		fmt.Println("该容器未配置域名映射。")
		return nil
	}
	if err := client.UnsetDomain(ct.Name); err != nil {
		return err
	}
	if err := removeHostsLine(ct.Name); err != nil {
		return fmt.Errorf("移除 /etc/hosts 行失败: %w", err)
	}
	fmt.Printf("✔ 域名映射已取消，并清理 /etc/hosts\n")
	return nil
}
