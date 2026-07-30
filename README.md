# sb_lxc

> 基于 Incus 的轻量级容器编排工具，提供类似 Docker 的命令体验。内置 macvlan 网络、端口映射、域名解析、Dockerfile 风格的镜像构建，一键编译跑通容器。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Incus](https://img.shields.io/badge/Incus-v6-00B4D8)](https://linuxcontainers.org/incus/)
[![License](https://img.shields.io/badge/license-MIT-green)](#-license)
[![Platform](https://img.shields.io/badge/platform-Linux%20amd64%20%7C%20arm64-blue)](#-安装)

---

## 目录

- [特性](#-特性)
- [快速开始](#-快速开始)
- [安装](#-安装)
- [命令参考](#-命令参考)
- [镜像构建 (Incusfile)](#-镜像构建-incusfile)
- [网络架构](#-网络架构)
- [环境变量](#-环境变量)
- [示例](#-示例)
- [与原生 incus CLI 对比](#-与原生-incus-cli-对比)
- [License](#-license)

---

## ✨ 特性

- **容器生命周期**：install / uninstall / start / stop / in / list，交互式菜单可选容器
- **macvlan 网络自动化**：自动识别父网卡、自动分配 shim IP、自动维护 `/32` 路由让宿主机与容器互通、ARP 隔离避免地址冲突
- **端口映射**：通过 Incus proxy 设备实现 TCP/UDP 端口映射（宿主机端口→容器端口），支持 DHCP IP 自动刷新
- **域名映射**：容器域名 → IP 自动写入宿主机 `/etc/hosts`，容器重启后自动更新
- **开机自启动**：一条命令配置 `boot.autostart`
- **备份与恢复**：`export` 导出容器为 tar.gz，`import` 一键还原
- **Dockerfile 风格镜像构建**：`Incusfile` 支持 `FROM / RUN / COPY / ENV / EXPOSE / DOMAIN / AUTOSTART` 指令，`sb_lxc build` 构建镜像，`sb_lxc run` 启动容器
- **自动清理**：启动时自动删除未被引用的 Incus 托管网桥（如默认 `incusbr0`），释放 53 端口和网段
- **跨平台编译**：纯 Go 实现，单二进制文件，Linux amd64 / arm64 一键交叉编译

---

## 🚀 快速开始

```bash
# 1. 安装二进制（已编译好 dist/sb_lxc）
sudo cp dist/sb_lxc /usr/local/bin/sb_lxc
sudo chmod +x /usr/local/bin/sb_lxc

# 2. 查看帮助
sb_lxc help

# 3. 安装一个 Alpine 容器（交互式选单）
sb_lxc install

# 4. 列出容器
sb_lxc list

# 5. 进入容器
sb_lxc in alpine-3-21

# 6. 构建镜像并启动（Dockerfile 体验）
cd my-project/
sb_lxc build      # 构建镜像
sb_lxc run        # 启动容器
```

---

## 📦 安装

### 前置要求

- **Incus v6** 已安装并初始化（`incus admin init`）
- 宿主机有可用的物理网卡（用于 macvlan）
- Linux amd64 或 arm64

### 从源码编译

```bash
git clone https://github.com/<your-user>/sb_lxc.git
cd sb_lxc

# 编译为当前平台的二进制
make build

# 交叉编译 Linux amd64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w" -o dist/sb_lxc .

# 交叉编译 Linux arm64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags "-s -w" -o dist/sb_lxc .

# 部署
sudo cp dist/sb_lxc /usr/local/bin/
sudo chmod +x /usr/local/bin/sb_lxc
```

### 构建 .deb 包

```bash
make deb    # 产物在 dist/*.deb
sudo dpkg -i dist/*.deb
```

---

## 📖 命令参考

运行 `sb_lxc help` 查看：

```
sb_lxc - Incus 容器管理工具 v1.3.0

容器管理:
  sb_lxc list                 | 列出已安装容器
  sb_lxc install              | 安装新容器 (交互式选择发行版)
  sb_lxc uninstall            | 删除容器
  sb_lxc start   [容器名]     | 启动容器
  sb_lxc stop    [容器名]     | 停止容器
  sb_lxc in      [容器名]     | 进入容器 shell
  sb_lxc export  [容器名]     | 导出容器为 tar.gz
  sb_lxc import  [文件] [名]  | 从 tar.gz 导入容器

容器设置 (sb_lxc set <容器名> ...):
  sb_lxc set <容器名> port [规格]      | 端口映射 (规格如 8080:80/tcp)
  sb_lxc set <容器名> port rm <规格>   | 取消端口映射
  sb_lxc set <容器名> port list        | 查看端口映射
  sb_lxc set <容器名> domain <域名>    | 域名映射 (写入 /etc/hosts)
  sb_lxc set <容器名> autostart [on|off] | 开机自启动

镜像构建 (类似 Dockerfile):
  sb_lxc build [Incusfile]               | 构建镜像 (默认 ./Incusfile)
  sb_lxc build --name <名> [Incusfile]   | 覆盖镜像别名
  sb_lxc run [容器名]                    | 从 ./Incusfile 读取镜像名并启动容器

  Incusfile 指令:
    FROM <镜像>   NAME <名称>     RUN <命令>
    COPY <源> <目标>   ENV <K=V>
    EXPOSE <端口>   DOMAIN <域名>   AUTOSTART on|off
    TEMP <名称> ... END   临时构建块 (隔离编译工具链)

其他:
  sb_lxc help                 | 显示此帮助

提示: [容器名] 省略时进入交互式选择菜单
```

> 💡 凡是带 `[容器名]` 的命令，省略参数时会弹出交互式选择菜单（↑↓ 选择，Enter 确认，q 退出）。

### 详细说明

#### 容器管理

| 命令 | 说明 |
|------|------|
| `sb_lxc list` | 列出所有容器，显示名称/状态/IPv4/自启动/端口映射 |
| `sb_lxc install [镜像] [名]` | 安装新容器；无参数则两级菜单（选发行版→选版本）；带参数时 `镜像` 可为 `debian/12` 或 `debian:12` |
| `sb_lxc uninstall` | 交互式选择并删除容器（含其快照） |
| `sb_lxc start [容器名]` | 启动容器，自动配置 macvlan NIC 并刷新宿主机路由 |
| `sb_lxc stop [容器名]` | 优雅停止容器（30s 超时后强制） |
| `sb_lxc in [容器名]` | 进入容器 shell（优先 `/bin/bash`，回退 `/bin/sh`），保留 TTY 尺寸 |
| `sb_lxc export [容器名]` | 导出容器为 tar.gz 备份文件（gzip 压缩） |
| `sb_lxc import <文件> [新名]` | 从 tar.gz 还原容器；无参数则交互式选择本地 tar.gz |

#### 容器设置 (`sb_lxc set`)

所有设置都幂等，可重复执行。

```bash
# 端口映射：规格格式 [宿主机端口:]容器端口/协议
sb_lxc set web port 8080:80/tcp     # 宿主机 8080 → 容器 80
sb_lxc set web port 53/udp          # 宿主机 53 → 容器 53 (UDP)
sb_lxc set web port 8080:80/tcp     # 再次执行 → 替换原有映射 (幂等)

sb_lxc set web port list            # 查看所有映射
sb_lxc set web port rm 8080:80/tcp   # 删除指定映射

# 域名映射：自动写入宿主机 /etc/hosts
sb_lxc set web domain web.test      # web.test → 容器 IP

# 开机自启动
sb_lxc set web autostart on
sb_lxc set web autostart off
```

**端口映射规格说明**

| 规格 | 宿主端口 | 容器端口 | 协议 |
|------|----------|----------|------|
| `8080:80/tcp` | 8080 | 80 | TCP |
| `53/udp` | 53 | 53 | UDP |
| `8080/tcp` | 8080 | 8080 | TCP |
| `8080:80` | 8080 | 80 | TCP (默认) |

合法端口范围 `1-65535`，非法格式（空串、`abc`、`0`、`65536`、`80:99999`、`80/abc`、`-1`）会被拒绝。

#### 镜像构建

详见下方 [Incusfile](#-镜像构建-incusfile) 章节。

---

## 🐳 镜像构建 (Incusfile)

`Incusfile` 是 Dockerfile 风格的 Incus 镜像构建描述文件。`sb_lxc build` 读取该文件，启动一个临时构建容器，按顺序执行指令，最后将容器发布为本地 Incus 镜像，可选地直接启动正式容器。

### 指令一览

| 指令 | 说明 | 示例 |
|------|------|------|
| `FROM <镜像>` | 基础镜像（兼容 `debian/12` 与 `debian:12`） | `FROM debian/12` |
| `NAME <名称>` | 镜像别名 + 容器名 | `NAME my-nginx` |
| `RUN <命令>` | 在容器内执行 shell 命令（`/bin/sh -c`） | `RUN apt-get update && apt-get install -y nginx` |
| `COPY <源> <目标>` | 从宿主机复制文件/目录到容器（递归） | `COPY ./index.html /var/www/html/` |
| `ENV <K>=<V>` | 设置环境变量（写入 `/etc/environment` + `profile.d`） | `ENV APP_PORT=8080` |
| `EXPOSE <端口>[/<协议>] ...` | 声明端口映射（运行时自动创建），空格分隔多个 | `EXPOSE 80/tcp 53/udp` |
| `DOMAIN <域名>` | 域名映射（运行时写入 `/etc/hosts`） | `DOMAIN nginx.test` |
| `AUTOSTART on\|off` | 开机自启动 | `AUTOSTART on` |
| `TEMP <名称> ... END` | 临时构建块（块内步骤在独立临时容器执行，不进最终镜像，用于隔离编译工具链） | 见下方示例 |

### 指令执行顺序

`RUN` / `COPY` / `ENV` **严格按 Incusfile 中的出现顺序执行**（不是分组批量执行）。这意味着你可以：

```dockerfile
RUN mkdir -p /app/config          # 先创建目录
COPY ./config.yaml /app/config/   # 再 COPY 到该目录
```

`ENV` 设置的变量在后续 `RUN` 中即可使用，同时会持久化到镜像的 `/etc/environment` 和 `/etc/profile.d/sb_lxc-env.sh`。

### TEMP 临时构建块

`TEMP <名称> ... END` 定义一个临时构建块：块内的 `RUN`/`COPY`/`WORKDIR`/`ENV` 在一个**独立临时容器**中执行，**不进入最终镜像**。适用于隔离编译工具链（golang/nodejs 等）对运行时镜像的污染。

- 块继承外层 `FROM` 镜像（单 FROM all-in-one 模式，不支持多 FROM 混用）
- 块名可用于 `COPY --from=<块名>` 将构建产物拷回最终镜像
- 多个 `TEMP` 块按出现顺序执行，均早于主阶段的步骤
- 块必须用 `END` 关闭，不支持嵌套

```dockerfile
FROM debian/13
NAME my-app
RUN apt-get update && apt-get install -y ca-certificates mysql-server

TEMP builder
  RUN apt-get update && apt-get install -y golang-go
  WORKDIR /src
  COPY ./main.go .
  RUN go build -o app .
END

COPY --from=builder /src/app /usr/local/bin/app
EXPOSE 8080/tcp
AUTOSTART on
```

上例中 `golang-go` 只装在 `builder` 临时容器里，最终镜像只有 `ca-certificates` + `mysql-server` + 编译好的 `app` 二进制。

### 构建流程

```
sb_lxc build
   │
   ├─ [1/4] 启动构建容器 (临时, 镜像=FROM)
   ├─ [2/4] 等待网络就绪
   ├─ [3/4] 按 Incusfile 顺序执行 Steps (RUN/COPY/ENV)
   ├─ [4/4] 停止构建容器 → 发布为本地镜像 (别名=NAME)
   └─ 启动正式容器并应用 EXPOSE / DOMAIN / AUTOSTART
```

构建是**幂等**的：重新构建同名镜像时，会先删除旧别名，并清理无其他别名引用的孤儿镜像。

### 命令

```bash
# 构建镜像（默认读取 ./Incusfile）
sb_lxc build

# 覆盖镜像别名
sb_lxc build --name my-app

# 指定 Incusfile 路径
sb_lxc build path/to/MyIncusfile

# 从 ./Incusfile 读取镜像名并启动容器（EXPOSE/DOMAIN/AUTOSTART 取自 Incusfile）
sb_lxc run

# 启动时覆盖容器名
sb_lxc run my-container
```

### 完整示例：一键构建 Nginx 站点

**项目结构**
```
my-site/
├── Incusfile
└── index.html
```

**Incusfile**
```dockerfile
FROM debian/12
NAME my-nginx
RUN apt-get update && apt-get install -y nginx
RUN echo "daemon off;" >> /etc/nginx/nginx.conf
COPY ./index.html /var/www/html/index.html
ENV NGINX_HOST=nginx.test
EXPOSE 80/tcp
DOMAIN nginx.test
AUTOSTART on
```

**index.html**
```html
<h1>Hello from sb_lxc build!</h1>
```

**构建镜像并启动**
```bash
cd my-site/
sb_lxc build      # 构建镜像
sb_lxc run        # 从 ./Incusfile 读取镜像名并启动容器
```

输出：
```
╭─ Incusfile 构建
│ 文件:     Incusfile
│ 基础镜像: debian/12
│ 目标镜像: my-nginx
│ ...
╰─

✔ 镜像已发布: my-nginx

▶ 从镜像 my-nginx 启动容器 my-nginx
✔ 域名映射: nginx.test -> 192.168.10.110
✔ 端口映射: 80/tcp
```

随后即可在宿主机访问 `http://nginx.test`（已写入 `/etc/hosts`）或 `curl 192.168.10.110:80`。

---

## 🌐 网络架构

sb_lxc 采用 **macvlan** 网络（而非 Incus 默认 bridge），让容器直接获得与宿主机同一网段的 IP，性能接近裸机。

### 自动化流程

1. **识别父网卡**：依次尝试 `SB_LXC_MACVLAN_PARENT` 环境变量 → 默认 IPv4 路由出口网卡 → `ens18` 兜底
2. **配置 default profile**：将 `eth0` 设为 macvlan NIC，parent 指向物理网卡
3. **容器启动时分配 MAC**：每个容器获得稳定的随机 MAC
4. **宿主机互通 shim**：在宿主机创建 `sb-lxc-mv` macvlan 子接口，配置 `/32` 路由让宿主机能访问容器
5. **ARP 隔离**：调整 `arp_ignore=1` / `arp_announce=2`，避免宿主机物理网卡和 shim 互相替对方应答 ARP
6. **shim IP 自动选择**：从网段末尾向前扫描未占用 IP（带 ping 探测），避免与网关/宿主机/已用 IP 冲突

### 与默认 bridge 网络的对比

| 特性 | Incus bridge (默认) | sb_lxc macvlan |
|------|---------------------|----------------|
| 容器 IP | NAT 内网 10.x.x.x | 与宿主机同网段 |
| 性能 | 有 NAT 开销 | 接近线速 |
| 端口映射 | 必需（外部访问） | 仅外部访问需要，宿主机/局域网可直接访问容器 IP |
| 宿主机↔容器 | 直接互通 | 需要 shim 路由（sb_lxc 自动配置） |
| DHCP/DNS | incusbr0 提供 | 由路由器/外部 DHCP 提供 |

### 端口映射机制

由于 macvlan 容器不通过 Incus bridge，sb_lxc 使用 Incus **proxy 设备**（非 NAT 模式）实现端口映射：

- 设备名格式：`port-<宿主端口>-<协议>`（幂等）
- 监听 `0.0.0.0:<宿主端口>`，转发到 `tcp:<容器IP>:<容器端口>`
- 容器重启后 IP 可能变化，sb_lxc 启动时自动刷新所有映射的 connect 地址

---

## 🔧 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `SB_LXC_MACVLAN_PARENT` | 自动识别 | 指定 macvlan 父网卡名（如 `eth0`、`ens18`） |
| `SB_LXC_HOST_SHIM_CIDR` | 自动选择 | 指定宿主机 shim 接口的 CIDR（如 `192.168.10.254/32`） |
| `SB_LXC_INCUS_URL` | 本地 Unix socket | 连接远程 Incus 的 URL（如 `https://192.168.1.10:8443`） |
| `SB_LXC_INCUS_INSECURE` | `false` | 设为 `true` 跳过 TLS 证书校验 |
| `SB_LXC_INCUS_SERVER_CERT` | - | 远程 Incus 服务端证书文件路径 |
| `SB_LXC_INCUS_CLIENT_CERT` | - | 客户端证书文件路径 |
| `SB_LXC_INCUS_CLIENT_KEY` | - | 客户端私钥文件路径 |
| `SB_LXC_INCUS_CA` | - | CA 证书文件路径 |

---

## 📚 示例

### 示例 1：快速安装并使用 Alpine

```bash
# 交互式安装
sb_lxc install
# → 选择 Alpine → 选择 3.21 → 回车使用默认名

# 进入容器
sb_lxc in alpine-3-21

# 暴露端口
sb_lxc set alpine-3-21 port 8080/tcp

# 设置域名
sb_lxc set alpine-3-21 domain alpine.test

# 开机自启
sb_lxc set alpine-3-21 autostart on

# 查看状态
sb_lxc list
```

### 示例 2：构建带 ENV 的 Python 应用

```dockerfile
FROM ubuntu/24.04
NAME py-app
RUN apt-get update && apt-get install -y python3 python3-pip
RUN pip3 install --break-system-packages flask
RUN mkdir -p /app
COPY ./app.py /app/app.py
ENV FLASK_APP=/app/app.py
ENV FLASK_ENV=development
EXPOSE 5000/tcp
DOMAIN py.test
AUTOSTART on
```

```bash
sb_lxc build
# 容器启动后，进入容器运行：
sb_lxc in py-app
# 在容器内：
flask run --host=0.0.0.0
```

### 示例 3：备份与迁移

```bash
# 导出
sb_lxc export web
# → 生成 web_20260728_153000.tar.gz

# 在另一台机器导入
sb_lxc import web_20260728_153000.tar.gz web-restored
```

### 示例 4：连接远程 Incus

```bash
export SB_LXC_INCUS_URL=https://192.168.1.10:8443
export SB_LXC_INCUS_INSECURE=true
sb_lxc list
```

---

## 🆚 与原生 incus CLI 对比

| 场景 | 原生 incus CLI | sb_lxc |
|------|----------------|--------|
| 安装容器 | `incus launch images:debian/12 web` | `sb_lxc install` (交互式菜单) |
| 进入容器 | `incus exec web -- bash` | `sb_lxc in web` |
| 端口映射 | 手动 `incus config device add web port proxy ...` (一长串参数) | `sb_lxc set web port 8080:80/tcp` |
| 域名映射 | 手动编辑 `/etc/hosts` | `sb_lxc set web domain web.test` |
| 开机自启 | `incus config set web boot.autostart true` | `sb_lxc set web autostart on` |
| 镜像构建 | 不支持 (需手动执行命令) | `sb_lxc build` (Dockerfile 风格) |
| 宿主机↔容器互通 | 需手动配置 macvlan shim | 自动完成 |
| 网桥清理 | 手动 `incus network delete incusbr0` | 启动时自动清理 |

---

## 📄 License

MIT License. 详见 [LICENSE](LICENSE)。
