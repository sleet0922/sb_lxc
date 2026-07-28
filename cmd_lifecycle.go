package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CmdStart 启动容器，若配置了域名映射则自动更新 /etc/hosts。
func CmdStart(name string) error {
	client := NewIncusClient()

	// 容器已在运行则跳过启动，继续处理域名逻辑
	ct, _ := client.GetContainer(name)
	if ct != nil && strings.EqualFold(ct.Status, "Running") {
		fmt.Printf("ℹ 容器 %s 已在运行\n", name)
	} else {
		fmt.Printf("启动容器 %s ...\n", name)
		if err := client.Start(name); err != nil {
			return err
		}
		// 重新获取容器信息以读取域名配置
		ct, _ = client.GetContainer(name)
	}

	// 等待容器 IPv4，并自动补齐宿主机侧 macvlan shim 路由。
	ip := ""
	domain := ""
	if ct != nil {
		ip = ct.IPv4()
		domain = ct.Domain()
	}
	if ip == "" {
		ip = waitForIP(client, name, 15)
	}
	if ip != "" {
		warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))
	}

	// 刷新端口映射的 connect 地址，应对 DHCP 重新分配导致容器 IP 变化的场景。
	if refreshed, rerr := client.RefreshPortMappings(name); rerr != nil {
		fmt.Printf("⚠ 刷新端口映射失败: %v\n", rerr)
	} else if refreshed > 0 {
		fmt.Printf("✔ 已刷新 %d 条端口映射的容器 IP (%s)\n", refreshed, ip)
	}

	// 检查域名映射，有则更新 /etc/hosts。
	if domain == "" {
		return nil
	}
	if ip == "" {
		fmt.Printf("⚠ 容器未获取到 IPv4，跳过 hosts 更新\n")
		return nil
	}
	if err := updateHosts(name, domain, ip); err != nil {
		return fmt.Errorf("更新 /etc/hosts 失败: %w", err)
	}
	fmt.Printf("✔ 已更新 /etc/hosts: %s -> %s\n", domain, ip)
	return nil
}

// CmdUninstall 交互式选择并删除容器，并清理 /etc/hosts 与 /32 路由残留。
func CmdUninstall() error {
	name, err := selectContainer("选择要删除的容器")
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	client := NewIncusClient()
	// 先获取容器信息用于清理路由
	ct, _ := client.GetContainer(name)
	fmt.Printf("停止容器 %s ...\n", name)
	_ = client.Stop(name)
	fmt.Printf("删除容器 %s ...\n", name)
	if err := client.Delete(name); err != nil {
		return err
	}
	// 清理 /etc/hosts 残留行
	if err := removeHostsLine(name); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ 清理 /etc/hosts 失败: %v\n", err)
	}
	// 清理 /32 路由残留
	if ct != nil {
		if ip := ct.IPv4(); ip != "" {
			removeHostMacvlanRoute(ip)
		}
	}
	fmt.Printf("✔ 容器 %s 已删除\n", name)
	return nil
}

// CmdStop 停止容器，并清理对应的 /32 路由避免死路由堆积。
func CmdStop(name string) error {
	fmt.Printf("停止容器 %s ...\n", name)
	client := NewIncusClient()
	// 先获取容器 IP 用于后续路由清理
	ct, _ := client.GetContainer(name)
	if err := client.Stop(name); err != nil {
		return err
	}
	// 清理 /32 路由
	if ct != nil {
		if ip := ct.IPv4(); ip != "" {
			removeHostMacvlanRoute(ip)
		}
	}
	return nil
}

// CmdIn 进入容器（交互式透传 stdio）。
func CmdIn(name string) error {
	return NewIncusClient().Exec(name)
}

// CmdExport 导出容器为 ./容器名_YYYYMMDD_HHMMSS.tar.gz。
func CmdExport(name string) error {
	ts := time.Now().Format("20060102_150405")
	path := fmt.Sprintf("./%s_%s.tar.gz", name, ts)
	fmt.Printf("导出容器 %s -> %s ...\n", name, path)
	if err := NewIncusClient().Export(name, path); err != nil {
		return err
	}
	fmt.Printf("✔ 已导出到 %s\n", path)
	return nil
}

// CmdImport 从备份文件导入容器。有参数则直接使用，无参数则扫描当前目录 .tar.gz 供选择。
func CmdImport(args []string) error {
	path := ""
	name := ""
	if len(args) >= 1 {
		path = args[0]
	} else {
		// 扫描当前目录下的 .tar.gz 文件
		matches, _ := filepath.Glob("*.tar.gz")
		if len(matches) == 0 {
			return fmt.Errorf("当前目录未找到 .tar.gz 文件")
		}
		choice := selectMenu(matches, "选择要导入的文件 (↑↓ 选择, Enter 确认, q 退出)")
		if choice < 0 {
			return nil
		}
		path = matches[choice]
	}
	if len(args) >= 2 {
		name = args[1]
	}
	fmt.Printf("导入 %s ...\n", path)
	client := NewIncusClient()
	var before map[string]bool
	if name == "" {
		before = containerNameSet(client)
	}
	if name != "" {
		if err := client.Import(path, name); err != nil {
			return err
		}
		if err := client.ConfigureImportedNetwork(name); err != nil {
			return err
		}
	} else {
		if err := client.Import(path, ""); err != nil {
			return err
		}
		importedName := findImportedContainerName(client, before)
		if importedName == "" {
			return fmt.Errorf("导入完成，但无法确认新容器名，未配置 macvlan 网络")
		}
		if err := client.ConfigureImportedNetwork(importedName); err != nil {
			return err
		}
	}
	warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))
	fmt.Printf("✔ 导入完成\n")
	return nil
}

func containerNameSet(client *IncusClient) map[string]bool {
	cs, err := client.ListContainers()
	if err != nil {
		return nil
	}
	names := make(map[string]bool, len(cs))
	for _, ct := range cs {
		names[ct.Name] = true
	}
	return names
}

func findImportedContainerName(client *IncusClient, before map[string]bool) string {
	if before == nil {
		return ""
	}
	cs, err := client.ListContainers()
	if err != nil {
		return ""
	}
	importedName := ""
	for _, ct := range cs {
		if before[ct.Name] {
			continue
		}
		if importedName != "" {
			return ""
		}
		importedName = ct.Name
	}
	return importedName
}
