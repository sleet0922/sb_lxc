package lxc

import "sb_lxc/internal/core"

type NetworkService struct {
	exec core.Executor
}

func NewNetworkService(exec core.Executor) *NetworkService {
	return &NetworkService{exec: exec}
}