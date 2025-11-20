package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RPCURL          string
	ContractAddress string
	MySQLDSN        string
	HTTPPort        string
}

func Load() *Config {
	_ = godotenv.Load("../.env")    // when run from cmd/server
	_ = godotenv.Load("../../.env") // fallback
	_ = godotenv.Load(".env")       // just in case

	cfg := &Config{
		RPCURL:          os.Getenv("RPC_URL"),
		ContractAddress: os.Getenv("CONTRACT_ADDRESS"),
		MySQLDSN:        os.Getenv("MYSQL_DSN"),
		HTTPPort:        os.Getenv("HTTP_PORT"),
	}

	if cfg.RPCURL == "" {
		log.Fatal("RPC_URL not set")
	}
	return cfg
}
