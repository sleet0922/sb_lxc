package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// addMount 交互式添加宿主机文件夹到容器的双向挂载（disk 设备 + shift=true）。
// 在 CmdSet 菜单中调用，容器已选定，仅需输入宿主机路径与容器内路径。
func addMount(client *IncusClient, ct *Container) error {
	r := bufio.NewReader(os.Stdin)
	hostPath := prompt(r, "宿主机路径: ")
	containerPath := prompt(r, "容器内路径: ")

	if hostPath == "" {
		return fmt.Errorf("宿主机路径不能为空")
	}
	if containerPath == "" {
		return fmt.Errorf("容器内路径不能为空")
	}

	// 宿主机路径转绝对路径
	if !filepath.IsAbs(hostPath) {
		abs, err := filepath.Abs(hostPath)
		if err != nil {
			return fmt.Errorf("无法解析宿主机路径: %w", err)
		}
		hostPath = abs
	}
	// 容器内路径必须为绝对路径
	if !strings.HasPrefix(containerPath, "/") {
		return fmt.Errorf("容器内路径必须为绝对路径: %s", containerPath)
	}
	// 校验宿主机路径存在且为目录
	info, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("宿主机路径不存在: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("宿主机路径不是目录: %s", hostPath)
	}

	// 宿主机路径为系统关键目录时警告并要求二次确认：
	// 将 /root、/etc 等挂入容器会让容器内对应路径被宿主机目录覆盖，
	// 容器配置文件/家目录被穿透，极易造成混乱（如容器内命令缺失、配置错乱）。
	if isSystemPath(hostPath) {
		fmt.Printf("⚠ 宿主机路径 %s 是系统关键目录，挂载后容器内对应路径将被宿主机目录覆盖，可能造成混乱。\n", hostPath)
		confirm := prompt(r, "确认继续? [y/N]: ")
		if !strings.EqualFold(confirm, "y") {
			fmt.Println("已取消。")
			return nil
		}
	}

	// 已存在相同挂载则直接提示
	for _, d := range ct.MountDevices() {
		if d["source"] == hostPath && d["path"] == containerPath {
			fmt.Printf("ℹ 该挂载已存在: %s:%s  <->  %s\n", ct.Name, containerPath, hostPath)
			return nil
		}
	}

	device := uniqueMountDeviceName(ct, containerPath)
	if err := client.AddDiskDevice(ct.Name, device, hostPath, containerPath); err != nil {
		return err
	}
	fmt.Printf("✔ 已挂载 (双向): %s:%s  <->  %s  (设备 %s)\n", ct.Name, containerPath, hostPath, device)
	return nil
}

// removeMount 列出已有挂载设备供选择移除。
func removeMount(client *IncusClient, ct *Container) error {
	devs := ct.MountDevices()
	if len(devs) == 0 {
		fmt.Println("该容器没有挂载设备。")
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
		labels[i] = fmt.Sprintf("%s  (%s -> %s)", n, d["source"], d["path"])
	}

	choice := selectMenu(labels, "选择要移除的挂载 (↑↓ 选择, Enter 确认, q 退出)")
	if choice < 0 {
		return nil
	}
	target := names[choice]
	if err := client.RemoveDevice(ct.Name, target); err != nil {
		return err
	}
	fmt.Printf("✔ 挂载 %s 已移除\n", target)
	return nil
}

// deviceNameForMount 由容器内路径生成设备名前缀（去除首尾 /，内部 / 转为 -）。
func deviceNameForMount(containerPath string) string {
	sanitized := strings.ReplaceAll(strings.Trim(containerPath, "/"), "/", "-")
	if sanitized == "" {
		sanitized = "root-target"
	}
	return "mount-" + sanitized
}

// systemPaths 宿主机系统关键目录，挂载这些目录到容器会造成穿透覆盖，需警告。
var systemPaths = map[string]bool{
	"/": true, "/root": true, "/etc": true, "/var": true, "/usr": true,
	"/boot": true, "/bin": true, "/sbin": true, "/lib": true, "/lib64": true,
	"/proc": true, "/sys": true, "/dev": true, "/home": true,
}

// isSystemPath 判断路径是否为系统关键目录。
func isSystemPath(p string) bool {
	return systemPaths[filepath.Clean(p)]
}

// uniqueMountDeviceName 在容器现有设备名中取一个不冲突的挂载设备名。
func uniqueMountDeviceName(ct *Container, containerPath string) string {
	base := deviceNameForMount(containerPath)
	existing := ct.ExpandedDevices
	if existing == nil {
		existing = ct.Devices
	}
	name := base
	for i := 2; ; i++ {
		if _, ok := existing[name]; !ok {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}
