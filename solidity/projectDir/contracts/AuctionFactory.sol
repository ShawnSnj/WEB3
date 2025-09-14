pragma solidity ^0.8.28;

import "./Auction.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

contract AuctionFactory is Initializable,UUPSUpgradeable, OwnableUpgradeable {
    address[] public allAuctions;
    address public ethUsdPriceFeed;

    event AuctionCreated(
        address indexed auctionAddress,
        address indexed nftAddress,
        uint256 indexed tokenId,
        uint256 duration
    );

    function initialize(address _ethUsdPriceFeed) public initializer {
        __Ownable_init();
        __UUPSUpgradeable_init();
        ethUsdPriceFeed = _ethUsdPriceFeed;
    }

    function _authorizeUpgrade(
        address newImplementation
    ) internal override onlyOwner {}

    function createAuction(
        address _nftAddress,
        uint256 _tokenId,
        uint256 _biddingTime,
        address _ethUsdPriceFeed
    ) external returns (address) {
        Auction newAuction = new Auction(
            _nftAddress,
            _tokenId,
            _biddingTime,
            _ethUsdPriceFeed
        );
        allAuctions.push(address(newAuction));
        emit AuctionCreated(
            address(newAuction),
            _nftAddress,
            _tokenId,
            block.timestamp + _biddingTime
        );
        return address(newAuction);
    }

    function getAllAuctions() external view returns (address[] memory) {
        return allAuctions;
    }
}