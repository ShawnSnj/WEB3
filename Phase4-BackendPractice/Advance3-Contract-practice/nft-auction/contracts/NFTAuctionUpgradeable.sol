// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

// 引入 ERC721 接口
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";

// 引入 UUPS 升級相關庫
// 關鍵修正：確保所有 UUPS 相關的基礎合約都從 'contracts-upgradeable' 路徑導入。
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol"; // <-- 已修正為升級版路徑
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol"; // <-- 已修正為升級版路徑
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";
/**
 * @title NFTAuction
 * @dev UUPS 可升級版本的 NFT 拍賣合約。
 * 繼承 Initializable、UUPSUpgradeable 和 OwnableUpgradeable。
 */
contract NFTAuction is Initializable, UUPSUpgradeable, OwnableUpgradeable {
    // 確保使用 Address 庫中的 SafeTransferLib 功能
    // 註解掉 'using Address for address payable;'，因為 Solidity 0.8+ 已內建安全轉帳功能。

    // 拍賣狀態結構體 - 儲存順序對應儲存槽位，不能更改
    struct Auction {
        address payable seller;
        IERC721 nftContract;
        uint256 tokenId;
        uint256 startPrice;
        uint256 highestBid;
        address payable highestBidder;
        uint256 endTime;
        bool ended;
    }

    // 儲存變量 - 順序不能更改 (Storage Layout Preservation)
    mapping(uint256 => Auction) public auctions;
    uint256 public nextAuctionId;
    mapping(uint256 => mapping(address => uint256)) public fundsToWithdraw;

    // 最小出價增幅 (Constant 變量不會佔用儲存槽位，是安全的)
    uint256 public constant MIN_BID_INCREMENT = 10000000000000000; // 0.01 Ether

    // ----------------------------------------------------
    // 初始化函數 (替代 Constructor)
    // ----------------------------------------------------

    /**
     * @notice 合約初始化函數，在部署時呼叫一次。
     * @dev 必須使用 initializer 修飾符。
     */
    function initialize() public initializer {
        // 1. 初始化 Ownable (設置呼叫者為擁有者)
        __Ownable_init(msg.sender);
        // 2. 初始化 UUPS (啟用升級檢查)
        __UUPSUpgradeable_init();

        // 3. 設置業務邏輯初始狀態
        nextAuctionId = 1;
    }

    // ----------------------------------------------------
    // UUPS 升級授權
    // ----------------------------------------------------

    /**
     * @notice 實作 UUPS 升級的授權檢查。
     * @dev 只有合約擁有者 (owner) 可以授權升級。
     */
    function _authorizeUpgrade(
        address newImplementation
    ) internal override onlyOwner {}

    // ----------------------------------------------------
    // 事件 (Events)
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
        // 核心要求：賣家必須先授權給拍賣合約（即 Proxy 地址）
        // 由於合約地址在 createAuction 之前會被設定為 proxy 的地址，address(this) 指向 proxy
        IERC721(_nftContract).transferFrom(msg.sender, address(this), _tokenId);

        // 創建拍賣結構體
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

        // 發出事件供後端監聽
        emit AuctionCreated(
            auctionId,
            msg.sender,
            _nftContract,
            _tokenId,
            _startPrice,
            auctions[auctionId].endTime
        );
    }

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
        require(
            msg.sender != auction.seller,
            "Seller cannot bid on their own auction."
        );

        // 2. 驗證出價金額
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

        // --- 3. 處理前一位出價者的退款 (Pull Over Push 模式) ---
        address payable previousBidder = auction.highestBidder;
        uint256 previousBid = auction.highestBid;

        if (previousBidder != address(0)) {
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

        // 修正：使用 low-level call 函數進行安全轉帳
        (bool success, ) = payable(msg.sender).call{value: amount}("");
        require(success, "ETH transfer failed during withdrawal.");
    }

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
            auction.nftContract.safeTransferFrom(
                address(this),
                winner,
                auction.tokenId
            );

            // // 2.2. 轉移 ETH 給賣家
            // payable(auction.seller).sendValue(finalPrice);
            // 修正：使用 low-level call 函數進行安全轉帳
            (bool success, ) = payable(auction.seller).call{value: finalPrice}(
                ""
            );
            require(success, "ETH transfer failed during withdrawal.");

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

            // 2.2. 發出失敗事件
            emit AuctionEnded(
                _auctionId,
                address(0), // 無贏家
                0, // 最終價格為 0
                auction.seller,
                block.timestamp
            );
        }
    }
}
