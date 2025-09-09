pragma solidity ^0.8.20;

import "./Auction.sol";

contract AuctionFactory {
    address[] public auctions;
    function createAuction(
        address _nft,
        uint256 _tokenId,
        address _erc20,
        address _priceFeed,
        uint256 _biddingTime
    ) external {
        Auction newAuction = new Auction(
            _nft,
            _tokenId,
            _erc20,
            _priceFeed,
            _biddingTime
        );
        auctions.push(address(newAuction));
    }

    function getAuctions() external view returns(address[] memory){
        return auctions;
    }
}
