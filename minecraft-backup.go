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
	"time"

	"github.com/BurntSushi/toml"
)

// TOMLConfig 是 TOML 配置文件的结构
type TOMLConfig struct {
	// 基础配置
	General GeneralConfig `toml:"general"`
	
	// AWS/R2 凭证
	AWS AWSConfig `toml:"aws"`
	
	// Restic 配置
	Restic ResticConfig `toml:"restic"`
	
	// 备份策略
	Retention RetentionConfig `toml:"retention"`
}

// GeneralConfig 通用配置
type GeneralConfig struct {
	ContainerName string `toml:"container_name"`
	WorldDir      string `toml:"world_dir"`
	BackupTag     string `toml:"backup_tag"`
	BackupHost    string `toml:"backup_host"`
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

// Config 运行时配置（合并了 TOML 配置和环境变量）
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
	sampleConfig := `# Minecraft 备份工具配置文件

[general]
# Minecraft Docker 容器名称
container_name = "minecraft-mc-1"

# 世界文件目录（主机路径）
world_dir = "%s/docker/minecraft"

# 备份标签（用于标识备份）
backup_tag = "minecraft-test"

# 主机标识（默认为主机名）
backup_host = "%s"

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
`

	// 格式化配置内容
	configContent := fmt.Sprintf(sampleConfig, homeDir, hostname)

	// 写入文件，设置安全权限
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return err
	}

	logger.Log("已创建示例配置文件: %s", configPath)
	logger.Log("")
	logger.Log("请编辑配置文件并填入正确的信息：")
	logger.Log("  1. 修改 [aws] 部分，填入您的 R2 凭证")
	logger.Log("  2. 修改 [restic] 部分，设置仓库地址和密码")
	logger.Log("  3. 根据需要调整其他配置")
	logger.Log("  4. 重新运行程序")

	return nil
}

// loadConfig 加载配置文件
func loadConfig(configPath string) (*Config, error) {
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

	// 转换为运行时配置
	config := &Config{
		ConfigFile:         configPath,
		MCContainer:        tomlConfig.General.ContainerName,
		WorldDir:           tomlConfig.General.WorldDir,
		BackupTag:          tomlConfig.General.BackupTag,
		BackupHost:         tomlConfig.General.BackupHost,
		AWSAccessKeyID:     tomlConfig.AWS.AccessKeyID,
		AWSSecretAccessKey: tomlConfig.AWS.SecretAccessKey,
		AWSRegion:          tomlConfig.AWS.Region,
		ResticRepository:   tomlConfig.Restic.Repository,
		ResticPassword:     tomlConfig.Restic.Password,
		KeepDaily:          tomlConfig.Retention.KeepDaily,
		KeepWeekly:         tomlConfig.Retention.KeepWeekly,
		KeepMonthly:        tomlConfig.Retention.KeepMonthly,
		KeepLast:           tomlConfig.Retention.KeepLast,
	}

	// 展开环境变量（如果路径中包含 ~）
	if strings.HasPrefix(config.WorldDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		config.WorldDir = filepath.Join(homeDir, config.WorldDir[2:])
	}

	// 设置环境变量
	os.Setenv("AWS_ACCESS_KEY_ID", config.AWSAccessKeyID)
	os.Setenv("AWS_SECRET_ACCESS_KEY", config.AWSSecretAccessKey)
	os.Setenv("AWS_DEFAULT_REGION", config.AWSRegion)
	os.Setenv("RESTIC_REPOSITORY", config.ResticRepository)
	os.Setenv("RESTIC_PASSWORD", config.ResticPassword)

	return config, nil
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
func showConfig(config *Config) {
	logger.Log("当前备份配置：")
	logger.Log("  配置文件: %s", config.ConfigFile)
	logger.Log("  容器名称: %s", config.MCContainer)
	logger.Log("  世界目录: %s", config.WorldDir)
	logger.Log("  备份标签: %s", config.BackupTag)
	logger.Log("  主机标识: %s", config.BackupHost)
	logger.Log("  保留策略:")
	logger.Log("    - 每日快照: %d 个", config.KeepDaily)
	logger.Log("    - 每周快照: %d 个", config.KeepWeekly)
	logger.Log("    - 每月快照: %d 个", config.KeepMonthly)
	logger.Log("    - 最近快照: %d 个", config.KeepLast)
	repoDisplay := config.ResticRepository
	if len(repoDisplay) > 50 {
		repoDisplay = repoDisplay[:50] + "..."
	}
	logger.Log("  仓库地址: %s", repoDisplay)
	logger.Log("")
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
	config, err := loadConfig(configPath)
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

	// 检查容器是否运行
	if err := checkContainerRunning(config.MCContainer); err != nil {
		os.Exit(1)
	}

	// 验证 restic 仓库连接
	logger.Log("验证 Restic 仓库连接...")
	if err := checkRepositoryConnection(); err != nil {
		os.Exit(1)
	}

	// 显示当前配置
	showConfig(config)

	logger.Log("开始备份流程...")

	// 设置清理函数
	saveOnExecuted := false
	defer func() {
		cleanup(config.MCContainer, saveOnExecuted)
	}()

	// 暂停写入
	logger.Log("暂停 Minecraft 世界写入...")
	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-off"); err != nil {
		logger.Log("错误: 无法执行 save-off 命令")
		os.Exit(1)
	}

	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-all"); err != nil {
		logger.Log("错误: 无法执行 save-all 命令")
		os.Exit(1)
	}

	// 等待保存完成
	waitForSaveCompletion(config.MCContainer, config.WorldDir)

	// 执行备份
	if err := performBackup(config); err != nil {
		os.Exit(1)
	}

	// 恢复写入
	logger.Log("恢复 Minecraft 世界写入...")
	if err := execDockerCommand(config.MCContainer, "rcon-cli", "save-on"); err != nil {
		logger.Log("警告: 无法执行 save-on 命令，请手动检查")
	} else {
		saveOnExecuted = true
	}

	// 显示最新快照信息
	getLatestSnapshotInfo()

	// 清理旧快照
	cleanupSnapshots(config)

	logger.Log("备份流程全部完成")
} 