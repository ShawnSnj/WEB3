// contracts/NFTAuction.sol
pragma solidity ^0.8.28;
// 引入 ERC721 接口，用於轉移 NFT
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
// 引入 SafeTransferLib，用於安全的 ETH 轉賬
import "@openzeppelin/contracts/utils/Address.sol";

contract NFTAuction {
    // 使用 SafeTransferLib 的 sendValue 確保安全轉賬
    using Address for address payable;
    address public owner;

    // 拍賣狀態結構體
    struct Auction {
        address payable seller; // 賣家地址
        IERC721 nftContract; // NFT 合約地址
        uint256 tokenId; // NFT ID
        uint256 startPrice; // 起始價格/底價
        uint256 highestBid; // 當前最高出價
        address payable highestBidder; // 當前最高出價者
        uint256 endTime; // 拍賣結束時間戳
        bool ended; // 拍賣是否已結束
    }

    // 儲存所有拍賣，使用 auctionId 作為鍵
    mapping(uint256 => Auction) public auctions;
    uint256 public nextAuctionId = 1;

    // ----------------------------------------------------
    // 事件 (Events) - 後端 Go 服務需要監聽這些事件！
    // ----------------------------------------------------

    event AuctionCreated(
        uint256 auctionId,
        address indexed seller,
        address nftContract,
        uint256 tokenId,
        uint256 startPrice,
        uint256 endTime
    );

    event BidPlaced(
        uint256 indexed auctionId,
        address indexed bidder,
        uint256 amount,
        address refundedBidder,
        uint256 refundedAmount
    );

    event AuctionEnded(
        uint256 indexed auctionId,
        address winner,
        uint256 finalPrice,
        address seller,
        uint256 time
    );

    // ----------------------------------------------------
    // 核心邏輯函數
    // ----------------------------------------------------

    function createAuction(
        address _nftContract,
        uint256 _tokenId,
        uint256 _startPrice,
        uint256 _duration
    ) public {
        // 1. 基本檢查 (省略... e.g., duration > 0)

        // 2. 核心要求：賣家必須先授權給拍賣合約
        IERC721(_nftContract).transferFrom(msg.sender, address(this), _tokenId);

        // 3. 創建拍賣結構體
        uint256 auctionId = nextAuctionId;
        auctions[auctionId] = Auction({
            seller: payable(msg.sender),
            nftContract: IERC721(_nftContract),
            tokenId: _tokenId,
            startPrice: _startPrice,
            highestBid: _startPrice,
            highestBidder: payable(address(0)),
            endTime: block.timestamp + _duration,
            ended: false
        });

        nextAuctionId++;

        // 4. 發出事件供後端監聽
        emit AuctionCreated(
            auctionId,
            msg.sender,
            _nftContract,
            _tokenId,
            _startPrice,
            auctions[auctionId].endTime
        );
    }
    modifier onlyOwner() {
        require(msg.sender == owner, "");
        _;
    }
    //it tracks the refund money in Auction
    mapping(uint256 => mapping(address => uint256)) public fundsToWithdraw;
    //the minimal bid price increment
    uint256 public constant MIN_BID_INCREMENT = 10000000000000000; // 0.01 Ether in Wei
    // [TODO] 實現 placeBid 函數

    /**
     * @notice 參與拍賣並出價。
     * @param _auctionId 要出價的拍賣 ID。
     */
    function placeBid(uint256 _auctionId) public payable {
        Auction storage auction = auctions[_auctionId];

        // 1. 驗證拍賣狀態
        require(auction.seller != address(0), "Auction does not exist.");
        require(!auction.ended, "Auction has already ended.");
        require(
            block.timestamp < auction.endTime,
            "Auction time is over. Call endAuction."
        );

        // 2. 驗證出價金額 (msg.value 是用戶隨函數調用發送的 ETH 數量)
        require(
            msg.value > auction.highestBid,
            "Bid must be greater than current highest bid."
        );

        // 確保新出價比當前最高價至少高出最小增幅
        if (auction.highestBidder != address(0)) {
            require(
                msg.value >= auction.highestBid + MIN_BID_INCREMENT,
                "Bid must increase by at least 0.01 Ether."
            );
        } else {
            // 如果是第一個出價，必須至少達到起始價
            require(
                msg.value >= auction.startPrice,
                "Bid must meet the start price."
            );
        }

        // --- 3. 處理前一位出價者的退款 ---
        address payable previousBidder = auction.highestBidder;
        uint256 previousBid = auction.highestBid;

        if (previousBidder != address(0)) {
            // 由於直接在 `placeBid` 函數中發送 ETH 給用戶有風險 (重入攻擊),
            // 最佳實踐是使用 "Pull Over Push" 模式，讓用戶自己提取退款。
            // 我們將資金記錄在 fundsToWithdraw 映射中。
            fundsToWithdraw[_auctionId][previousBidder] += previousBid;
        }

        // --- 4. 更新拍賣狀態 ---
        auction.highestBid = msg.value;
        auction.highestBidder = payable(msg.sender);

        // 5. 發出事件供後端監聽
        emit BidPlaced(
            _auctionId,
            msg.sender,
            msg.value,
            previousBidder,
            previousBid
        );
    }

    /**
     * @notice 允許用戶提取他們在拍賣中被取代的出價。
     * @param _auctionId 拍賣 ID。
     */
    function withdraw(uint256 _auctionId) public {
        uint256 amount = fundsToWithdraw[_auctionId][msg.sender];
        require(amount > 0, "No funds to withdraw.");

        fundsToWithdraw[_auctionId][msg.sender] = 0; // 清除記錄，防止重複提款

        // 安全地將 ETH 發送回用戶
        payable(msg.sender).sendValue(amount);
    }
    // [TODO] 實現 endAuction 函數

    /**
     * @notice 結束拍賣並結算結果。
     * 任何人都可以調用此函數，但只有在時間結束後才能執行。
     * @param _auctionId 要結束的拍賣 ID。
     */
    function endAuction(uint256 _auctionId) public {
        Auction storage auction = auctions[_auctionId];

        // 1. 驗證拍賣狀態和時間
        require(auction.seller != address(0), "Auction does not exist.");
        require(block.timestamp >= auction.endTime, "Auction is still active.");
        require(!auction.ended, "Auction has already ended.");

        // 標記拍賣為已結束，防止重複執行
        auction.ended = true;

        // ----------------------------------------------------
        // 2. 結算邏輯
        // ----------------------------------------------------

        if (auction.highestBidder != address(0)) {
            // 情況 A: 有人出價 (拍賣成功)

            address winner = auction.highestBidder;
            uint256 finalPrice = auction.highestBid;

            // 2.1. 轉移 NFT 給最高出價者 (贏家)
            // 拍賣合約在 createAuction 時已取得 NFT 的控制權
            auction.nftContract.safeTransferFrom(
                address(this),
                winner,
                auction.tokenId
            );

            // 2.2. 轉移 ETH 給賣家
            // 拍賣合約目前持有最高出價的 ETH (msg.value in placeBid)
            // 使用 sendValue 安全地轉移資金
            auction.seller.sendValue(finalPrice);

            // 2.3. 發出成功事件
            emit AuctionEnded(
                _auctionId,
                winner,
                finalPrice,
                auction.seller,
                block.timestamp
            );
        } else {
            // 情況 B: 沒有人出價 (拍賣失敗)

            // 2.1. 將 NFT 退還給賣家
            auction.nftContract.safeTransferFrom(
                address(this),
                auction.seller,
                auction.tokenId
            );

            // 2.2. 發出失敗事件 ( winner 設為 address(0) 且 finalPrice 設為 0 )
            emit AuctionEnded(
                _auctionId,
                address(0), // 無贏家
                0, // 最終價格為 0
                auction.seller,
                block.timestamp
            );
        }

        // 3. 清理: 拍賣結束後，相關的資金可以從 fundsToWithdraw 提取，無需額外操作。
    }
}
