package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// CmdInstall 两级菜单：先选发行版，再选具体版本，最后安装。
func CmdInstall() error {
	fmt.Println("正在从镜像源获取可用发行版列表 ...")
	client := NewIncusClient()
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
	r := bufio.NewReader(os.Stdin)
	defaultName := defaultNameFromImage(version.Image)
	name := prompt(r, fmt.Sprintf("容器名称 (回车默认 %s): ", defaultName))
	if name == "" {
		name = defaultName
	}

	imageRef := MirrorRemote + ":" + version.Image
	fmt.Printf("\n正在安装 %s %s (%s) ...\n", group.Distro, version.Release, imageRef)
	if err := client.Launch(imageRef, name); err != nil {
		return err
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
