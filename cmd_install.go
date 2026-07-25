package main

import (
	"fmt"
	"strings"
)

// CmdInstall 两级菜单：先选发行版，再选具体版本，最后安装。
// 若传入参数：sb_lxc install [镜像名/引用] [容器名] 则跳过菜单直接安装。
func CmdInstall(args []string) error {
	client := NewIncusClient()

	if len(args) >= 1 {
		imageRef := args[0]
		if !strings.Contains(imageRef, ":") {
			imageRef = MirrorRemote + ":" + imageRef
		}
		name := ""
		if len(args) >= 2 {
			name = args[1]
		} else {
			name = defaultNameFromImage(args[0])
		}
		fmt.Printf("正在安装 %s (名称: %s) ...\n", imageRef, name)
		if err := client.Launch(imageRef, name); err != nil {
			return err
		}
		ip := waitForIP(client, name, 15)
		if ip != "" {
			warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))
		}
		fmt.Printf("✔ 容器 %s 已安装并启动!\n", name)
		return nil
	}

	fmt.Println("正在从镜像源获取可用发行版列表 ...")
	groups, err := client.ListImages()
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return fmt.Errorf("未找到可用镜像")
	}

	// 一级菜单：发行版
	distroNames := make([]string, len(groups))
	for i, g := range groups {
		distroNames[i] = fmt.Sprintf("%s (%d)", g.Distro, len(g.Versions))
	}
	dChoice := selectMenu(distroNames, "选择发行版 (↑↓ 选择, Enter 确认, q 退出)")
	if dChoice < 0 {
		return nil
	}
	group := groups[dChoice]

	// 二级菜单：具体版本
	relNames := make([]string, len(group.Versions))
	for i, v := range group.Versions {
		relNames[i] = v.Release
	}
	vChoice := selectMenu(relNames, fmt.Sprintf("%s - 选择版本 (↑↓ 选择, Enter 确认, q 退出)", group.Distro))
	if vChoice < 0 {
		return nil
	}
	version := group.Versions[vChoice]

	// 容器名
	defaultName := defaultNameFromImage(version.Image)
	name := prompt(fmt.Sprintf("容器名称 (回车默认 %s): ", defaultName))
	if name == "" {
		name = defaultName
	}

	imageRef := MirrorRemote + ":" + version.Image
	fmt.Printf("\n正在安装 %s %s (%s) ...\n", group.Distro, version.Release, imageRef)
	if err := client.Launch(imageRef, name); err != nil {
		return err
	}

	ip := waitForIP(client, name, 15)
	if ip != "" {
		warnAutoHostMacvlan(AutoConfigureHostMacvlan(client))
	}

	fmt.Printf("✔ 容器 %s 已安装并启动!\n", name)
	return nil
}

// defaultNameFromImage 由镜像引用生成合法容器名。
// debian/bookworm -> debian-bookworm
func defaultNameFromImage(image string) string {
	s := strings.ReplaceAll(image, "/", "-")
	return strings.ToLower(s)
}
