pragma solidity ^0.8.20;

//Interact with ERC-20 tokens Payments, balances, allowances
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
//Base contract for NFTs Minting and transferring NFTs
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

contract Auction {
    address public seller;
    address public highestBidder;
    uint256 public highestBidUSD;
    uint256 public endTime;

    ERC721 public nft;
    uint256 public tokenId;
    IERC20 public erc20; // ERC-20 token used for bidding
    AggregatorV3Interface public priceFeed; //// Chainlink feed (ETH/USD or ERC20/USD)
    constructor(
        address _nft,
        uint256 _tokenId,
        address _erc20,
        address _priceFeed,
        uint256 _biddingTime
    ) {
        seller = msg.sender;
        nft = ERC721(_nft);
        tokenId = _tokenId;
        erc20 = IERC20(_erc20);
        priceFeed = AggregatorV3Interface(_priceFeed);
        endTime = block.timestamp + _biddingTime;

        //Transfer the NFT to the auction contract
        nft.transferFrom(msg.sender, address(this), tokenId);
    }

    function bidWithETH() external payable{
        require(block.timestamp < endTime, "Auction ended");
        uint256 bidUSD = convertToUSD(msg.value);
        require(bidUSD > highestBidUSD, "Bid too low");

        if(highestBidder != address(0)){
            payable(highestBidder).transfer(address(this).balance - msg.value);
        }
        highestBidder = msg.sender;
        highestBidUSD = bidUSD;
    }

    function bidWithERC20(uint256 _amount) external{
        require(block.timestamp < endTime, "Auction ended");
        uint256 bidUSD = convertToUSD(_amount);
        require(bidUSD > highestBidUSD, "Bid too low");

        erc20.transferFrom(msg.sender,address(this),_amount);

        if(highestBidder != address(0)){
            erc20.transfer(highestBidder,_amount);
        }
        highestBidder = msg.sender;
        highestBidUSD = bidUSD;
    }

    function endAuction() external{
        require(block.timestamp >= endTime, "Auction not ended");
        require(msg.sender == seller, "Only seller can end");

        nft.transferFrom(address(this),highestBidder,tokenId);
        payable(seller).transfer(address(this).balance);
    }

    function convertToUSD(uint256 _amount) internal view returns (uint256) {
        (, int256 price, , , ) = priceFeed.latestRoundData();
        // Assuming price feed returns price with 8 decimals
        return (_amount * uint256(price)) / 1e8;

}
