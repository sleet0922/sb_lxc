package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// CmdList 列出已安装容器，解析 JSON 后以表格展示，比原生命令更丰富。
func CmdList() error {
	client := NewIncusClient()
	cs, err := client.ListContainers()
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		fmt.Println("暂无容器。使用 sb_lxc install 安装一个。")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tIPV4\tAUTOSTART")
	for i := range cs {
		c := &cs[i]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.Name,
			strings.ToLower(c.Status),
			c.IPv4(),
			autostartBadge(c.Autostart()),
		)
	}
	return w.Flush()
}

func autostartBadge(v string) string {
	switch v {
	case "true":
		return "on"
	case "false":
		return "off"
	default:
		return "-"
	}
}

// trunc 截断过长字符串。
func trunc(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
