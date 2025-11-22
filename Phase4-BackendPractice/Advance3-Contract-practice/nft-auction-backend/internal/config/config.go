package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config 包含了應用程序所需的所有配置設置。
type Config struct {
	RPCURL          string
	ContractAddress string
	MySQLDSN        string
	HTTPPort        string

	// Redis 相關配置
	RedisHost     string
	RedisPort     string
	RedisPassword string // 可選，如果 Redis 設置了密碼
}

// Load 函數從環境變量中加載所有配置
func Load() *Config {
	// 嘗試從不同路徑加載 .env 文件以支持不同的運行環境
	_ = godotenv.Load("../.env")    // when run from cmd/server
	_ = godotenv.Load("../../.env") // fallback
	_ = godotenv.Load(".env")       // just in case

	cfg := &Config{
		RPCURL:          os.Getenv("RPC_URL"),
		ContractAddress: os.Getenv("CONTRACT_ADDRESS"),
		MySQLDSN:        os.Getenv("MYSQL_DSN"),
		HTTPPort:        os.Getenv("HTTP_PORT"),

		// Redis 配置加載
		RedisHost:     os.Getenv("REDIS_HOST"),
		RedisPort:     os.Getenv("REDIS_PORT"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
	}

	// 核心服務檢查
	if cfg.RPCURL == "" {
		log.Fatal("RPC_URL not set")
	}
	if cfg.MySQLDSN == "" {
		log.Fatal("MYSQL_DSN not set")
	}
	if cfg.HTTPPort == "" {
		log.Fatal("HTTP_PORT not set")
	}

	// Redis 檢查 (如果 Redis 是核心依賴，應該檢查)
	if cfg.RedisHost == "" {
		// 通常 Redis Host 是必須的
		log.Fatal("REDIS_HOST not set")
	}
	if cfg.RedisPort == "" {
		// 默認使用 6379, 但最好讓它明確配置
		log.Fatal("REDIS_PORT not set")
	}

	return cfg
}
