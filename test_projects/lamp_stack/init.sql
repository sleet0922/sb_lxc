-- MariaDB 初始化脚本
-- 在容器构建时执行，创建应用数据库和用户

CREATE DATABASE IF NOT EXISTS appdb CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'appuser'@'localhost' IDENTIFIED BY 'apppass';
CREATE USER IF NOT EXISTS 'appuser'@'127.0.0.1' IDENTIFIED BY 'apppass';
GRANT ALL PRIVILEGES ON appdb.* TO 'appuser'@'localhost';
GRANT ALL PRIVILEGES ON appdb.* TO 'appuser'@'127.0.0.1';
FLUSH PRIVILEGES;
