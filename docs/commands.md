# sb_lxc 命令参考

## 概述

`s`b_lxc` 是一个 LXC 容器管理 CLI 工具，提供简单的命令行接口管理 LXC 容器。

## 全局用法

```bash
sb_lxc [命令] [参数] [flags]
```

---

## 命令列表

### 1. `list` — 列出所有容器

显示所有 LXC 容器的详细信息，包括名称、状态、IP 地址等。

**用法：**

```bash
sb_lxc list
```

**示例：**

```bash
sb_lxc list
```

---

### 2. `install` — 安装/创建容器

从可用系统镜像创建一个新的 LXC 容器。如果不指定参数，会进入交互式选择模式。

**用法：**

```bash
sb_lxc install [容器名] [flags]
```

**Flags：**

| Flag | 简写 | 说明 |
|------|------|------|
| `--distro` | `-d` | 发行版（如 ubuntu, debian, alpine） |
| `--version` | `-v` | 版本（如 jammy, bookworm, latest） |
| `--arch` | `-a` | 架构（如 amd64, arm64） |

**示例：**

```bash
# 交互式选择镜像创建
sb_lxc install mycontainer

# 指定参数创建
sb_lxc install mycontainer -d ubuntu -v jammy -a amd64
```

---

### 3. `start` — 启动容器

启动一个已创建的 LXC 容器，默认后台运行。

**用法：**

```bash
sb_lxc start [容器名]
```

**示例：**

```bash
sb_lxc start mycontainer
```

---

### 4. `stop` — 关停容器

关停一个正在运行的 LXC 容器。

**用法：**

```bash
sb_lxc stop [容器名]
```

**示例：**

```bash
sb_lxc stop mycontainer
```

---

### 5. `in` — 进入容器

使用 `lxc-attach` 进入指定的 LXC 容器。进入后如同在容器内操作，退出容器后自动返回宿主机。

**用法：**

```bash
sb_lxc in [容器名]
```

**示例：**

```bash
sb_lxc in mycontainer
```

---

### 6. `enable` — 启用容器开机自启

设置指定容器在系统启动时自动启动。

**用法：**

```bash
sb_lxc enable [容器名]
```

**示例：**

```bash
sb_lxc enable mycontainer
```

---

### 7. `disable` — 禁用容器开机自启

取消指定容器的开机自动启动设置。

**用法：**

```bash
sb_lxc disable [容器名]
```

**示例：**

```bash
sb_lxc disable mycontainer
```

---

### 8. `port` — 端口映射

将容器的端口映射到宿主机端口。通过网络地址转换（NAT）实现端口转发。

**用法：**

```bash
sb_lxc port [容器名] [容器端口] [宿主机端口]
```

**示例：**

```bash
# 将容器的80端口映射到宿主机的8080端口
sb_lxc port mycontainer 80 8080
```

> 注意：需要重启容器使端口映射配置生效。

---

### 9. `status` — 查看容器配置状态

检测容器的各项配置是否已生效，包括开机自启和端口映射。会读取容器配置文件并检测相关设置，同时检查 iptables 规则是否已加载。

**用法：**

```bash
sb_lxc status [容器名]
```

**示例：**

```bash
sb_lxc status mycontainer
```

**输出示例：**

```
容器: mycontainer

【开机自启】
  状态: 已启用 ✔

【端口映射】
  宿主机 8080 -> 容器 80

  iptables 规则生效状态:
  端口 8080: 规则未生效（需重启容器） ✘
```

---

### 10. `uninstall` — 删除容器

永久删除指定的 LXC 容器及其所有数据。默认需要确认操作。

**用法：**

```bash
sb_lxc uninstall [容器名] [flags]
```

**Flags：**

| Flag | 简写 | 说明 |
|------|------|------|
| `--force` | `-f` | 强制删除，不提示确认 |

**示例：**

```bash
# 删除容器（需确认）
sb_lxc uninstall mycontainer

# 强制删除容器（无需确认）
sb_lxc uninstall mycontainer --force
```

---

## 全局配置

配置文件位于 `~/.sb_lxc/config.yaml`，支持以下默认配置：

```yaml
default:
  distro: ubuntu
  release: jammy
  arch: amd64
```

## 调试模式

设置环境变量 `SB_LXC_DEBUG=1` 可启用调试日志输出：

```bash
SB_LXC_DEBUG=1 sb_lxc list
```