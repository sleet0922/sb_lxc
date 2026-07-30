package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeIncusfile 写入临时 Incusfile 并返回其路径。
func writeIncusfile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写入测试 Incusfile 失败: %v", err)
	}
	return p
}

func TestTempBlockBasic(t *testing.T) {
	content := `FROM debian/13
NAME my-app
RUN apt-get install -y ca-certificates

TEMP builder
  RUN apt-get install -y golang-go
  WORKDIR /src
  COPY ./main.go .
  RUN go build -o app .
END

COPY --from=builder /src/app /usr/local/bin/app
EXPOSE 8080/tcp
AUTOSTART on
`
	p := writeIncusfile(t, "Incusfile", content)
	f, err := parseIncusfile(p)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 期望 2 个阶段: [builder, main]
	if len(f.Stages) != 2 {
		t.Fatalf("期望 2 个阶段 (builder + main), 实际 %d", len(f.Stages))
	}
	if f.Stages[0].Name != "builder" {
		t.Errorf("阶段0 名字期望 builder, 实际 %q", f.Stages[0].Name)
	}
	if f.Stages[0].From != "debian/13" {
		t.Errorf("阶段0 From 期望继承 debian/13, 实际 %q", f.Stages[0].From)
	}
	if len(f.Stages[0].Steps) != 4 {
		t.Errorf("阶段0 步骤数期望 4 (RUN/WORKDIR/COPY/RUN), 实际 %d", len(f.Stages[0].Steps))
	}
	// 主阶段 (最终阶段) 无 AS 名字
	if f.Stages[1].Name != "" {
		t.Errorf("主阶段名字期望空, 实际 %q", f.Stages[1].Name)
	}
	// 主阶段步骤: RUN apt + COPY --from=builder
	if len(f.Stages[1].Steps) != 2 {
		t.Errorf("主阶段步骤数期望 2, 实际 %d", len(f.Stages[1].Steps))
	}
	// 第二个步骤是 COPY --from=builder
	copyStep := f.Stages[1].Steps[1]
	if copyStep.Kind != "COPY" || copyStep.Copy.From != "builder" {
		t.Errorf("主阶段第二个步骤期望 COPY --from=builder, 实际 %+v", copyStep)
	}
	// 顶层字段同步自最终阶段
	if f.Name != "my-app" {
		t.Errorf("f.Name 期望 my-app, 实际 %q", f.Name)
	}
	if len(f.Exposes) != 1 || f.Exposes[0].Port != 8080 {
		t.Errorf("f.Exposes 期望 [8080], 实际 %+v", f.Exposes)
	}
	if f.Autostart == nil || !*f.Autostart {
		t.Errorf("f.Autostart 期望 on, 实际 %v", f.Autostart)
	}
}

func TestTempBlockUnclosed(t *testing.T) {
	content := `FROM debian/13
TEMP builder
  RUN echo hi
`
	p := writeIncusfile(t, "Incusfile", content)
	_, err := parseIncusfile(p)
	if err == nil {
		t.Fatal("未关闭的 TEMP 块应报错")
	}
}

func TestTempBlockNested(t *testing.T) {
	content := `FROM debian/13
TEMP a
  TEMP b
  END
END
`
	p := writeIncusfile(t, "Incusfile", content)
	_, err := parseIncusfile(p)
	if err == nil {
		t.Fatal("嵌套 TEMP 应报错")
	}
}

func TestTempBlockMultipleFromRejected(t *testing.T) {
	content := `FROM debian/13 AS x
RUN echo a
FROM debian/13
TEMP t
  RUN echo b
END
`
	p := writeIncusfile(t, "Incusfile", content)
	_, err := parseIncusfile(p)
	if err == nil {
		t.Fatal("TEMP 块与多 FROM 混用应报错")
	}
}

func TestTempBlockMultipleTemps(t *testing.T) {
	content := `FROM debian/13
TEMP a
  RUN echo a
END
TEMP b
  RUN echo b
END
RUN echo final
`
	p := writeIncusfile(t, "Incusfile", content)
	f, err := parseIncusfile(p)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// 期望 3 个阶段: [a, b, main]
	if len(f.Stages) != 3 {
		t.Fatalf("期望 3 个阶段, 实际 %d", len(f.Stages))
	}
	wantNames := []string{"a", "b", ""}
	gotNames := []string{f.Stages[0].Name, f.Stages[1].Name, f.Stages[2].Name}
	if !reflect.DeepEqual(wantNames, gotNames) {
		t.Errorf("阶段名顺序期望 %v, 实际 %v", wantNames, gotNames)
	}
}

func TestEndWithoutTemp(t *testing.T) {
	content := `FROM debian/13
RUN echo hi
END
`
	p := writeIncusfile(t, "Incusfile", content)
	_, err := parseIncusfile(p)
	if err == nil {
		t.Fatal("无 TEMP 的 END 应报错")
	}
}
