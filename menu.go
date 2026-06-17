package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// selectMenu 交互式上下键选择菜单，返回选中索引；-1 表示取消。
// 支持：↑↓/jk 选择、Enter 确认、q/Ctrl+C 退出、数字键直选。
func selectMenu(options []string, prompt string) int {
	fd := int(os.Stdin.Fd())
	old, err := makeRaw(fd)
	if err != nil {
		// 非 TTY 或失败时退化为序号输入
		return fallbackSelect(options, prompt)
	}
	defer restoreTerm(fd, old)

	m := &menu{out: os.Stdout}
	m.render(options, 0, prompt)

	reader := bufio.NewReader(os.Stdin)
	selected := 0
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return -1
		}
		switch b {
		case 0x1b: // ESC 序列
			b2, err := reader.ReadByte()
			if err != nil {
				return -1
			}
			if b2 == '[' {
				b3, _ := reader.ReadByte()
				switch b3 {
				case 'A': // ↑
					if selected > 0 {
						selected--
						m.render(options, selected, prompt)
					}
				case 'B': // ↓
					if selected < len(options)-1 {
						selected++
						m.render(options, selected, prompt)
					}
				}
			}
		case '\r', '\n': // Enter 确认
			fmt.Fprintln(os.Stdout)
			return selected
		case 3, 'q', 'Q': // Ctrl+C / q 退出
			fmt.Fprintln(os.Stdout)
			return -1
		case 'k', 'w': // vim 风格上移
			if selected > 0 {
				selected--
				m.render(options, selected, prompt)
			}
		case 'j', 's': // vim 风格下移
			if selected < len(options)-1 {
				selected++
				m.render(options, selected, prompt)
			}
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			idx := int(b - '1')
			if idx < len(options) {
				fmt.Fprintln(os.Stdout)
				return idx
			}
		}
	}
}

// menu 跟踪上次渲染行数以实现原地重绘。
type menu struct {
	lastLines int
	out       *os.File
}

func (m *menu) render(options []string, selected int, prompt string) {
	// 上移到菜单起始处，并清除该行及以下所有内容
	if m.lastLines > 0 {
		fmt.Fprintf(m.out, "\r\033[%dA\033[J", m.lastLines)
	}
	var sb strings.Builder
	sb.WriteString(prompt)
	sb.WriteString("\r\n")
	for i, opt := range options {
		if i == selected {
			sb.WriteString(fmt.Sprintf("  \033[36m❯\033[0m \033[1m%s\033[0m", opt))
		} else {
			sb.WriteString(fmt.Sprintf("    %s", opt))
		}
		sb.WriteString("\033[K\r\n")
	}
	fmt.Fprint(m.out, sb.String())
	m.lastLines = len(options) + 1
}

// fallbackSelect 非 TTY 环境下的序号选择。
func fallbackSelect(options []string, prompt string) int {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("请输入序号: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	var n int
	fmt.Sscanf(line, "%d", &n)
	if n >= 1 && n <= len(options) {
		return n - 1
	}
	return -1
}

// prompt 从标准输入读取一行（去空白）。
func prompt(r *bufio.Reader, label string) string {
	fmt.Print(label)
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}

// ──────────────────── termios 原始模式（零依赖，纯 syscall） ────────────────────

func makeRaw(fd int) (*syscall.Termios, error) {
	var old syscall.Termios
	if err := tcgetattr(fd, &old); err != nil {
		return nil, err
	}
	raw := old
	// 只关回显和规范模式，保留 OPOST 使 \n 正常转 \r\n
	raw.Iflag &^= syscall.IXON | syscall.BRKINT
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := tcsetattr(fd, &raw); err != nil {
		return nil, err
	}
	return &old, nil
}

func restoreTerm(fd int, state *syscall.Termios) {
	_ = tcsetattr(fd, state)
}

func tcgetattr(fd int, t *syscall.Termios) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(t)))
	if e != 0 {
		return e
	}
	return nil
}

func tcsetattr(fd int, t *syscall.Termios) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(syscall.TCSETS), uintptr(unsafe.Pointer(t)))
	if e != 0 {
		return e
	}
	return nil
}
