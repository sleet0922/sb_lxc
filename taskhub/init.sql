-- TaskHub 数据库初始化
CREATE DATABASE IF NOT EXISTS taskhub CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'taskhub'@'localhost' IDENTIFIED BY 'taskhub123';
GRANT ALL PRIVILEGES ON taskhub.* TO 'taskhub'@'localhost';
FLUSH PRIVILEGES;
