package lxc

import "sb_lxc/internal/core"

type SnapshotService struct {
	exec core.Executor
}

func NewSnapshotService(exec core.Executor) *SnapshotService {
	return &SnapshotService{exec: exec}
}

func (s *SnapshotService) Create(name string) (string, error) {
	return s.exec.Run("lxc-snapshot", "-n", name)
}

func (s *SnapshotService) List(name string) (string, error) {
	return s.exec.Run("lxc-snapshot", "-n", name, "-L")
}

func (s *SnapshotService) Restore(name, snapshot string) (string, error) {
	return s.exec.Run("lxc-snapshot", "-n", name, "-r", snapshot)
}

func (s *SnapshotService) Delete(name, snapshot string) (string, error) {
	return s.exec.Run("lxc-snapshot", "-n", name, "-d", snapshot)
}
