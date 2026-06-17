package main

import (
	"fmt"
	"path/filepath"
)

// CmdStart 启动容器，若配置了域名映射则自动更新 /etc/hosts。
func CmdStart(name string) error {
	client := NewIncusClient()
	fmt.Printf("启动容器 %s ...\n", name)
	if err := client.Start(name); err != nil {
		return err
	}

	// 检查域名映射，有则等待 IP 并更新 /etc/hosts
	ct, _ := client.GetContainer(name)
	domain := ""
	if ct != nil {
		domain = ct.Domain()
	}
	if domain == "" {
		return nil
	}

	fmt.Printf("检测到域名映射 %s，等待容器获取 IP ...\n", domain)
	ip := waitForIP(client, name, 15)
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

// CmdUninstall 交互式选择并删除容器。
func CmdUninstall() error {
	name, err := selectContainer("选择要删除的容器")
	if err != nil {
		return err
	}
	if name == "" {
		return nil
	}
	client := NewIncusClient()
	fmt.Printf("停止容器 %s ...\n", name)
	_ = client.Stop(name)
	fmt.Printf("删除容器 %s ...\n", name)
	if err := client.Delete(name); err != nil {
		return err
	}
	fmt.Printf("✔ 容器 %s 已删除\n", name)
	return nil
}

// CmdStop 停止容器。
func CmdStop(name string) error {
	fmt.Printf("停止容器 %s ...\n", name)
	return NewIncusClient().Stop(name)
}

// CmdIn 进入容器（交互式透传 stdio）。
func CmdIn(name string) error {
	return NewIncusClient().Exec(name)
}

// CmdExport 导出容器为 ./容器名.tar.gz。
func CmdExport(name string) error {
	path := fmt.Sprintf("./%s.tar.gz", name)
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
	if name != "" {
		if err := client.Import(path, name); err != nil {
			return err
		}
	} else {
		if err := client.run("import", path); err != nil {
			return err
		}
	}
	fmt.Printf("✔ 导入完成\n")
	return nil
}
