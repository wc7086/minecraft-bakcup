# Minecraft 多服务器备份工具使用说明

## 概述

这个更新版本的 Minecraft 备份工具支持同时备份多个 Minecraft 服务器，支持顺序备份和并行备份两种模式。

## 主要功能

- **多服务器支持**: 可以在一个配置文件中定义多个 Minecraft 服务器
- **并行备份**: 可以同时备份多个服务器，提高效率
- **选择性备份**: 可以选择启用或禁用特定服务器的备份
- **统一管理**: 所有服务器共享相同的 AWS/R2 凭证和 Restic 仓库
- **灵活配置**: 每个服务器可以有不同的容器名称、世界目录和备份标签

## 配置文件结构

### 全局配置 `[global]`

```toml
[global]
# 默认备份主机标识（当服务器未设置时使用）
default_backup_host = "my-minecraft-server"

# 是否启用并行备份（同时备份多个服务器）
parallel_backup = false

# 最大并发数（仅在启用并行备份时有效）
max_concurrency = 2
```

### AWS/R2 配置 `[aws]`

```toml
[aws]
# AWS/R2 访问密钥 ID
access_key_id = "your_access_key_here"

# AWS/R2 访问密钥
secret_access_key = "your_secret_key_here"

# AWS 区域（R2 使用 "auto"）
region = "auto"
```

### Restic 配置 `[restic]`

```toml
[restic]
# Restic 仓库地址
repository = "s3:https://your-account-id.r2.cloudflarestorage.com/minecraft-backup"

# Restic 仓库密码（用于加密备份）
password = "your_restic_repository_password_here"
```

### 保留策略 `[retention]`

```toml
[retention]
# 快照保留策略（适用于所有服务器）
keep_daily = 9      # 保留每日快照数量
keep_weekly = 14    # 保留每周快照数量
keep_monthly = 8    # 保留每月快照数量
keep_last = 12      # 保留最近快照数量
```

### 服务器配置 `[servers.服务器名称]`

```toml
[servers.survival]
# 服务器描述
description = "生存服务器"

# Minecraft Docker 容器名称
container_name = "minecraft-survival"

# 世界文件目录（主机路径）
world_dir = "~/docker/minecraft-survival"

# 备份标签（用于标识备份）
backup_tag = "minecraft-survival"

# 主机标识（可选，未设置时使用全局默认值）
backup_host = "my-server"

# 是否启用此服务器的备份
enabled = true
```

## 使用方法

### 1. 初始化配置

首次运行程序时，会自动创建示例配置文件：

```bash
./minecraft-backup-multi
```

这将在以下位置创建配置文件：

- Root 用户: `/etc/minecraft-backup/config.toml`
- 普通用户: `~/.config/minecraft-backup/config.toml`

### 2. 配置服务器

编辑配置文件，添加你的服务器：

```toml
# 示例：添加一个新的服务器
[servers.skyblock]
description = "空岛服务器"
container_name = "minecraft-skyblock"
world_dir = "~/docker/minecraft-skyblock"
backup_tag = "minecraft-skyblock"
backup_host = "my-server"
enabled = true
```

### 3. 启用/禁用服务器

通过设置 `enabled` 参数来控制是否备份特定服务器：

```toml
[servers.testserver]
# ... 其他配置 ...
enabled = false  # 不备份此服务器
```

### 4. 配置备份模式

#### 顺序备份（默认）

```toml
[global]
parallel_backup = false
```

#### 并行备份

```toml
[global]
parallel_backup = true
max_concurrency = 3  # 最多同时备份3个服务器
```

### 5. 运行备份

```bash
./minecraft-backup-multi
```

## 输出示例

### 顺序备份输出

```log
[2024-01-01 10:00:00] 多服务器备份配置：
[2024-01-01 10:00:00]   配置文件: /home/user/.config/minecraft-backup/config.toml
[2024-01-01 10:00:00]   并行备份: false
[2024-01-01 10:00:00]   保留策略:
[2024-01-01 10:00:00]     - 每日快照: 9 个
[2024-01-01 10:00:00]     - 每周快照: 14 个
[2024-01-01 10:00:00]     - 每月快照: 8 个
[2024-01-01 10:00:00]     - 最近快照: 12 个
[2024-01-01 10:00:00]   仓库地址: s3:https://account-id.r2.cloudflarestorage.com/...
[2024-01-01 10:00:00] 
[2024-01-01 10:00:00] 启用的服务器列表：
[2024-01-01 10:00:00]   [survival]
[2024-01-01 10:00:00]     容器名称: minecraft-survival
[2024-01-01 10:00:00]     世界目录: /home/user/docker/minecraft-survival
[2024-01-01 10:00:00]     备份标签: minecraft-survival
[2024-01-01 10:00:00]     主机标识: my-server
[2024-01-01 10:00:00] 
[2024-01-01 10:00:00]   [modded]
[2024-01-01 10:00:00]     容器名称: minecraft-modded
[2024-01-01 10:00:00]     世界目录: /home/user/docker/minecraft-modded
[2024-01-01 10:00:00]     备份标签: minecraft-modded
[2024-01-01 10:00:00]     主机标识: my-server
[2024-01-01 10:00:00] 
==================================================
[2024-01-01 10:00:05] 开始备份服务器: survival
[2024-01-01 10:00:05] [survival] 暂停 Minecraft 世界写入...
[2024-01-01 10:00:10] [survival] 服务器备份完成
==================================================

==================================================
[2024-01-01 10:02:05] 开始备份服务器: modded
[2024-01-01 10:02:05] [modded] 暂停 Minecraft 世界写入...
[2024-01-01 10:04:10] [modded] 服务器备份完成
==================================================

[2024-01-01 10:04:15] 备份结果摘要:
[2024-01-01 10:04:15]   成功: 2 个服务器
[2024-01-01 10:04:15]   失败: 0 个服务器
```

### 并行备份输出

```log
[2024-01-01 10:00:00] 启用并行备份，最大并发数: 2
[2024-01-01 10:00:00] [并行] 开始备份服务器: survival
[2024-01-01 10:00:00] [并行] 开始备份服务器: modded
[2024-01-01 10:02:05] [并行] 服务器 survival 备份成功
[2024-01-01 10:02:10] [并行] 服务器 modded 备份成功
[2024-01-01 10:02:15] 并行备份结果摘要:
[2024-01-01 10:02:15]   成功: 2 个服务器
[2024-01-01 10:02:15]   失败: 0 个服务器
```

## 注意事项

1. **容器名称**: 确保每个服务器的 `container_name` 对应实际运行的 Docker 容器
2. **世界目录**: 每个服务器的 `world_dir` 必须是正确的主机路径
3. **备份标签**: 使用不同的 `backup_tag` 来区分不同服务器的备份
4. **并行备份**: 启用并行备份时注意系统资源，避免设置过高的并发数
5. **错误处理**: 即使部分服务器备份失败，程序仍会继续备份其他服务器

## 故障排除

### 常见错误

1. **容器未运行**

   ```log
   错误: 容器 minecraft-survival 未运行
   ```

   解决：确保 Docker 容器正在运行

2. **配置文件错误**

   ```log
   错误: 没有启用的服务器
   ```

   解决：检查配置文件中是否有 `enabled = true` 的服务器

3. **权限问题**

   ```log
   警告: config.toml 权限不安全 (644)，建议设置为 600
   ```

   解决：执行 `chmod 600 ~/.config/minecraft-backup/config.toml`

### 调试方法

1. 检查配置文件语法：

   ```bash
   # 可以使用任何 TOML 验证工具
   ```

2. 测试单个服务器：

   ```bash
   # 在配置文件中只启用一个服务器进行测试
   ```

3. 查看 Docker 日志：

   ```bash
   docker logs minecraft-survival
   ```

## 从单服务器配置迁移

如果你之前使用的是单服务器版本的备份工具，可以按以下步骤迁移：

1. 备份原有配置文件
2. 运行新版本程序生成新的配置文件模板
3. 将原有配置转换为新的格式：

```toml
# 原有配置
[general]
container_name = "minecraft-mc-1"
world_dir = "~/docker/minecraft"
backup_tag = "minecraft-world"
backup_host = "my-server"

# 转换为新格式
[global]
default_backup_host = "my-server"
parallel_backup = false

[servers.main]
container_name = "minecraft-mc-1"
world_dir = "~/docker/minecraft"
backup_tag = "minecraft-world"
backup_host = "my-server"
enabled = true
```

## 性能建议

1. **顺序备份**: 更稳定，适合资源有限的环境
2. **并行备份**: 更快速，但会消耗更多系统资源
3. **并发数设置**: 建议不超过 CPU 核心数的一半
4. **网络带宽**: 并行备份会增加网络使用量

## 定时任务设置

可以使用 cron 设置定时备份：

```bash
# 每天凌晨2点执行备份
0 2 * * * /path/to/minecraft-backup-multi >> /var/log/minecraft-backup.log 2>&1
```
