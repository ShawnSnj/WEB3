pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

// 自定義 ERC20 interface，加入 decimals()
interface IERC20WithDecimals is IERC20 {
    function decimals() external view returns (uint8);
}

contract Auction {
    struct Bid {
        address bidder;
        uint256 amountUSD;
        address payToken;
        uint256 rawAmount;
    }

    address public nftAddress;
    uint256 public tokenId;
    address public seller;
    uint256 public endTime;
    bool public ended;
    Bid public highestBid;

    AggregatorV3Interface internal ethUsdPriceFeed;

    mapping(address => AggregatorV3Interface) public erc20UsdpriceFeed;

    constructor(
        address _nftAddress,
        uint256 _tokenId,
        uint256 _biddingTime,
        address _ethUsdPriceFeed
    ) {
        nftAddress = _nftAddress;
        tokenId = _tokenId;
        seller = msg.sender;
        endTime = block.timestamp + _biddingTime;
        ethUsdPriceFeed = AggregatorV3Interface(_ethUsdPriceFeed);
        // NFT 要先被批准給這個合約或直接 transfer
        IERC721(nftAddress).transferFrom(msg.sender, address(this), tokenId);
    }

    function addERC20PriceFeed(address token, address priceFeed) external {
        require(msg.sender == seller, "Only seller can add price feed");
        erc20UsdpriceFeed[token] = AggregatorV3Interface(priceFeed);
    }

    function _getUSDValueETH(
        uint256 ethAmount
    ) internal view returns (uint256) {
        (, int price, , , ) = ethUsdPriceFeed.latestRoundData();
        require(price > 0, "Invalid ETH/USD data");
        // price 的 decimals 通常是 8（在 ETH/USD feed） → 10^8
        // 假設 price 是 “ETH per USD * 10^8”，需要調整
        // 假設 ethAmount 是以 wei（10^18）

        // USD value = ethAmount * price / (10^8 * 10^18) → 看你要怎麼去 scale
        // 這裡示意：
        return (ethAmount * uint256(price)) / 1e26;
    }

    function _getUSDValueERC20(
        address token,
        uint256 tokenAmount
    ) internal view returns (uint256) {
        AggregatorV3Interface priceFeed = erc20UsdpriceFeed[token];
        require(
            address(priceFeed) != address(0),
            "Price feed not available for this token"
        );
        (, int price, , , ) = priceFeed.latestRoundData();
        require(price > 0, "Invalid token/USD data");
        // 假設 price 的 decimals 是 8（在某些 feed） → 10^8
        // 假設 tokenAmount 是以該 ERC20 的最小單位（decimals）

        // USD value = tokenAmount * price / (10^8 * 10^decimals) → 看你要怎麼去 scale
        // 這裡示意：
         uint8 decimals = IERC20WithDecimals(token).decimals();
        return (tokenAmount * uint256(price)) / (10 ** (8 + decimals));
    }

    function _refundHighest() internal {
        if (highestBid.bidder != address(0)) {
            if (highestBid.payToken == address(0)) {
                payable(highestBid.bidder).transfer(highestBid.rawAmount);
            } else {
                IERC20(highestBid.payToken).transfer(
                    highestBid.bidder,
                    highestBid.rawAmount
                );
            }
        }
    }

    function bidWithETH() external payable {
        require(block.timestamp < endTime, "Auction already ended");
        uint256 bidAmountUSD = _getUSDValueETH(msg.value);
        require(
            bidAmountUSD > highestBid.amountUSD,
            "There already is a higher or equal bid"
        );

        _refundHighest();

        highestBid = Bid({
            bidder: msg.sender,
            amountUSD: bidAmountUSD,
            payToken: address(0),
            rawAmount: msg.value
        });
    }

    function bidWithERC20(address token, uint256 amount) external {
        require(block.timestamp < endTime, "Auction ended");
        require(amount > 0, "Invalid bid amount");
        require(
            erc20UsdpriceFeed[token] != AggregatorV3Interface(address(0)),
            "Unsupported token"
        );

        uint256 bidAmountUSD = _getUSDValueERC20(token, amount);
        require(
            bidAmountUSD > highestBid.amountUSD,
            "There already is a higher or equal bid"
        );

        _refundHighest();

        highestBid = Bid({
            bidder: msg.sender,
            amountUSD: bidAmountUSD,
            payToken: token,
            rawAmount: amount
        });
    }
    function endAuction() external {
        require(block.timestamp >= endTime, "Auction not yet ended");
        require(!ended, "Auction already ended");
        ended = true;

        if (highestBid.bidder != address(0)) {
            // 有人出價
            IERC721(nftAddress).transferFrom(
                address(this),
                highestBid.bidder,
                tokenId
            );
            if (highestBid.payToken == address(0)) {
                payable(seller).transfer(highestBid.rawAmount);
            } else {
                IERC20(highestBid.payToken).transfer(
                    seller,
                    highestBid.rawAmount
                );
            }
        } else {
            // 沒有人出價，退回給賣家
            IERC721(nftAddress).transferFrom(address(this), seller, tokenId);
        }
    }
}
