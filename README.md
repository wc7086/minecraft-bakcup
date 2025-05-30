# Minecraft 备份工具 (Go 版本)

这是一个用 Go 语言编写的 Minecraft 服务器备份工具，使用 Restic 进行增量备份到 Cloudflare R2 存储。

## 功能特性

- 自动暂停 Minecraft 世界写入以确保数据一致性
- 使用 Restic 进行增量备份，节省存储空间
- 支持备份到 Cloudflare R2 (S3 兼容存储)
- 自动清理旧快照，支持灵活的保留策略
- 完整的错误处理和日志记录
- 使用 TOML 配置文件统一管理所有配置（包括密码）

## 系统要求

- Go 1.21 或更高版本
- Docker (用于访问 Minecraft 容器)
- Restic (备份工具)
- 运行中的 Minecraft Docker 容器

## 安装

### 快速安装

```bash
# 1. 检查系统依赖
make check-deps

# 2. 编译程序
make build

# 3. 安装到用户目录（推荐，不需要 sudo）
make install-user

# 或者安装到系统目录（需要 sudo）
make install
```

### 手动编译

```bash
# 编译为当前平台的可执行文件
go build -o minecraft-backup minecraft-backup.go

# 或者直接运行
go run minecraft-backup.go
```

### 设置 PATH

如果使用 `make install-user` 安装，程序会被安装到 `~/.local/bin`。你需要确保这个目录在 PATH 中：

1. **对于 zsh 用户（macOS 默认）**：

   ```bash
   echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
   source ~/.zshrc
   ```

2. **对于 bash 用户**：

   ```bash
   echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
   source ~/.bashrc
   ```

3. **验证 PATH 设置**：

   ```bash
   echo $PATH | grep -q "$HOME/.local/bin" && echo "✓ PATH 已设置" || echo "✗ PATH 未设置"
   ```

如果使用 `make install` 安装到系统目录，通常 `/usr/local/bin` 已经在 PATH 中。

## 配置

### 配置文件位置

程序使用 TOML 格式的配置文件，默认位置：

- 普通用户: `~/.config/minecraft-backup/config.toml`
- Root 用户: `/etc/minecraft-backup/config.toml`
- 自定义位置: 设置环境变量 `MINECRAFT_BACKUP_CONFIG`

### 配置文件格式

首次运行时，程序会自动创建示例配置文件：

```toml
# Minecraft 备份工具配置文件

[general]
# Minecraft Docker 容器名称
container_name = "minecraft-mc-1"

# 世界文件目录（主机路径）
world_dir = "~/docker/minecraft"

# 备份标签（用于标识备份）
backup_tag = "minecraft-test"

# 主机标识（默认为主机名）
backup_host = "your-hostname"

[aws]
# AWS/R2 访问密钥 ID
access_key_id = "your_access_key_here"

# AWS/R2 访问密钥
secret_access_key = "your_secret_key_here"

# AWS 区域（R2 使用 "auto"）
region = "auto"

[restic]
# Restic 仓库地址
# 格式: s3:https://[account-id].r2.cloudflarestorage.com/[bucket-name]
repository = "s3:https://your-account-id.r2.cloudflarestorage.com/minecraft-backup"

# Restic 仓库密码（用于加密备份）
password = "your_restic_repository_password_here"

[retention]
# 快照保留策略
keep_daily = 9      # 保留每日快照数量
keep_weekly = 14    # 保留每周快照数量
keep_monthly = 8    # 保留每月快照数量
keep_last = 12      # 保留最近快照数量
```

### 安全注意事项

配置文件包含敏感信息（AWS 凭证和 Restic 密码），请确保：

1. 文件权限设置为 `600`：`chmod 600 config.toml`
2. 不要将配置文件提交到版本控制系统
3. 定期备份配置文件到安全位置

## 使用方法

1. 确保 Minecraft 服务器正在 Docker 容器中运行
2. 安装 Restic: `sudo apt install restic` 或 `brew install restic`
3. 编译并安装程序: `make build && make install-user`
4. 首次运行创建配置文件: `minecraft-backup`
5. 编辑配置文件 `config.toml` 填入正确信息
6. 再次运行进行备份: `minecraft-backup`

### 使用自定义配置文件

```bash
# 使用环境变量指定配置文件
export MINECRAFT_BACKUP_CONFIG=/path/to/your/config.toml
minecraft-backup

# 或者一次性使用
MINECRAFT_BACKUP_CONFIG=/path/to/your/config.toml minecraft-backup
```

## 定时备份

可以使用 cron 设置定时备份：

```bash
# 编辑 crontab
crontab -e

# 每天凌晨 3 点执行备份
0 3 * * * /home/user/.local/bin/minecraft-backup >> /var/log/minecraft-backup.log 2>&1

# 或者使用完整路径（如果 PATH 未设置）
0 3 * * * $HOME/.local/bin/minecraft-backup >> /var/log/minecraft-backup.log 2>&1
```

## 恢复备份

使用 Restic 恢复备份：

```bash
# 设置必要的环境变量（从配置文件中获取）
export AWS_ACCESS_KEY_ID="your_access_key"
export AWS_SECRET_ACCESS_KEY="your_secret_key"
export RESTIC_REPOSITORY="s3:https://..."
export RESTIC_PASSWORD="your_password"

# 列出所有快照
restic snapshots

# 恢复特定快照
restic restore <snapshot-id> --target /path/to/restore

# 挂载快照浏览
restic mount /mnt/restic
```

## 故障排除

1. **命令未找到**: 确保程序在 PATH 中，或使用完整路径
2. **Docker 服务未运行**: 启动 Docker 服务 `sudo systemctl start docker`
3. **容器未找到**: 检查容器名称是否正确 `docker ps`
4. **权限问题**: 确保配置文件权限为 600
5. **网络问题**: 检查网络连接和 R2 凭证
6. **仓库锁定**: 程序会自动尝试解锁，或手动执行 `restic unlock`
7. **配置文件错误**: 检查 TOML 语法是否正确

## 从旧版本迁移

如果你之前使用的是分离的 credentials 和 password 文件，可以手动将内容迁移到新的 TOML 配置文件中。

## 卸载

```bash
# 卸载程序
make uninstall

# 删除配置文件（可选）
rm -rf ~/.config/minecraft-backup
```

## 许可证

MIT License
