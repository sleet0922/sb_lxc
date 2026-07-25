package main

import (
	"fmt"
	"strings"
)

// CmdSet 容器设置：交互式菜单或直接命令行参数设置。
func CmdSet(args []string) error {
	client := NewIncusClient()
	name := ""
	if len(args) >= 1 {
		name = args[0]
	} else {
		var err error
		name, err = selectContainer("选择要设置的容器")
		if err != nil || name == "" {
			return err
		}
	}

	ct, err := client.GetContainer(name)
	if err != nil {
		return err
	}

	if len(args) >= 2 {
		sub := strings.ToLower(args[1])
		switch sub {
		case "domain", "host", "dns":
			if len(args) >= 3 && args[2] != "--unset" && args[2] != "" {
				return applyDomain(client, ct, args[2])
			}
			return removeDomain(client, ct)
		case "autostart":
			on := true
			if len(args) >= 3 && (args[2] == "off" || args[2] == "false" || args[2] == "0") {
				on = false
			}
			if err := client.SetBootAutostart(name, on); err != nil {
				return err
			}
			if on {
				fmt.Printf("✔ 容器 %s 已开启开机自启动\n", name)
			} else {
				fmt.Printf("✔ 容器 %s 已关闭开机自启动\n", name)
			}
			return nil
		}
	}

	options := []string{
		"域名映射",
		"取消域名映射",
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
		domain := prompt("域名 (如 alpine.test): ")
		return applyDomain(client, ct, domain)
	case 1:
		return removeDomain(client, ct)
	case 2:
		if err := client.SetBootAutostart(name, true); err != nil {
			return err
		}
		fmt.Printf("✔ 容器 %s 已开启开机自启动\n", name)
	case 3:
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

// setDomain 设置域名映射，若容器已运行则立即更新 /etc/hosts。
func applyDomain(client *IncusClient, ct *Container, domain string) error {
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
