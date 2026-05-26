package core

import (
	"os"
	"os/exec"
)

type Executor interface {
	Run(name string, args ...string) (string, error)
}

type ShellExecutor struct{}

func (e *ShellExecutor) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func GetStdin() *os.File {
	return os.Stdin
}