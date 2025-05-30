package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// TOMLConfig 是 TOML 配置文件的结构
type TOMLConfig struct {
	// 全局配置
	Global GlobalConfig `toml:"global"`
	
	// AWS/R2 凭证
	AWS AWSConfig `toml:"aws"`
	
	// Restic 配置
	Restic ResticConfig `toml:"restic"`
	
	// 备份策略
	Retention RetentionConfig `toml:"retention"`
	
	// 服务器列表
	Servers map[string]ServerConfig `toml:"servers"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	// 默认备份主机标识
	DefaultBackupHost string `toml:"default_backup_host"`
	// 是否并行备份
	ParallelBackup bool `toml:"parallel_backup"`
	// 最大并发数
	MaxConcurrency int `toml:"max_concurrency"`
}

// ServerConfig 单个服务器配置
type ServerConfig struct {
	ContainerName string `toml:"container_name"`
	WorldDir      string `toml:"world_dir"`
	BackupTag     string `toml:"backup_tag"`
	BackupHost    string `toml:"backup_host"`
	Enabled       bool   `toml:"enabled"`
	Description   string `toml:"description"`
}

// AWSConfig AWS/R2 凭证配置
type AWSConfig struct {
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Region          string `toml:"region"`
}

// ResticConfig Restic 相关配置
type ResticConfig struct {
	Repository string `toml:"repository"`
	Password   string `toml:"password"`
}

// RetentionConfig 快照保留策略
type RetentionConfig struct {
	KeepDaily   int `toml:"keep_daily"`
	KeepWeekly  int `toml:"keep_weekly"`
	KeepMonthly int `toml:"keep_monthly"`
	KeepLast    int `toml:"keep_last"`
}

// Config 运行时配置（单个服务器）
type Config struct {
	// Minecraft 容器配置
	MCContainer string
	WorldDir    string

	// Restic 仓库配置
	ResticRepository string
	ResticPassword   string
	AWSRegion        string

	// 备份标签配置
	BackupTag  string
	BackupHost string

	// 快照保留策略
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepLast    int

	// 配置文件路径
	ConfigFile string

	// AWS 凭证
	AWSAccessKeyID     string
	AWSSecretAccessKey string
}

// MultiServerConfig 多服务器运行时配置
type MultiServerConfig struct {
	// 配置文件路径
	ConfigFile string
	
	// 全局配置
	ParallelBackup bool
	MaxConcurrency int
	
	// AWS 凭证和 Restic 配置（所有服务器共享）
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSRegion          string
	ResticRepository   string
	ResticPassword     string
	
	// 快照保留策略（所有服务器共享）
	KeepDaily   int
	KeepWeekly  int
	KeepMonthly int
	KeepLast    int
	
	// 服务器列表
	Servers map[string]*Config
}

// Logger 结构体
type Logger struct {
	prefix string
}

// NewLogger 创建新的日志记录器
func NewLogger() *Logger {
	return &Logger{}
}

// Log 记录日志
func (l *Logger) Log(format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", timestamp, message)
}

// 全局日志实例
var logger = NewLogger()

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	// 优先使用环境变量指定的路径
	if configPath := os.Getenv("MINECRAFT_BACKUP_CONFIG"); configPath != "" {
		return configPath
	}

	// 根据用户权限选择默认路径
	if os.Geteuid() == 0 {
		// root 用户
		return "/etc/minecraft-backup/config.toml"
	}

	// 普通用户
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".config", "minecraft-backup", "config.toml")
}

// createSampleConfig 创建示例配置文件
func createSampleConfig(configPath string) error {
	logger.Log("创建示例配置文件...")

	// 确保目录存在
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// 获取默认值
	hostname, _ := os.Hostname()
	homeDir, _ := os.UserHomeDir()

	// 创建示例配置
	sampleConfig := `# Minecraft 多服务器备份工具配置文件

[global]
# 默认备份主机标识（当服务器未设置时使用）
default_backup_host = "%s"

# 是否启用并行备份（同时备份多个服务器）
parallel_backup = false

# 最大并发数（仅在启用并行备份时有效）
max_concurrency = 2

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
# 快照保留策略（适用于所有服务器）
keep_daily = 9      # 保留每日快照数量
keep_weekly = 14    # 保留每周快照数量
keep_monthly = 8    # 保留每月快照数量
keep_last = 12      # 保留最近快照数量

# 服务器配置
# 每个服务器一个配置块，格式为 [servers.服务器名称]

[servers.survival]
# 服务器描述
description = "生存服务器"

# Minecraft Docker 容器名称
container_name = "minecraft-survival"

# 世界文件目录（主机路径）
world_dir = "%s/docker/minecraft-survival"

# 备份标签（用于标识备份）
backup_tag = "minecraft-survival"

# 主机标识（可选，未设置时使用全局默认值）
backup_host = "%s"

# 是否启用此服务器的备份
enabled = true

[servers.creative]
# 服务器描述
description = "创造服务器"

# Minecraft Docker 容器名称
container_name = "minecraft-creative"

# 世界文件目录（主机路径）
world_dir = "%s/docker/minecraft-creative"

# 备份标签（用于标识备份）
backup_tag = "minecraft-creative"

# 主机标识（可选，未设置时使用全局默认值）
backup_host = "%s"

# 是否启用此服务器的备份
enabled = false

[servers.modded]
# 服务器描述
description = "模组服务器"

# Minecraft Docker 容器名称
container_name = "minecraft-modded"

# 世界文件目录（主机路径）
world_dir = "%s/docker/minecraft-modded"

# 备份标签（用于标识备份）
backup_tag = "minecraft-modded"

# 主机标识（可选，未设置时使用全局默认值）
backup_host = "%s"

# 是否启用此服务器的备份
enabled = true
`

	// 格式化配置内容
	configContent := fmt.Sprintf(sampleConfig, hostname, homeDir, hostname, homeDir, hostname, homeDir, hostname, hostname)

	// 写入文件，设置安全权限
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return err
	}

	logger.Log("已创建示例配置文件: %s", configPath)
	logger.Log("")
	logger.Log("请编辑配置文件并填入正确的信息：")
	logger.Log("  1. 修改 [aws] 部分，填入您的 R2 凭证")
	logger.Log("  2. 修改 [restic] 部分，设置仓库地址和密码")
	logger.Log("  3. 修改 [servers] 部分，配置您的 Minecraft 服务器")
	logger.Log("     - 启用需要备份的服务器（enabled = true）")
	logger.Log("     - 设置正确的容器名称和世界目录")
	logger.Log("  4. 根据需要调整其他配置")
	logger.Log("  5. 重新运行程序")

	return nil
}

// loadConfig 加载配置文件并解析为多服务器配置
func loadConfig(configPath string) (*MultiServerConfig, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 检查文件权限
	info, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}

	// 检查是否是常规文件
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s 不是常规文件", configPath)
	}

	// 检查权限（应该是 600）
	if info.Mode().Perm() != 0600 {
		logger.Log("警告: %s 权限不安全 (%o)，建议设置为 600", configPath, info.Mode().Perm())
		logger.Log("执行: chmod 600 %s", configPath)
	}

	// 读取 TOML 配置
	var tomlConfig TOMLConfig
	if _, err := toml.DecodeFile(configPath, &tomlConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 获取默认主机名
	defaultHost := tomlConfig.Global.DefaultBackupHost
	if defaultHost == "" {
		defaultHost, _ = os.Hostname()
	}

	// 设置默认值
	if tomlConfig.Global.MaxConcurrency <= 0 {
		tomlConfig.Global.MaxConcurrency = 2
	}

	// 转换为多服务器运行时配置
	multiConfig := &MultiServerConfig{
		ConfigFile:         configPath,
		ParallelBackup:     tomlConfig.Global.ParallelBackup,
		MaxConcurrency:     tomlConfig.Global.MaxConcurrency,
		AWSAccessKeyID:     tomlConfig.AWS.AccessKeyID,
		AWSSecretAccessKey: tomlConfig.AWS.SecretAccessKey,
		AWSRegion:          tomlConfig.AWS.Region,
		ResticRepository:   tomlConfig.Restic.Repository,
		ResticPassword:     tomlConfig.Restic.Password,
		KeepDaily:          tomlConfig.Retention.KeepDaily,
		KeepWeekly:         tomlConfig.Retention.KeepWeekly,
		KeepMonthly:        tomlConfig.Retention.KeepMonthly,
		KeepLast:           tomlConfig.Retention.KeepLast,
		Servers:            make(map[string]*Config),
	}

	// 检查是否有服务器配置
	if len(tomlConfig.Servers) == 0 {
		return nil, fmt.Errorf("配置文件中未找到任何服务器配置")
	}

	// 转换服务器配置
	for serverName, serverConfig := range tomlConfig.Servers {
		// 跳过未启用的服务器
		if !serverConfig.Enabled {
			logger.Log("跳过未启用的服务器: %s", serverName)
			continue
		}

		// 设置备份主机（如果未设置则使用默认值）
		backupHost := serverConfig.BackupHost
		if backupHost == "" {
			backupHost = defaultHost
		}

		// 展开环境变量（如果路径中包含 ~）
		worldDir := serverConfig.WorldDir
		if strings.HasPrefix(worldDir, "~/") {
			homeDir, _ := os.UserHomeDir()
			worldDir = filepath.Join(homeDir, worldDir[2:])
		}

		config := &Config{
			ConfigFile:         configPath,
			MCContainer:        serverConfig.ContainerName,
			WorldDir:           worldDir,
			BackupTag:          serverConfig.BackupTag,
			BackupHost:         backupHost,
			AWSAccessKeyID:     multiConfig.AWSAccessKeyID,
			AWSSecretAccessKey: multiConfig.AWSSecretAccessKey,
			AWSRegion:          multiConfig.AWSRegion,
			ResticRepository:   multiConfig.ResticRepository,
			ResticPassword:     multiConfig.ResticPassword,
			KeepDaily:          multiConfig.KeepDaily,
			KeepWeekly:         multiConfig.KeepWeekly,
			KeepMonthly:        multiConfig.KeepMonthly,
			KeepLast:           multiConfig.KeepLast,
		}

		multiConfig.Servers[serverName] = config
	}

	// 检查是否有启用的服务器
	if len(multiConfig.Servers) == 0 {
		return nil, fmt.Errorf("没有启用的服务器，请在配置文件中将需要备份的服务器设置为 enabled = true")
	}

	// 设置环境变量（所有服务器共享）
	os.Setenv("AWS_ACCESS_KEY_ID", multiConfig.AWSAccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", multiConfig.AWSSecretAccessKey)
	os.Setenv("AWS_DEFAULT_REGION", multiConfig.AWSRegion)
	os.Setenv("RESTIC_REPOSITORY", multiConfig.ResticRepository)
	os.Setenv("RESTIC_PASSWORD", multiConfig.ResticPassword)

	return multiConfig, nil
}

// checkDependencies 检查系统依赖
func checkDependencies() error {
	// 检查 Docker 服务
	if runtime.GOOS == "linux" {
		cmd := exec.Command("systemctl", "is-active", "--quiet", "docker")
		if err := cmd.Run(); err != nil {
			logger.Log("错误: Docker 服务未运行")
			logger.Log("请启动Docker服务: sudo systemctl start docker")
			return fmt.Errorf("Docker service not running")
		}
	}

	// 检查必要命令
	commands := []string{"docker", "restic"}
	var missingDeps []string

	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			missingDeps = append(missingDeps, cmd)
		}
	}

	if len(missingDeps) > 0 {
		logger.Log("错误: 缺少必要命令: %s", strings.Join(missingDeps, ", "))
		return fmt.Errorf("missing dependencies: %v", missingDeps)
	}

	return nil
}

// checkNetwork 检查网络连接
func checkNetwork() {
	logger.Log("检查网络连接...")
	cmd := exec.Command("ping", "-c", "1", "-W", "5", "1.1.1.1")
	if err := cmd.Run(); err != nil {
		logger.Log("警告: 网络连接可能有问题")
	}
}

// showConfig 显示当前配置
func showConfig(multiConfig *MultiServerConfig) {
	logger.Log("多服务器备份配置：")
	logger.Log("  配置文件: %s", multiConfig.ConfigFile)
	logger.Log("  并行备份: %t", multiConfig.ParallelBackup)
	if multiConfig.ParallelBackup {
		logger.Log("  最大并发: %d", multiConfig.MaxConcurrency)
	}
	logger.Log("  保留策略:")
	logger.Log("    - 每日快照: %d 个", multiConfig.KeepDaily)
	logger.Log("    - 每周快照: %d 个", multiConfig.KeepWeekly)
	logger.Log("    - 每月快照: %d 个", multiConfig.KeepMonthly)
	logger.Log("    - 最近快照: %d 个", multiConfig.KeepLast)
	repoDisplay := multiConfig.ResticRepository
	if len(repoDisplay) > 50 {
		repoDisplay = repoDisplay[:50] + "..."
	}
	logger.Log("  仓库地址: %s", repoDisplay)
	logger.Log("")
	
	logger.Log("启用的服务器列表：")
	for serverName, config := range multiConfig.Servers {
		logger.Log("  [%s]", serverName)
		logger.Log("    容器名称: %s", config.MCContainer)
		logger.Log("    世界目录: %s", config.WorldDir)
		logger.Log("    备份标签: %s", config.BackupTag)
		logger.Log("    主机标识: %s", config.BackupHost)
		logger.Log("")
	}
}

// checkContainerRunning 检查容器是否运行
func checkContainerRunning(containerName string) error {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, container := range containers {
		if container == containerName {
			return nil
		}
	}

	logger.Log("错误: 容器 %s 未运行", containerName)
	logger.Log("可用容器:")
	cmd = exec.Command("docker", "ps", "--format", "table {{.Names}}\t{{.Status}}")
	output, _ = cmd.Output()
	fmt.Print(string(output))

	return fmt.Errorf("container %s not running", containerName)
}

// execDockerCommand 执行 Docker 命令
func execDockerCommand(container string, args ...string) error {
	cmdArgs := append([]string{"exec", container}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getSnapshotCount 获取快照数量
func getSnapshotCount() (int, error) {
	cmd := exec.Command("restic", "snapshots", "--json", "--no-lock")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var snapshots []interface{}
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return 0, err
	}

	return len(snapshots), nil
}

// checkRepositoryConnection 检查仓库连接
func checkRepositoryConnection() error {
	maxAttempts := 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Log("尝试连接仓库 (第 %d/%d 次)...", attempt, maxAttempts)

		cmd := exec.Command("restic", "snapshots")
		output, err := cmd.CombinedOutput()

		if err == nil {
			logger.Log("仓库连接成功")
			return nil
		}

		outputStr := string(output)

		// 检查是否是锁定问题
		if strings.Contains(outputStr, "repository is already locked") {
			logger.Log("检测到仓库锁定问题，尝试解锁...")

			// 显示锁定信息
			lines := strings.Split(outputStr, "\n")
			for _, line := range lines {
				if strings.Contains(line, "locked by") || strings.Contains(line, "lock was created") || strings.Contains(line, "storage ID") {
					logger.Log("  %s", line)
				}
			}

			// 尝试解锁
			unlockCmd := exec.Command("restic", "unlock")
			if err := unlockCmd.Run(); err == nil {
				logger.Log("仓库解锁成功")
				time.Sleep(2 * time.Second)
			} else {
				logger.Log("警告: 自动解锁失败")
				if attempt == maxAttempts {
					logger.Log("错误: 无法解锁仓库，请手动执行: restic unlock")
					return fmt.Errorf("repository locked")
				}
			}
		} else if strings.Contains(outputStr, "Is there a repository") {
			// 仓库不存在，尝试初始化
			logger.Log("仓库不存在，尝试初始化...")
			initCmd := exec.Command("restic", "init")
			if err := initCmd.Run(); err == nil {
				logger.Log("仓库初始化成功")
				return nil
			} else {
				logger.Log("错误: 仓库初始化失败，请检查：")
				logger.Log("  1. R2 凭证是否正确")
				logger.Log("  2. 存储桶是否存在")
				logger.Log("  3. 网络连接是否正常")
				return fmt.Errorf("repository initialization failed")
			}
		} else {
			logger.Log("连接失败，错误信息：")
			lines := strings.Split(outputStr, "\n")
			for i, line := range lines {
				if i < 5 && line != "" {
					logger.Log("  %s", line)
				}
			}

			if attempt == maxAttempts {
				logger.Log("错误: 无法连接到 Restic 仓库")
				return fmt.Errorf("cannot connect to repository")
			}
		}

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("failed to connect to repository")
}

// waitForSaveCompletion 等待保存完成
func waitForSaveCompletion(container string, worldDir string) {
	logger.Log("等待世界保存完成...")
	maxAttempts := 30
	startTime := time.Now()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 获取最近的日志
		cmd := exec.Command("docker", "logs", "--since", startTime.Format(time.RFC3339), container)
		output, _ := cmd.Output()
		logs := string(output)

		// 检查保存完成信息
		if strings.Contains(logs, "Saved the game") ||
			strings.Contains(logs, "Saved the world") ||
			strings.Contains(logs, "ThreadedAnvilChunkStorage") && strings.Contains(logs, "Saved") {
			logger.Log("检测到保存完成日志")
			return
		}

		time.Sleep(2 * time.Second)
		logger.Log("等待保存完成... (%d/%d)", attempt+1, maxAttempts)
	}

	logger.Log("警告: 未检测到明确的保存完成信号，但继续备份")
}

// performBackup 执行备份
func performBackup(config *Config) error {
	logger.Log("开始增量备份...")

	// 记录备份前的快照数量
	snapshotCountBefore, _ := getSnapshotCount()

	// 执行备份
	cmd := exec.Command("restic", "backup",
		"--verbose",
		"--host", config.BackupHost,
		"--tag", config.BackupTag,
		config.WorldDir)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Log("备份失败")
		return err
	}

	// 验证备份
	snapshotCountAfter, _ := getSnapshotCount()

	if snapshotCountAfter > snapshotCountBefore {
		logger.Log("备份成功完成，新增快照 %d 个", snapshotCountAfter-snapshotCountBefore)
	} else {
		logger.Log("警告: 备份命令成功但未检测到新快照")
	}

	return nil
}

// 显示最新快照信息
func getLatestSnapshotInfo() {
	logger.Log("最新快照信息:")
	cmd := exec.Command("restic", "snapshots", "--latest", "1", "--compact", "--no-lock")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// backupSingleServer 备份单个服务器
func backupSingleServer(serverName string, config *Config) error {
	logger.Log("开始备份服务器: %s", serverName)
	
	// 检查容器是否运行
	if err := checkContainerRunning(config.MCContainer); err != nil {
		return fmt.Errorf("服务器 %s: %v", serverName, err)
	}

	// 设置清理函数
	saveOnExecuted := false
	defer func() {
		cleanup(config.MCContainer, saveOnExecuted)
	}()

	// 暂停写入
	logger.Log("[%s] 暂停 Minecraft 世界写入...", serverName)
	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-off"); err != nil {
		return fmt.Errorf("服务器 %s: 无法执行 save-off 命令: %v", serverName, err)
	}

	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-all"); err != nil {
		return fmt.Errorf("服务器 %s: 无法执行 save-all 命令: %v", serverName, err)
	}

	// 等待保存完成
	waitForSaveCompletion(config.MCContainer, config.WorldDir)

	// 执行备份
	if err := performBackup(config); err != nil {
		return fmt.Errorf("服务器 %s: 备份失败: %v", serverName, err)
	}

	// 恢复写入
	logger.Log("[%s] 恢复 Minecraft 世界写入...", serverName)
	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-on"); err != nil {
		logger.Log("警告: 服务器 %s 无法执行 save-on 命令，请手动检查", serverName)
	} else {
		saveOnExecuted = true
	}

	logger.Log("[%s] 服务器备份完成", serverName)
	return nil
}

// backupAllServers 备份所有启用的服务器
func backupAllServers(multiConfig *MultiServerConfig) error {
	if multiConfig.ParallelBackup {
		return backupServersParallel(multiConfig)
	} else {
		return backupServersSequential(multiConfig)
	}
}

// backupServersSequential 顺序备份所有服务器
func backupServersSequential(multiConfig *MultiServerConfig) error {
	var failedServers []string
	successCount := 0

	for serverName, config := range multiConfig.Servers {
		logger.Log("=" + strings.Repeat("=", 50))
		if err := backupSingleServer(serverName, config); err != nil {
			logger.Log("错误: %v", err)
			failedServers = append(failedServers, serverName)
		} else {
			successCount++
		}
		logger.Log("=" + strings.Repeat("=", 50))
		logger.Log("")
	}

	// 显示备份结果摘要
	logger.Log("备份结果摘要:")
	logger.Log("  成功: %d 个服务器", successCount)
	logger.Log("  失败: %d 个服务器", len(failedServers))
	
	if len(failedServers) > 0 {
		logger.Log("  失败的服务器: %s", strings.Join(failedServers, ", "))
		return fmt.Errorf("部分服务器备份失败")
	}

	return nil
}

// backupServersParallel 并行备份所有服务器
func backupServersParallel(multiConfig *MultiServerConfig) error {
	// 创建信号量控制并发数
	semaphore := make(chan struct{}, multiConfig.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failedServers []string
	successCount := 0

	logger.Log("启用并行备份，最大并发数: %d", multiConfig.MaxConcurrency)

	for serverName, config := range multiConfig.Servers {
		wg.Add(1)
		go func(name string, cfg *Config) {
			defer wg.Done()
			
			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			logger.Log("[并行] 开始备份服务器: %s", name)
			if err := backupSingleServer(name, cfg); err != nil {
				mu.Lock()
				failedServers = append(failedServers, name)
				mu.Unlock()
				logger.Log("[并行] 服务器 %s 备份失败: %v", name, err)
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
				logger.Log("[并行] 服务器 %s 备份成功", name)
			}
		}(serverName, config)
	}

	// 等待所有备份完成
	wg.Wait()

	// 显示备份结果摘要
	logger.Log("并行备份结果摘要:")
	logger.Log("  成功: %d 个服务器", successCount)
	logger.Log("  失败: %d 个服务器", len(failedServers))
	
	if len(failedServers) > 0 {
		logger.Log("  失败的服务器: %s", strings.Join(failedServers, ", "))
		return fmt.Errorf("部分服务器备份失败")
	}

	return nil
}

// cleanupSnapshots 清理旧快照
func cleanupSnapshots(config *Config) {
	logger.Log("开始清理旧快照...")
	maxAttempts := 2

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Log("尝试清理快照 (第 %d/%d 次)...", attempt, maxAttempts)

		cmd := exec.Command("restic", "forget", "--prune",
			"--keep-daily", strconv.Itoa(config.KeepDaily),
			"--keep-weekly", strconv.Itoa(config.KeepWeekly),
			"--keep-monthly", strconv.Itoa(config.KeepMonthly),
			"--keep-last", strconv.Itoa(config.KeepLast),
			"--tag", config.BackupTag)

		output, err := cmd.CombinedOutput()

		if err == nil {
			logger.Log("快照清理完成")
			return
		}

		outputStr := string(output)

		// 检查是否是锁定问题
		if strings.Contains(outputStr, "repository is already locked") {
			logger.Log("清理时检测到仓库锁定，尝试解锁...")
			unlockCmd := exec.Command("restic", "unlock")
			if err := unlockCmd.Run(); err == nil {
				logger.Log("解锁成功，重试清理...")
				time.Sleep(2 * time.Second)
			} else {
				logger.Log("警告: 解锁失败")
				break
			}
		} else {
			logger.Log("清理失败，错误信息：")
			lines := strings.Split(outputStr, "\n")
			for i, line := range lines {
				if i < 3 && line != "" {
					logger.Log("  %s", line)
				}
			}
			break
		}
	}

	logger.Log("警告: 快照清理失败，但备份已完成")
}

// cleanup 清理函数（错误处理）
func cleanup(container string, saveOnExecuted bool) {
	if saveOnExecuted {
		return
	}

	logger.Log("脚本执行失败，尝试恢复 Minecraft 世界写入...")
	if err := execDockerCommand(container, "rcon-cli", "save-on"); err != nil {
		logger.Log("警告: 无法恢复世界写入")
	}
}

func main() {
	// 获取配置文件路径
	configPath := getConfigPath()

	// 检查系统依赖
	if err := checkDependencies(); err != nil {
		os.Exit(1)
	}

	// 尝试加载配置文件
	multiConfig, err := loadConfig(configPath)
	if err != nil {
		if strings.Contains(err.Error(), "配置文件不存在") {
			logger.Log("配置文件不存在: %s", configPath)
			logger.Log("")
			
			// 创建示例配置文件
			if err := createSampleConfig(configPath); err != nil {
				logger.Log("创建配置文件失败: %v", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
		
		logger.Log("加载配置文件失败: %v", err)
		os.Exit(1)
	}

	// 检查网络连接
	checkNetwork()

	// 验证 restic 仓库连接
	logger.Log("验证 Restic 仓库连接...")
	if err := checkRepositoryConnection(); err != nil {
		os.Exit(1)
	}

	// 显示当前配置
	showConfig(multiConfig)

	logger.Log("开始多服务器备份流程...")
	logger.Log("共 %d 个服务器需要备份", len(multiConfig.Servers))

	// 备份所有启用的服务器
	if err := backupAllServers(multiConfig); err != nil {
		logger.Log("备份流程完成，但存在错误: %v", err)
		// 显示最新快照信息
		getLatestSnapshotInfo()
		// 清理旧快照（使用第一个服务器的配置）
		for _, config := range multiConfig.Servers {
			cleanupSnapshots(config)
			break
		}
		os.Exit(1)
	}

	// 显示最新快照信息
	getLatestSnapshotInfo()

	// 清理旧快照（使用共享的保留策略）
	if len(multiConfig.Servers) > 0 {
		cleanupSnapshots(multiConfig.Servers[0])
	}

	logger.Log("所有服务器备份流程全部完成")
} 