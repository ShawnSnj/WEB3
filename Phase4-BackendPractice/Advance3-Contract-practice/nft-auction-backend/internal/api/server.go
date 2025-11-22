package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"nft-auction-backend/internal/config"
	"nft-auction-backend/internal/storage" // 引入 Redis 存儲

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
)

// Server 結構體用於存放依賴，例如資料庫連接和 Redis 客戶端
type Server struct {
	DB  *sql.DB
	RDB *storage.RedisClient // 新增 Redis 客戶端
}

// NewServer 創建一個新的 Server 實例
func NewServer(db *sql.DB, rdb *storage.RedisClient) *Server {
	return &Server{DB: db, RDB: rdb}
}

// --- 中間件 (Middleware) ---

// AuthMiddleware 是 Gin 框架的認證中間件
func (s *Server) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ⚠️ 實際應用中，應從 Header (e.g., Authorization) 獲取並驗證 JWT Token

		// Mock 用戶 ID
		userID := "0x1a41a486130B3f75ed350e9873177B1A75Ac9c33"

		// 將用戶ID存入 Gin Context 中，供後續 Handler 使用
		c.Set("userID", userID)

		c.Next()
	}
}

// --- 新增 API 處理函數 (New API Handlers) ---

// handleMyBids 處理出價者查看自己參與的所有拍賣 (無需 Redis，直接查詢 MySQL)
// GET /api/v1/me/bids?status=active|ended
func (s *Server) handleMyBids(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	status := c.Query("status") // active or ended

	// 基礎查詢：找出用戶出價過的拍賣 ID
	baseQuery := `
        SELECT DISTINCT a.auction_id
        FROM auctions a
        JOIN bids b ON a.auction_id = b.auction_id
        WHERE b.bidder = ?`

	// 根據 status 調整 where 條件
	whereStatus := ""
	if status == "active" {
		whereStatus = "AND a.ended = 0"
	} else if status == "ended" {
		whereStatus = "AND a.ended = 1"
	}

	// 最終查詢：查詢用戶參與拍賣的詳細信息
	query := fmt.Sprintf(`
        SELECT 
            t.auction_id, t.token_id, t.seller, t.start_price, t.end_time, t.winner, t.final_price, t.ended,
            IFNULL(MAX(b.amount), t.start_price) AS highest_bid,
            COUNT(b.id) AS bid_count, 'MockContractAddress' AS nft_contract
        FROM auctions t
        LEFT JOIN bids b ON t.auction_id = b.auction_id
        WHERE t.auction_id IN (%s %s)
        GROUP BY t.auction_id
        ORDER BY t.end_time DESC`, baseQuery, whereStatus)

	rows, err := s.DB.Query(query, userID, userID) // 這裡需要傳遞兩次 userID
	if err != nil {
		log.Printf("Error executing my bids query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query my bids"})
		return
	}
	defer rows.Close()

	auctions := []Auction{}
	for rows.Next() {
		var a Auction
		var winner, finalPrice sql.NullString

		err := rows.Scan(
			&a.AuctionID, &a.TokenID, &a.Seller, &a.StartPrice, &a.EndTime,
			&winner, &finalPrice, &a.Ended, &a.HighestBid, &a.BidCount, &a.NFTContract)

		if err != nil {
			log.Printf("Error scanning my bids row: %v", err)
			continue
		}

		a.Winner = winner.String
		if finalPrice.Valid {
			if fp, e := strconv.ParseInt(finalPrice.String, 10, 64); e == nil {
				a.FinalPrice = fp
			}
		}

		auctions = append(auctions, a)
	}

	c.JSON(http.StatusOK, auctions)
}

// handleSubmitBid 處理出價者參與出價
// POST /api/v1/auctions/:auctionId/bid
func (s *Server) handleSubmitBid(c *gin.Context) {
	userID, _ := c.Get("userID")
	auctionIDStr := c.Param("auctionId")
	auctionID, err := strconv.ParseInt(auctionIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid auction ID format"})
		return
	}

	var req BidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bid request format"})
		return
	}

	// 1. 檢查拍賣狀態和當前最高價 (優先從 Redis 獲取實時最高價)
	var currentHighestBid int64 = 0
	var endTime int64
	var ended bool
	foundInRedis := false

	// 1.1. 嘗試從 Redis 獲取當前最高出價 (實時數據)
	redisKey := fmt.Sprintf("auction:%d:highest_bid", auctionID)
	bidMap, err := s.RDB.Client.HGetAll(s.RDB.Ctx, redisKey).Result()
	if err == nil {
		if amountStr, ok := bidMap["amount"]; ok && amountStr != "" {
			if highestBid, e := strconv.ParseInt(amountStr, 10, 64); e == nil {
				currentHighestBid = highestBid
				foundInRedis = true
			}
		}
	} else {
		log.Printf("Redis HGetAll error in handleSubmitBid for AuctionID %d: %v", auctionID, err)
	}

	// 1.2. 從 MySQL 獲取拍賣基礎信息 (結束時間和狀態)
	err = s.DB.QueryRow(`
        SELECT end_time, ended
        FROM auctions
        WHERE auction_id = ?`, auctionID).Scan(&endTime, &ended)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Auction not found"})
		return
	} else if err != nil {
		log.Printf("Error checking auction status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check auction status"})
		return
	}

	// 1.3. 如果 Redis 中沒有找到最高出價 (例如：Redis 重啟或剛開始)，則從 MySQL 獲取 StartPrice/Max Bid 作為備用
	if !foundInRedis {
		err = s.DB.QueryRow(`
			SELECT IFNULL(MAX(b.amount), a.start_price)
			FROM auctions a
			LEFT JOIN bids b ON a.auction_id = b.auction_id
			WHERE a.auction_id = ?
			GROUP BY a.auction_id`, auctionID).Scan(&currentHighestBid)

		if err != nil {
			log.Printf("Error checking bid status fallback: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to determine current highest bid"})
			return
		}
	}
	// --- 狀態檢查 (使用從 Redis 或 MySQL 獲取的 currentHighestBid) ---
	if ended {
		c.JSON(http.StatusConflict, gin.H{"error": "Auction has already ended"})
		return
	}

	if time.Now().Unix() > endTime {
		c.JSON(http.StatusConflict, gin.H{"error": "Auction time has expired"})
		return
	}

	if req.Amount <= currentHighestBid {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Bid amount (%d) must be higher than current highest bid (%d)", req.Amount, currentHighestBid)})
		return
	}

	// 2. 模擬區塊鏈交易
	mockTxData := gin.H{
		"sender":  userID,
		"to":      "0x627bEd9E638C4158da5d79cA503006361F7c2b66", // Auction Contract
		"data":    fmt.Sprintf("0x...encodedBid(%d, %d)...", auctionID, req.Amount),
		"summary": fmt.Sprintf("Submit bid of %d for Auction ID %d", req.Amount, auctionID),
	}

	// 3. 模擬將交易結果寫入資料庫
	// ⚠️ 在實際應用中，這一行為應由區塊鏈監聽器觸發，而不是直接在 API 中寫入
	_, err = s.DB.Exec(`
        INSERT INTO bids (auction_id, bidder, amount, timestamp_utc) 
        VALUES (?, ?, ?, ?)`,
		auctionID, userID, req.Amount, time.Now().Unix())

	if err != nil {
		log.Printf("Error inserting mock bid: %v", err)
	}

	c.JSON(http.StatusAccepted, mockTxData)
}

// handleFinalizeAuction 處理獲勝者結算拍賣 (無需 Redis)
// POST /api/v1/auctions/:auctionId/finalize
func (s *Server) handleFinalizeAuction(c *gin.Context) {
	userID, _ := c.Get("userID")
	auctionIDStr := c.Param("auctionId")
	auctionID, err := strconv.ParseInt(auctionIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid auction ID format"})
		return
	}

	// 1. 檢查拍賣狀態、時間和最高出價者
	var isEnded int
	var winner sql.NullString
	var finalPrice sql.NullInt64

	err = s.DB.QueryRow(`
        SELECT a.ended, t.bidder, t.amount
        FROM auctions a
        LEFT JOIN (SELECT bidder, amount FROM bids WHERE auction_id = ? ORDER BY amount DESC LIMIT 1) t
        ON 1=1
        WHERE a.auction_id = ?`, auctionID, auctionID).Scan(&isEnded, &winner, &finalPrice)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Auction not found"})
		return
	} else if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking auction for finalize: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check auction status"})
		return
	}

	if isEnded == 1 {
		c.JSON(http.StatusConflict, gin.H{"error": "Auction is already finalized"})
		return
	}

	if time.Now().Unix() < s.getAuctionEndTime(auctionID) {
		c.JSON(http.StatusConflict, gin.H{"error": "Auction has not expired yet"})
		return
	}

	// 檢查用戶是否是贏家
	if !winner.Valid || winner.String != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the auction winner can finalize this auction"})
		return
	}

	// 2. 模擬區塊鏈交易 (調用合約的 finalize 函數)
	mockTxData := gin.H{
		"sender":  userID,
		"to":      "0x627bEd9E638C4158da5d79cA503006361F7c2b66", // Auction Contract
		"data":    fmt.Sprintf("0x...encodedFinalize(%d)...", auctionID),
		"summary": fmt.Sprintf("Finalize Auction ID %d and claim NFT for %d", auctionID, finalPrice.Int64),
	}

	// 3. 模擬將交易結果寫入資料庫
	// ⚠️ 在實際應用中，這一行為應由區塊鏈監聽器觸發
	_, err = s.DB.Exec(`
        UPDATE auctions 
        SET ended = 1, winner = ?, final_price = ? 
        WHERE auction_id = ?`,
		userID, finalPrice.Int64, auctionID)

	if err != nil {
		log.Printf("Error updating mock auction status: %v", err)
	}

	c.JSON(http.StatusAccepted, mockTxData)
}

// getAuctionEndTime 輔助函數：獲取拍賣結束時間
func (s *Server) getAuctionEndTime(auctionID int64) int64 {
	var endTime int64
	err := s.DB.QueryRow("SELECT end_time FROM auctions WHERE auction_id = ?", auctionID).Scan(&endTime)
	if err != nil {
		return 0 // 返回 0 或其他錯誤指示
	}
	return endTime
}

// --- 舊有 API 處理函數 (Existing API Handlers) ---

// handleAuthConnect 處理用戶連接加密錢包
func (s *Server) handleAuthConnect(c *gin.Context) {
	// ⚠️ 實際邏輯：驗證簽名，生成 JWT
	c.JSON(http.StatusOK, gin.H{
		"token": "mock-jwt-token-for-0x1a41a486130B3f75ed350e9873177B1A75Ac9c33",
	})
	log.Println("POST /api/v1/auth/connect: Mock token issued.")
}

// handleStats 處理平台統計資訊
func (s *Server) handleStats(c *gin.Context) {
	var stats StatsResponse

	// 1. 拍賣總數
	err := s.DB.QueryRow("SELECT COUNT(*) FROM auctions").Scan(&stats.TotalAuctions)
	// 2. 出價總數
	if err == nil {
		err = s.DB.QueryRow("SELECT COUNT(*) FROM bids").Scan(&stats.TotalBids)
	}
	// 3. 用戶總數
	if err == nil {
		err = s.DB.QueryRow(`
            SELECT COUNT(DISTINCT address) FROM (
                SELECT seller AS address FROM auctions
                UNION
                SELECT bidder AS address FROM bids
            ) AS participants`).Scan(&stats.TotalUsers)
	}

	if err != nil {
		log.Printf("Error querying stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query platform statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleAuctions 處理拍賣列表 (進行中/已結束) (無需 Redis，直接查詢 MySQL)
func (s *Server) handleAuctions(c *gin.Context) {
	status := c.Query("status")
	whereClause := "WHERE ended = 0"

	if status == "ended" {
		whereClause = "WHERE ended = 1"
	}

	query := fmt.Sprintf(`
        SELECT 
            a.auction_id, a.token_id, a.seller, a.start_price, a.end_time, a.winner, a.final_price, a.ended,
            IFNULL(MAX(b.amount), a.start_price) AS highest_bid,
            COUNT(b.id) AS bid_count, 'MockContractAddress' AS nft_contract
        FROM auctions a
        LEFT JOIN bids b ON a.auction_id = b.auction_id
        %s
        GROUP BY a.auction_id
        ORDER BY a.end_time DESC`, whereClause)

	rows, err := s.DB.Query(query)
	if err != nil {
		log.Printf("Error executing auctions query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query auctions"})
		return
	}
	defer rows.Close()

	auctions := []Auction{}
	for rows.Next() {
		var a Auction
		var winner, finalPrice sql.NullString

		err := rows.Scan(
			&a.AuctionID, &a.TokenID, &a.Seller, &a.StartPrice, &a.EndTime,
			&winner, &finalPrice, &a.Ended, &a.HighestBid, &a.BidCount, &a.NFTContract)

		if err != nil {
			log.Printf("Error scanning auction row: %v", err)
			continue
		}

		a.Winner = winner.String
		if finalPrice.Valid {
			if fp, e := strconv.ParseInt(finalPrice.String, 10, 64); e == nil {
				a.FinalPrice = fp
			}
		}

		auctions = append(auctions, a)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rows iteration error"})
		return
	}

	c.JSON(http.StatusOK, auctions)
}

// handleAuctionDetail 處理單個拍賣詳情 (新增 Redis 查詢實時最高價)
func (s *Server) handleAuctionDetail(c *gin.Context) {
	// 從 URL 參數中獲取 auctionId
	auctionIDStr := c.Param("auctionId")
	auctionID, err := strconv.ParseInt(auctionIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid auction ID format"})
		return
	}

	// 1. 查詢拍賣主體信息 (使用 MySQL 獲取所有數據作為可靠來源)
	var a Auction
	var winner, finalPrice sql.NullString
	auctionQuery := `
        SELECT 
            a.auction_id, a.token_id, a.seller, a.start_price, a.end_time, a.winner, a.final_price, a.ended,
            IFNULL(MAX(b.amount), a.start_price) AS highest_bid,
            COUNT(b.id) AS bid_count, 'MockContractAddress' AS nft_contract
        FROM auctions a
        LEFT JOIN bids b ON a.auction_id = b.auction_id
        WHERE a.auction_id = ?
        GROUP BY a.auction_id`

	row := s.DB.QueryRow(auctionQuery, auctionID)
	err = row.Scan(
		&a.AuctionID, &a.TokenID, &a.Seller, &a.StartPrice, &a.EndTime,
		&winner, &finalPrice, &a.Ended, &a.HighestBid, &a.BidCount, &a.NFTContract)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Auction not found"})
		return
	} else if err != nil {
		log.Printf("Error querying auction details: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query auction details"})
		return
	}

	a.Winner = winner.String
	if finalPrice.Valid {
		if fp, e := strconv.ParseInt(finalPrice.String, 10, 64); e == nil {
			a.FinalPrice = fp
		}
	}

	// 2. 活躍拍賣：嘗試從 Redis 獲取實時最高出價，覆蓋 MySQL 的可能延遲數據
	if a.Ended == 0 {
		redisKey := fmt.Sprintf("auction:%d:highest_bid", a.AuctionID)
		bidMap, err := s.RDB.Client.HGetAll(s.RDB.Ctx, redisKey).Result()
		if err != nil {
			log.Printf("Redis HGetAll error for AuctionID %d: %v. Using MySQL fallback.", a.AuctionID, err)
			// Redis 失敗，繼續使用 MySQL 獲取的 a.HighestBid
		} else if amountStr, ok := bidMap["amount"]; ok && amountStr != "" {
			// Redis 數據存在且有效
			if highestBid, e := strconv.ParseInt(amountStr, 10, 64); e == nil {
				a.HighestBid = highestBid // 使用 Redis 提供的實時最高出價
			}
		}
	}

	// 3. 查詢出價歷史記錄
	bids := []Bid{}
	bidsQuery := `
        SELECT id, auction_id, bidder, amount, timestamp_utc 
        FROM bids 
        WHERE auction_id = ? 
        ORDER BY amount DESC`

	bidsRows, err := s.DB.Query(bidsQuery, auctionID)
	if err != nil {
		log.Printf("Error querying bids: %v", err)
	} else {
		defer bidsRows.Close()
		for bidsRows.Next() {
			var b Bid
			// 假設 timestamp_utc 字段用於 Bid.Timestamp
			err := bidsRows.Scan(&b.ID, &b.AuctionID, &b.Bidder, &b.Amount, &b.Timestamp)
			if err != nil {
				log.Printf("Error scanning bid row: %v", err)
				continue
			}
			bids = append(bids, b)
		}
	}

	response := AuctionDetailResponse{Auction: a, Bids: bids}
	c.JSON(http.StatusOK, response)
}

// handleMyNFTs 處理發起者查看自己擁有的所有 NFT (Mock)
func (s *Server) handleMyNFTs(c *gin.Context) {
	userID, _ := c.Get("userID") // 從中間件中獲取

	// ⚠️ 實際邏輯：呼叫外部服務/合約查詢擁有的 NFT
	c.JSON(http.StatusOK, gin.H{
		"owner": userID,
		"nfts": []map[string]interface{}{
			{"contract_address": "0xD0f38035f932Fd968b7803d26132762629e5CCAB", "token_id": 0, "name": "Mock NFT #0"},
			{"contract_address": "0xABC...", "token_id": 10, "name": "CryptoPunk-like #10"},
		},
	})
}

// handleCreateAuction 處理發起者創建一個拍賣 (Mock)
func (s *Server) handleCreateAuction(c *gin.Context) {
	userID, _ := c.Get("userID") // 從中間件中獲取
	var req CreateAuctionRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// ⚠️ 實際邏輯：驗證所有權，構建交易，並返回給前端簽名
	mockTxData := gin.H{
		"sender":  userID,
		"to":      "0x627bEd9E638C4158da5d79cA503006361F7c2b66", // Auction Contract
		"data":    "0x...encodedCreateAuctionData...",
		"summary": fmt.Sprintf("Create auction for NFT %s/%d with start price %d ETH", req.NFTContract, req.TokenID, req.StartPrice),
	}

	// 模擬將拍賣寫入資料庫
	// ⚠️ 在實際應用中，這一行為應由區塊鏈監聽器觸發
	_, err := s.DB.Exec(`
        INSERT INTO auctions (token_id, seller, start_price, end_time, ended) 
        VALUES (?, ?, ?, ?, 0)`,
		req.TokenID, userID, req.StartPrice, time.Now().Unix()+req.Duration)

	if err != nil {
		log.Printf("Error inserting mock auction: %v", err)
	}

	c.JSON(http.StatusAccepted, mockTxData)
}

// handleMyAuctions 處理發起者查看自己創建的所有拍賣
func (s *Server) handleMyAuctions(c *gin.Context) {
	userID, _ := c.Get("userID")
	status := c.Query("status")

	whereStatus := ""
	if status == "active" {
		whereStatus = "AND ended = 0"
	} else if status == "ended" {
		whereStatus = "AND ended = 1"
	}

	query := fmt.Sprintf(`
        SELECT 
            a.auction_id, a.token_id, a.seller, a.start_price, a.end_time, a.winner, a.final_price, a.ended,
            IFNULL(MAX(b.amount), a.start_price) AS highest_bid,
            COUNT(b.id) AS bid_count, 'MockContractAddress' AS nft_contract
        FROM auctions a
        LEFT JOIN bids b ON a.auction_id = b.auction_id
        WHERE a.seller = ? %s
        GROUP BY a.auction_id
        ORDER BY a.end_time DESC`, whereStatus)

	rows, err := s.DB.Query(query, userID)
	if err != nil {
		log.Printf("Error executing user auctions query: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query user's auctions"})
		return
	}
	defer rows.Close()

	auctions := []Auction{}
	for rows.Next() {
		var a Auction
		var winner, finalPrice sql.NullString

		err := rows.Scan(
			&a.AuctionID, &a.TokenID, &a.Seller, &a.StartPrice, &a.EndTime,
			&winner, &finalPrice, &a.Ended, &a.HighestBid, &a.BidCount, &a.NFTContract)

		if err != nil {
			log.Printf("Error scanning user auction row: %v", err)
			continue
		}

		a.Winner = winner.String
		if finalPrice.Valid {
			if fp, e := strconv.ParseInt(finalPrice.String, 10, 64); e == nil {
				a.FinalPrice = fp
			}
		}

		auctions = append(auctions, a)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rows iteration error for user auctions"})
		return
	}

	c.JSON(http.StatusOK, auctions)
}

// StartServer 設置 Gin 路由並啟動 HTTP 服務器
// 接受 RedisClient 實例
func StartServer(cfg *config.Config, db *sql.DB, rdb *storage.RedisClient) {
	// 創建服務器實例並傳入 DB 連接和 Redis 客戶端
	server := NewServer(db, rdb)

	// 創建 Gin 引擎
	r := gin.Default()

	// --- V1 API Group ---
	v1 := r.Group("/api/v1")
	{
		// 認證路由
		auth := v1.Group("/auth")
		{
			auth.POST("/connect", server.handleAuthConnect)
		}

		// 公共數據路由
		v1.GET("/stats", server.handleStats)
		v1.GET("/auctions", server.handleAuctions)
		// 查詢單個拍賣詳情，GET /api/v1/auctions/123
		v1.GET("/auctions/:auctionId", server.handleAuctionDetail)

		// 需要認證的路由
		authenticated := v1.Group("/")
		authenticated.Use(server.AuthMiddleware()) // 使用認證中間件
		{
			// 發起者 (Seller) 相關
			authenticated.GET("/me/nfts", server.handleMyNFTs)
			authenticated.POST("/auctions", server.handleCreateAuction)
			authenticated.GET("/me/auctions", server.handleMyAuctions)

			// 出價者 (Bidder) 相關
			authenticated.GET("/me/bids", server.handleMyBids)
			authenticated.POST("/auctions/:auctionId/bid", server.handleSubmitBid)
			authenticated.POST("/auctions/:auctionId/finalize", server.handleFinalizeAuction)
		}
	}

	// 啟動服務器
	log.Printf("🌐 Starting REST API server on http://localhost:%s", cfg.HTTPPort)
	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
