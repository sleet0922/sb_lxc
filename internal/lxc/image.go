package lxc

import (
	"bufio"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	"sb_lxc/internal/core"
)

type ImageInfo struct {
	Distro     string
	Release    string
	Arch       string
	Build      string
	UpdateTime string
}

type ImageService struct {
	exec core.Executor
}

func NewImageService(exec core.Executor) *ImageService {
	return &ImageService{exec: exec}
}

func (s *ImageService) ListDownloadImages() (string, error) {
	core.Log.Info("listing downloadable images via lxc-download template")
	return s.exec.Run("/usr/share/lxc/templates/lxc-download", "--list")
}

func (s *ImageService) ParseImages(output string) []ImageInfo {
	var images []ImageInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Using") || strings.HasPrefix(line, "DIST") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			images = append(images, ImageInfo{
				Distro:     fields[0],
				Release:    fields[1],
				Arch:       fields[2],
				Build:      fields[3],
				UpdateTime: fields[4],
			})
		}
	}
	return images
}

func (s *ImageService) GetUniqueDistros(images []ImageInfo) []string {
	distroSet := make(map[string]bool)
	for _, img := range images {
		distroSet[img.Distro] = true
	}
	distros := make([]string, 0, len(distroSet))
	for d := range distroSet {
		distros = append(distros, d)
	}
	sort.Strings(distros)
	return distros
}

func (s *ImageService) GetReleasesForDistro(images []ImageInfo, distro string) []string {
	releaseSet := make(map[string]bool)
	for _, img := range images {
		if img.Distro == distro {
			releaseSet[img.Release] = true
		}
	}
	releases := make([]string, 0, len(releaseSet))
	for r := range releaseSet {
		releases = append(releases, r)
	}
	sort.Strings(releases)
	return releases
}

func (s *ImageService) GetArchesForRelease(images []ImageInfo, distro, release string) []string {
	archSet := make(map[string]bool)
	for _, img := range images {
		if img.Distro == distro && img.Release == release {
			archSet[img.Arch] = true
		}
	}
	arches := make([]string, 0, len(archSet))
	for a := range archSet {
		arches = append(arches, a)
	}
	sort.Strings(arches)
	return arches
}

func promptChoice(items []string, label string, allowEmpty bool) (string, error) {
	reader := bufio.NewReader(core.GetStdin())
	for {
		fmt.Printf("\n请选择%s (输入序号或名称): ", label)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			if allowEmpty {
				return "", nil
			}
			fmt.Println("输入不能为空，请重新选择")
			continue
		}

		var n int
		if _, err := fmt.Sscanf(input, "%d", &n); err == nil && n > 0 && n <= len(items) {
			return items[n-1], nil
		}

		for _, item := range items {
			if strings.EqualFold(item, input) {
				return item, nil
			}
		}

		fmt.Printf("无效输入，请重新选择 (1-%d 或名称)\n", len(items))
	}
}

func (s *ImageService) SelectImageInteractive() (distro, version, arch string, err error) {
	core.Log.Info("fetching available images...")
	fmt.Println("正在获取可用镜像列表...")
	output, err := s.ListDownloadImages()
	if err != nil {
		return "", "", "", fmt.Errorf("获取镜像列表失败: %w", err)
	}

	images := s.ParseImages(output)
	if len(images) == 0 {
		return "", "", "", fmt.Errorf("未找到可用镜像")
	}

	distros := s.GetUniqueDistros(images)

	fmt.Println("\n=== 可用镜像列表 ===")
	fmt.Printf("共找到 %d 个镜像，%d 个发行版\n", len(images), len(distros))

	for i, d := range distros {
		fmt.Printf("  %2d. %s\n", i+1, d)
	}

	selectedDistro, err := promptChoice(distros, "发行版", false)
	if err != nil {
		return "", "", "", err
	}

	releases := s.GetReleasesForDistro(images, selectedDistro)
	fmt.Printf("\n=== %s 可用版本 ===\n", selectedDistro)
	for i, r := range releases {
		fmt.Printf("  %2d. %s\n", i+1, r)
	}

	selectedRelease, err := promptChoice(releases, "版本", false)
	if err != nil {
		return "", "", "", err
	}

	arches := s.GetArchesForRelease(images, selectedDistro, selectedRelease)
	fmt.Printf("\n=== %s %s 可用架构 ===\n", selectedDistro, selectedRelease)
	for i, a := range arches {
		fmt.Printf("  %2d. %s\n", i+1, a)
	}

	selectedArch, err := promptChoice(arches, "架构 (默认 amd64)", true)
	if err != nil {
		return "", "", "", err
	}
	if selectedArch == "" {
		selectedArch = "amd64"
	}

	fmt.Printf("\n已选择: %s / %s / %s\n", selectedDistro, selectedRelease, selectedArch)
	return selectedDistro, selectedRelease, selectedArch, nil
}

func (s *ImageService) CreateFromDownload(name, distro, version, arch string) (string, error) {
	core.Log.Info("creating container from download template",
		zap.String("name", name),
		zap.String("distro", distro),
		zap.String("version", version),
		zap.String("arch", arch),
	)
	return s.exec.Run("lxc-create", "-n", name, "-t", "download", "--",
		"-d", distro, "-r", version, "-a", arch)
}
