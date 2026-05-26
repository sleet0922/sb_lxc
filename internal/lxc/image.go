package lxc

import (
	"go.uber.org/zap"
	"sb_lxc/internal/core"
)

type ImageService struct {
	exec core.Executor
}

func NewImageService(exec core.Executor) *ImageService {
	return &ImageService{exec: exec}
}

func (s *ImageService) ListDownloadImages() (string, error) {
	core.Log.Info("listing downloadable images via lxc-download template")
	return s.exec.Run("/usr/share/lxc/templates/lxc-download", "-l")
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
