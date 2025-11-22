package storage

import (
	"context"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient 包裹了原生的 redis.Client
type RedisClient struct {
	Client *redis.Client
	Ctx    context.Context
}

// GlobalCtx 作為所有 Redis 操作的基礎 context
var GlobalCtx = context.Background()

// InitRedis 初始化並連接到 Redis 服務
func InitRedis(addr, password string) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password, // 密碼 (如果沒有設置，則為空)
		DB:           0,        // 使用默認數據庫
		PoolSize:     10,       // 連接池大小
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// 嘗試 Ping 服務器以驗證連接
	// 由於這是啟動流程的一部分，我們使用 GlobalCtx
	status := rdb.Ping(GlobalCtx)
	if status.Err() != nil {
		log.Fatalf("Fatal error: Failed to connect to Redis at %s: %v", addr, status.Err())
	}

	log.Println("✅ Redis client initialized and connected successfully.")

	return &RedisClient{
		Client: rdb,
		Ctx:    GlobalCtx,
	}
}

// CloseRedis 關閉 Redis 連接
func CloseRedis(rdb *RedisClient) {
	if rdb != nil && rdb.Client != nil {
		err := rdb.Client.Close()
		if err != nil {
			log.Printf("Error closing Redis client: %v", err)
		} else {
			log.Println("Redis client closed.")
		}
	}
}
