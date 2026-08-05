// Package config 负责加载应用配置。
// 唯一配置来源：代码内默认值 + 根目录 .env 文件 + 真实环境变量。
// 优先级：真实环境变量 > .env 文件 > 代码默认值。
// 注意：viper AutomaticEnv 对未注册的 key 不生效，因此全部键必须用 SetDefault 显式注册。
package config

import (
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 应用全局配置
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	CORS      CORSConfig      `mapstructure:"cors"`
	Log       LogConfig       `mapstructure:"log"`
	App       AppConfig       `mapstructure:"app"`
	Wechat    WechatConfig    `mapstructure:"wechat"`
	Upload    UploadConfig    `mapstructure:"upload"`
	OSS       OSSConfig       `mapstructure:"oss"`
	Watermark WatermarkConfig `mapstructure:"watermark"`
	Admin     AdminConfig     `mapstructure:"admin"`
	SPA       SPAConfig       `mapstructure:"spa"`
	Env       string          `mapstructure:"-"` // 当前运行环境（dev/prod），来自 APP_ENV
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type PostgresConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

type CORSConfig struct {
	AllowOrigins []string `mapstructure:"allow_origins"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type AppConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

// WechatConfig 小程序配置；AppID 为空或 Mock=true 时进入 mock 登录模式（仅开发联调）。
type WechatConfig struct {
	AppID  string `mapstructure:"appid"`
	Secret string `mapstructure:"secret"`
	Mock   bool   `mapstructure:"mock"`
}

// MockEnabled 是否处于 mock 模式（配置缺失自动降级，便于开发联调）。
func (w WechatConfig) MockEnabled() bool { return w.Mock || w.AppID == "" || w.Secret == "" }

type UploadConfig struct {
	Mode         string   `mapstructure:"mode"`
	LocalDir     string   `mapstructure:"local_dir"`
	MaxFileSize  int64    `mapstructure:"max_file_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
}

type OSSConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	RoleArn         string `mapstructure:"role_arn"`
	Bucket          string `mapstructure:"bucket"`
	Endpoint        string `mapstructure:"endpoint"`
	ExpireSeconds   int    `mapstructure:"expire_seconds"`
}

type WatermarkConfig struct {
	FontPath string `mapstructure:"font_path"`
}

// AdminConfig 初始超管账号（seed 时读取，不存在才创建）。
type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

// SPAConfig 生产单端口托管前端静态资源。
type SPAConfig struct {
	DistPath string `mapstructure:"dist_path"` // 前端构建产物目录，容器内 /app/dist
}

// Load 读取配置：先加载 .env 文件（不覆盖已存在的真实环境变量），再以环境变量覆盖代码默认值。
func Load() (*Config, error) {
	loadEnvFile()

	v := viper.New()
	// 环境变量：PI_POSTGRES_HOST -> postgres.host
	v.SetEnvPrefix("PI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	// 无前缀的关键变量（部署规范约定）
	v.BindEnv("admin.username", "ADMIN_USERNAME")
	v.BindEnv("admin.password", "ADMIN_PASSWORD")
	v.BindEnv("admin.name", "ADMIN_NAME")
	v.BindEnv("spa.dist_path", "SPA_DIST_PATH")
	registerDefaults(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.Env = os.Getenv("APP_ENV")
	if cfg.Env == "" {
		cfg.Env = "dev"
	}
	return &cfg, nil
}

// registerDefaults 代码内默认值（同时完成键注册，保证 AutomaticEnv 覆盖生效）。
func registerDefaults(v *viper.Viper) {
	defaults := map[string]any{
		"server.port":            8090,
		"server.mode":            "debug",
		"postgres.host":          "127.0.0.1",
		"postgres.port":          5432,
		"postgres.user":          "postgres",
		"postgres.password":      "",
		"postgres.dbname":        "property_inspection",
		"postgres.sslmode":       "disable",
		"postgres.max_open_conns": 20,
		"postgres.max_idle_conns": 5,
		"redis.addr":             "127.0.0.1:6379",
		"redis.password":         "",
		"redis.db":               0,
		"jwt.secret":             "dev-jwt-secret-please-change",
		"jwt.access_ttl":         "2h",
		"jwt.refresh_ttl":        "168h",
		"cors.allow_origins":     []string{"*"},
		"log.level":              "info",
		"app.base_url":           "http://127.0.0.1:8090",
		"wechat.appid":           "",
		"wechat.secret":          "",
		"wechat.mock":            true,
		"upload.mode":            "dev",
		"upload.local_dir":       "uploads",
		"upload.max_file_size":   20971520,
		"upload.allowed_types":   []string{"jpg", "jpeg", "png", "heic"},
		"oss.access_key_id":      "",
		"oss.access_key_secret":  "",
		"oss.role_arn":           "",
		"oss.bucket":             "",
		"oss.endpoint":           "",
		"oss.expire_seconds":     3600,
		"watermark.font_path":    "",
		"admin.username":         "admin",
		"admin.password":         "Admin@123",
		"admin.name":             "系统管理员",
		"spa.dist_path":          "",
	}
	for k, val := range defaults {
		v.SetDefault(k, val)
	}
}

// loadEnvFile 按 ENV_FILE / APP_ENV 加载项目根目录 .env 文件（真实环境变量优先）。
// 依次尝试上级目录与当前目录（backend/ 本地运行时根目录在上一级）。
func loadEnvFile() {
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		env := os.Getenv("APP_ENV")
		if env == "" {
			env = "dev"
		}
		envFile = ".env." + env
	}
	candidates := []string{envFile}
	if !strings.ContainsRune(envFile, '/') && !strings.ContainsRune(envFile, '\\') {
		candidates = append([]string{"../" + envFile}, candidates...)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			// Load 不覆盖已存在的环境变量，保证真实环境变量优先
			_ = godotenv.Load(p)
			return
		}
	}
}
