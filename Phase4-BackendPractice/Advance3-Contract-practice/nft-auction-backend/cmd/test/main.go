package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	// 使用 go-redis/v8 版本
	rdb "github.com/go-redis/redis/v8"
)

// 定义上下文，用于 Redis 操作
var ctx = context.Background()

func main() {
	// 1. 初始化 Redis 客户端
	client := rdb.NewClient(&rdb.Options{
		Addr:        "localhost:6379",
		Password:    "",              // 无密码
		DB:          0,               // 使用默认数据库
		DialTimeout: time.Second * 5, // 设置连接超时
		ReadTimeout: time.Second * 3, // 设置读取超时
	})

	// 测试连接
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("无法连接到 Redis: %v", err)
	}
	fmt.Println("成功连接到 Redis 服务器！")
	fmt.Println("----------------------------------------")

	// 清理旧数据，确保每次运行结果一致
	client.FlushDB(ctx)

	// --- 1. 字符串 (Strings) ---
	demoStrings(client)

	// --- 2. 列表 (Lists) - 增加范围删除 LTRIM ---
	demoLists(client)

	// --- 3. 哈希 (Hashes) ---
	demoHashes(client)

	// --- 4. 集合 (Sets) ---
	demoSets(client)

	// --- 5. 有序集合 (Sorted Sets - ZSets) - 增加范围删除 ZREMRANGEBY... ---
	demoSortedSets(client)
}

// 演示 String 类型操作 (使用 DEL 删除整个 Key)
func demoStrings(client *rdb.Client) {
	fmt.Println("--- 1. 字符串 (String) 演示 (使用 DEL 删除 Key) ---")
	keyName := "user:100:name"

	// SET/GET 操作
	client.Set(ctx, keyName, "Alice", 0).Err()
	val, _ := client.Get(ctx, keyName).Result()
	fmt.Printf("1. 初始获取的字符串值: %s\n", val)

	// --- 删除操作：DEL key ---
	delCount, _ := client.Del(ctx, keyName).Result()
	fmt.Printf("2. 执行 DEL 操作，删除键数量: %d\n", delCount)

	// 验证删除
	_, err := client.Get(ctx, keyName).Result()
	if err == rdb.Nil {
		fmt.Printf("3. 验证删除：键 '%s' 已不存在\n", keyName)
	}

	fmt.Println("----------------------------------------")
}

// 演示 List 类型操作 (包含 LTRIM 范围删除)
func demoLists(client *rdb.Client) {
	fmt.Println("--- 2. 列表 (List) 演示 (包含 LTRIM 范围删除) ---")
	listKey := "logs:daily"

	// 初始数据
	// 列表元素: [A, B, C, D, E, F] (从左到右，索引 0 到 5)
	client.RPush(ctx, listKey, "A", "B", "C", "D", "E", "F").Err()
	elements, _ := client.LRange(ctx, listKey, 0, -1).Result()
	fmt.Printf("1. 初始列表元素 (索引 0~5): %v\n", elements)

	// --- 范围删除操作：LTRIM key start end ---
	// LTRIM 的逻辑是：保留 (Trim) 指定范围内的元素，删除范围外的元素。
	// 这里我们希望保留索引 2 到 4 的元素（C, D, E），删除 A, B, F。
	// 列表最终应该只包含 [C, D, E]
	client.LTrim(ctx, listKey, 2, 4).Err()
	fmt.Printf("2. 执行 LTRIM 操作：保留索引 [2, 4] 的元素\n")

	// 验证范围删除
	remaining, _ := client.LRange(ctx, listKey, 0, -1).Result()
	fmt.Printf("3. 验证 LTRIM 后的剩余元素: %v\n", remaining) // 应该是 [C, D, E]

	// 演示 LPop/RPop 删除元素
	poppedRight, _ := client.RPop(ctx, listKey).Result()
	fmt.Printf("4. RPop 从右侧弹出元素: %s\n", poppedRight) // E

	// 最终使用 DEL 删除整个列表 Key
	client.Del(ctx, listKey).Err()

	fmt.Println("----------------------------------------")
}

// 演示 Hash 类型操作 (使用 HDEL 删除字段)
func demoHashes(client *rdb.Client) {
	fmt.Println("--- 3. 哈希 (Hash) 演示 (使用 HDEL 删除字段) ---")
	hashKey := "product:iphone15"

	// 初始数据
	client.HSet(ctx, hashKey,
		"model", "iPhone 15 Pro",
		"price", "9999",
		"storage", "256GB",
		"color", "Black",
	).Err()

	allFields, _ := client.HGetAll(ctx, hashKey).Result()
	fmt.Printf("1. 产品初始信息 (共 %d 个字段): %v\n", len(allFields), allFields)

	// --- 删除操作：HDEL key field ---
	// 删除 price 和 color 两个字段
	delFieldsCount, _ := client.HDel(ctx, hashKey, "price", "color").Result()
	fmt.Printf("2. 执行 HDEL 操作，删除字段数量: %d\n", delFieldsCount)

	// 验证删除
	allFieldsAfterDel, _ := client.HGetAll(ctx, hashKey).Result()
	fmt.Printf("3. 验证删除：剩余信息 (共 %d 个字段): %v\n", len(allFieldsAfterDel), allFieldsAfterDel)

	// 最终删除整个 Key
	client.Del(ctx, hashKey).Err()

	fmt.Println("----------------------------------------")
}

// 演示 Set 类型操作 (使用 SREM 删除成员)
func demoSets(client *rdb.Client) {
	fmt.Println("--- 4. 集合 (Set) 演示 (使用 SREM 删除成员) ---")
	setKeyA := "users:frontend_dev"

	// 初始数据 (集合会自动去重)
	client.SAdd(ctx, setKeyA, "user:1", "user:2", "user:3", "user:4").Err()

	members, _ := client.SMembers(ctx, setKeyA).Result()
	fmt.Printf("1. 初始集合成员: %v\n", members)

	// --- 删除操作：SREM key member ---
	// 移除 user:2 和 user:4
	remCount, _ := client.SRem(ctx, setKeyA, "user:2", "user:4").Result()
	fmt.Printf("2. 执行 SREM 操作，移除成员数量: %d\n", remCount)

	// 验证删除
	membersAfterRem, _ := client.SMembers(ctx, setKeyA).Result()
	fmt.Printf("3. 验证删除：剩余集合成员: %v\n", membersAfterRem)

	// 最终删除整个 Key
	client.Del(ctx, setKeyA).Err()

	fmt.Println("----------------------------------------")
}

// 演示 Sorted Set (ZSet) 类型操作 (包含范围删除 ZREMRANGEBY...)
func demoSortedSets(client *rdb.Client) {
	fmt.Println("--- 5. 有序集合 (Sorted Set / ZSet) 演示 (包含范围删除) ---")
	zsetKey := "game:leaderboard"

	// 初始数据
	members := []*rdb.Z{
		{Score: 80.0, Member: "Player D"},  // 排名 4
		{Score: 90.0, Member: "Player C"},  // 排名 2
		{Score: 100.0, Member: "Player A"}, // 排名 1
		{Score: 85.0, Member: "Player B"},  // 排名 3
		{Score: 70.0, Member: "Player E"},  // 排名 5
	}
	client.ZAdd(ctx, zsetKey, members...).Err()

	// 按降序获取所有成员 (排名)
	initialPlayers, _ := client.ZRevRangeWithScores(ctx, zsetKey, 0, -1).Result()
	fmt.Println("1. 初始排行榜 (降序):")
	for i, z := range initialPlayers {
		fmt.Printf("  No.%d: %s (分数: %s)\n", i+1, z.Member, strconv.FormatFloat(z.Score, 'f', 0, 64))
	}

	// --- 范围删除操作 A: ZREMRANGEBYRANK (按排名删除) ---
	// 删除排名第 4 到第 5 的玩家（即索引 3 到 4，Player D 和 Player E）。
	// 注意：Redis 索引从 0 开始。
	remRankCount, _ := client.ZRemRangeByRank(ctx, zsetKey, 3, 4).Result()
	fmt.Printf("2. 执行 ZREMRANGEBYRANK：删除排名索引 [3, 4] 的成员，数量: %d\n", remRankCount)

	// 验证排名删除后的剩余成员
	remainingPlayers, _ := client.ZRevRangeWithScores(ctx, zsetKey, 0, -1).Result()
	fmt.Println("3. 剩余排行榜 (删除 Player D, E):")
	for i, z := range remainingPlayers {
		fmt.Printf("  No.%d: %s (分数: %s)\n", i+1, z.Member, strconv.FormatFloat(z.Score, 'f', 0, 64))
	}

	// --- 范围删除操作 B: ZREMRANGEBYSCORE (按分数删除) ---
	// 删除分数低于 90.0 的玩家（即 (85.0, Player B)）。
	// 语法：`ZRemRangeByScore(ctx, key, minScore, maxScore)`
	remScoreCount, _ := client.ZRemRangeByScore(ctx, zsetKey, "-inf", "89.9").Result() // "-inf" 表示负无穷
	fmt.Printf("4. 执行 ZREMRANGEBYSCORE：删除分数在 [-inf, 89.9] 范围的成员，数量: %d\n", remScoreCount)

	// 验证分数删除后的最终成员
	finalPlayers, _ := client.ZRevRangeWithScores(ctx, zsetKey, 0, -1).Result()
	fmt.Println("5. 最终排行榜 (删除 Player B):")
	for i, z := range finalPlayers {
		fmt.Printf("  No.%d: %s (分数: %s)\n", i+1, z.Member, strconv.FormatFloat(z.Score, 'f', 0, 64))
	}

	// 最终删除整个 Key
	client.Del(ctx, zsetKey).Err()

	fmt.Println("----------------------------------------")
}
