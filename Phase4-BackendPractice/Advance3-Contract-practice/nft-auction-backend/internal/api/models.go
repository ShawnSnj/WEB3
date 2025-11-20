package api

// Auction 結構體用於拍賣概覽
type Auction struct {
	AuctionID   int64   `json:"auction_id"`
	TokenID     int64   `json:"token_id"`
	Seller      string  `json:"seller"`
	StartPrice  float64 `json:"start_price"`
	EndTime     int64   `json:"end_time"`
	Winner      string  `json:"winner"`
	FinalPrice  int64   `json:"final_price"` // 使用 int64 模擬合約中的大數字
	Ended       int     `json:"ended"`
	HighestBid  int64   `json:"highest_bid"` // 使用 int64 模擬合約中的大數字
	BidCount    int     `json:"bid_count"`
	NFTContract string  `json:"nft_contract"`
}

// Bid 結構體用於出價記錄
type Bid struct {
	ID        int64  `json:"id"`
	AuctionID int64  `json:"auction_id"`
	Bidder    string `json:"bidder"`
	Amount    int64  `json:"amount"` // 使用 int64 模擬合約中的大數字
	Timestamp int64  `json:"timestamp"`
}

// StatsResponse 結構體用於平台統計
type StatsResponse struct {
	TotalAuctions int `json:"total_auctions"`
	TotalBids     int `json:"total_bids"`
	TotalUsers    int `json:"total_users"`
}

// CreateAuctionRequest 結構體用於發起拍賣請求
type CreateAuctionRequest struct {
	NFTContract string `json:"nft_contract" binding:"required"`
	TokenID     int64  `json:"token_id" binding:"required"`
	StartPrice  int64  `json:"start_price" binding:"required"`
	Duration    int64  `json:"duration" binding:"required"` // 拍賣持續時間（秒），**這是關鍵字段**
}

// BidRequest 結構體用於出價請求
type BidRequest struct {
	Amount int64 `json:"amount" binding:"required"`
}

// AuctionDetailResponse 結構體用於單個拍賣詳情
type AuctionDetailResponse struct {
	Auction Auction `json:"auction"`
	Bids    []Bid   `json:"bids"`
}
