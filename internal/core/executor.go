package core

import (
	"bytes"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

type Executor interface {
	Run(name string, args ...string) (string, error)
}

type ShellExecutor struct{}

func (e *ShellExecutor) Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		// 失败时合并 stderr 用于错误诊断
		result := stdout.String()
		if stderr.Len() > 0 {
			if result != "" {
				result += "\n"
			}
			result += stderr.String()
		}
		return result, err
	}

	// 成功时只返回 stdout，stderr 的 LXC 版本警告等不混入输出
	if stderr.Len() > 0 && Log != nil {
		Log.Debug("cmd stderr", zap.String("cmd", name), zap.String("stderr", stderr.String()))
	}

	return stdout.String(), nil
}

func GetStdin() *os.File {
	return os.Stdin
}