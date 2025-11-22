// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

// 引入 V1 合約的介面，確保 V2 繼承所有 V1 的儲存佈局和邏輯
import {NFTAuctionUpgradeable} from "./NFTAuctionUpgradeable.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721.sol"; // 需要用到 IERC721

/**
 * @title NFTAuctionUpgradeableV2
 * @dev NFTAuction 合約的 V2 升級實作。
 * 添加了贏家轉移 NFT 給新擁有者的功能。
 */
contract NFTAuctionUpgradeableV2 is NFTAuctionUpgradeable {
    // ----------------------------------------------------
    // 新增事件 (Events)
    // ----------------------------------------------------

    /**
     * @notice 當贏家將贏得的 NFT 轉移給新擁有者時觸發。
     * @param auctionId 拍賣 ID。
     * @param previousOwner 原始贏家地址 (即調用者)。
     * @param newOwner 最終接收 NFT 的地址。
     * @param tokenId 轉移的 NFT ID。
     */
    event WinnedNFTTransferred(
        uint256 indexed auctionId,
        address indexed previousOwner,
        address indexed newOwner,
        uint256 tokenId
    );

    // ----------------------------------------------------
    // V2 新增邏輯
    // ----------------------------------------------------

    /**
     * @notice 允許拍賣贏家將他們贏得的 NFT 轉移給新的擁有者。
     * @dev 此函數由贏家調用。在調用前，贏家必須先授權 (approve) 此拍賣合約來轉移該 NFT。
     * @param _auctionId 相關的拍賣 ID。
     * @param _newOwner 接收 NFT 的新地址。
     */
    function transferWinnedNFT(uint256 _auctionId, address _newOwner) public {
        Auction storage auction = auctions[_auctionId];

        // 1. 驗證拍賣狀態
        require(auction.seller != address(0), "Auction does not exist.");
        // 拍賣必須已結束，且有人得標
        require(auction.ended, "Auction is still active.");
        require(auction.highestBidder != address(0), "Auction had no winner.");

        // 2. 驗證調用者身份
        // 只有最高出價者 (贏家) 才能調用此函數。
        require(
            msg.sender == auction.highestBidder,
            "Only the winner can initiate this transfer."
        );
        require(_newOwner != address(0), "New owner address cannot be zero.");

        // 3. 執行 NFT 轉移
        // 贏家 (msg.sender) 必須先對此合約 (address(this)) 進行授權，
        // 才能讓此合約代為轉移 NFT。
        // 注意：這裡假設 NFT 目前在贏家手上，因為 endAuction 已經將 NFT 轉給了贏家。
        auction.nftContract.safeTransferFrom(
            msg.sender, // 從贏家轉出 (msg.sender 必須是當前 NFT 擁有者)
            _newOwner, // 轉入新擁有者
            auction.tokenId
        );

        // 4. 發出事件
        emit WinnedNFTTransferred(
            _auctionId,
            msg.sender,
            _newOwner,
            auction.tokenId
        );
    }

    // ----------------------------------------------------
    // UUPS 升級規範 (已修正)
    // ----------------------------------------------------
}
