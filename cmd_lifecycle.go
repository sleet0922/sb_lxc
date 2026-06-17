package main

import "fmt"

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

// CmdImport 从备份文件导入容器，args: [文件路径] [新容器名]
func CmdImport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("用法: sb_lxc import <文件路径> [新容器名]")
	}
	path := args[0]
	name := ""
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
