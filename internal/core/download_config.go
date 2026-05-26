package core

const DownloadConfig = `# LXC Download Configuration
# 镜像源配置 - 从官方镜像源下载 LXC 镜像

# 默认镜像服务器（官方）
DOWNLOAD_SERVER="images.linuxcontainers.org"

# 可选：国内镜像源（如果官方源慢或无法访问）
# 清华大学镜像源：
# DOWNLOAD_SERVER="mirrors.tuna.tsinghua.edu.cn/lxc-images"
# 
# 中科大镜像源：
# DOWNLOAD_SERVER="mirrors.ustc.edu.cn/lxc-images"

# 默认架构
DOWNLOAD_ARCH="amd64"

# 默认模式
DOWNLOAD_MODE="system"

# 缓存目录
DOWNLOAD_CACHE_DIR="/var/cache/lxc/download"
`