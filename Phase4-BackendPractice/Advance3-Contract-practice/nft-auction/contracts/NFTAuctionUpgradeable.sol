// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

// OpenZeppelin Contracts for standard interfaces and utilities
import "@openzeppelin/contracts/token/ERC721/IERC721.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

// OpenZeppelin Upgradeable Contracts for UUPS Proxy
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

// ----------------------------------------------------
// EXTENDED INTERFACES
// ----------------------------------------------------

/**
 * @dev Standard ERC20 interface (used by SafeERC20) extended to include the commonly used decimals() function.
 */
interface IExtendedERC20 is IERC20 {
    function decimals() external view returns (uint8);
}

// Chainlink Aggregator Interface (Used for fetching real-time token prices)
interface AggregatorV3Interface {
    function latestRoundData()
        external
        view
        returns (
            uint80 roundId,
            int256 answer,
            uint256 startedAt,
            uint256 updatedAt,
            uint80 answeredInRound
        );
}

/**
 * @title NFTAuctionUpgradeable
 * @dev UUPS upgradeable contract supporting multi-token bidding and USD-pegged pricing
 * using Chainlink Oracles. It emits USD-based values for off-chain calculation (e.g., scoring/airdrop).
 */
contract NFTAuctionUpgradeable is
    Initializable,
    UUPSUpgradeable,
    OwnableUpgradeable
{
    using SafeERC20 for IERC20;

    // Constants for token and price representation
    address public constant ETH_ADDRESS = address(0);
    uint256 public constant USD_DECIMALS = 8;
    uint256 public constant ETH_DECIMALS = 18;

    // Auction data structure
    struct Auction {
        address payable seller;
        IERC721 nftContract;
        uint256 tokenId;
        uint256 highestBidUSDValue; // Core price stored in USD (8 decimals)
        address highestBidToken; // Token used for the highest bid (0x0 for ETH)
        uint256 highestBidAmount; // Raw token amount of the highest bid
        address payable highestBidder;
        uint256 endTime;
        bool ended;
    }

    // Storage variables
    mapping(uint256 => Auction) public auctions;
    uint256 public nextAuctionId;
    // Funds for withdrawal: [User Address] -> [Token Address (0x0 for ETH)] -> Amount
    mapping(address => mapping(address => uint256)) public fundsToWithdraw;

    // Oracle configuration
    AggregatorV3Interface public ethUsdOracle;
    // Maps ERC20 token addresses to their respective USD Oracle addresses
    mapping(address => AggregatorV3Interface) public tokenToOracle;
    uint256 public minBidIncrementUSD; // Minimum bid increase (USD, 8 decimals)

    // ----------------------------------------------------
    // Events
    // ----------------------------------------------------

    event AuctionCreated(
        uint256 indexed auctionId,
        address indexed seller,
        address nftContract,
        uint256 tokenId,
        uint256 startPriceUSD, // USD value (8 decimals)
        uint256 endTime
    );

    event BidPlaced(
        uint256 indexed auctionId,
        address indexed bidder,
        uint256 usdValue, // USD value of the bid
        address bidToken, // Token address (0x0 for ETH)
        uint256 bidAmount // Raw token amount
    );

    event AuctionEnded(
        uint256 indexed auctionId,
        address winner,
        uint256 finalPriceUSD, // Final USD value
        address seller,
        uint256 time
    );

    // ----------------------------------------------------
    // Initialization and Configuration
    // ----------------------------------------------------

    /**
     * @notice Initializes the contract.
     * @param _ethUsdOracleAddress Address of the ETH/USD Chainlink Oracle.
     * @param _minBidIncrementUSD Minimum required bid increase in USD (8 decimals).
     */
    function initialize(
        address _ethUsdOracleAddress,
        uint256 _minBidIncrementUSD
    ) public initializer {
        __Ownable_init(msg.sender);
        __UUPSUpgradeable_init();

        nextAuctionId = 1;
        ethUsdOracle = AggregatorV3Interface(_ethUsdOracleAddress);
        minBidIncrementUSD = _minBidIncrementUSD;
    }

    /**
     * @notice Sets the USD oracle address for a specific ERC20 token.
     */
    function setTokenOracle(address _token, address _oracle) public onlyOwner {
        require(_token != ETH_ADDRESS, "Cannot set oracle for native ETH.");
        tokenToOracle[_token] = AggregatorV3Interface(_oracle);
    }

    /**
     * @notice Sets the minimum required bid increment in USD.
     */
    function setMinBidIncrementUSD(uint256 _incrementUSD) public onlyOwner {
        minBidIncrementUSD = _incrementUSD;
    }

    // ----------------------------------------------------
    // UUPS Upgrade Authorization
    // ----------------------------------------------------

    function _authorizeUpgrade(
        address newImplementation
    ) internal override onlyOwner {}

    // ----------------------------------------------------
    // Internal Helpers (Price Conversion)
    // ----------------------------------------------------

    /**
     * @notice Fetches the latest price from a Chainlink oracle.
     */
    function getLatestPrice(
        AggregatorV3Interface _oracle
    ) public view returns (int256) {
        // We only care about the price (answer)
        (, int256 price, , , ) = _oracle.latestRoundData();
        require(price > 0, "Oracle price is invalid.");
        return price;
    }

    /**
     * @notice Converts a raw token amount (ETH or ERC20) into its USD value (8 decimals).
     * @param _token Token address (0x0 for ETH).
     * @param _amount Raw token amount.
     * @return usdValue The USD value (8 decimals).
     */
    function getTokenUSDValue(
        address _token,
        uint256 _amount
    ) public view returns (uint256 usdValue) {
        if (_token == ETH_ADDRESS) {
            int256 ethPrice = getLatestPrice(ethUsdOracle);
            // Conversion: (Amount * Price) / 10**(ETH_DECIMALS - USD_DECIMALS)
            // (18 decimals * 8 decimals) / 10^10 -> Result is 8 decimals
            usdValue = uint256(
                (_amount * uint256(ethPrice)) /
                    (10 ** (ETH_DECIMALS - USD_DECIMALS))
            );
        } else {
            // ERC20 token
            AggregatorV3Interface oracle = tokenToOracle[_token];
            require(address(oracle) != address(0), "Oracle not set for token.");

            int256 tokenPrice = getLatestPrice(oracle);

            // Use IExtendedERC20 to call decimals()
            uint256 tokenDecimals = IExtendedERC20(_token).decimals();

            // Conversion: (Amount * Price) / 10**tokenDecimals
            // Result is 8 decimals
            usdValue = uint256(
                (_amount * uint256(tokenPrice)) / (10 ** tokenDecimals)
            );
        }

        require(usdValue > 0, "Calculated USD value is zero.");
        return usdValue;
    }

    // ----------------------------------------------------
    // Core Auction Logic
    // ----------------------------------------------------

    /**
     * @notice Creates a new NFT auction.
     * Requires the seller to approve the NFT to this contract address beforehand.
     */
    function createAuction(
        address _nftContract,
        uint256 _tokenId,
        uint256 _startPriceUSD,
        uint256 _duration
    ) public {
        // 1. Transfer NFT ownership to the contract (Proxy address)
        IERC721(_nftContract).transferFrom(msg.sender, address(this), _tokenId);

        require(_startPriceUSD > 0, "Start price must be positive.");
        require(_duration > 0, "Duration must be positive.");

        uint256 auctionId = nextAuctionId;

        // 2. Create the auction structure
        auctions[auctionId] = Auction({
            seller: payable(msg.sender),
            nftContract: IERC721(_nftContract),
            tokenId: _tokenId,
            highestBidUSDValue: _startPriceUSD, // Initial USD price
            highestBidToken: ETH_ADDRESS, // Placeholder
            highestBidAmount: 0,
            highestBidder: payable(address(0)), // No bidder yet
            endTime: block.timestamp + _duration,
            ended: false
        });

        nextAuctionId++;

        // 3. Emit event
        emit AuctionCreated(
            auctionId,
            msg.sender,
            _nftContract,
            _tokenId,
            _startPriceUSD,
            auctions[auctionId].endTime
        );
    }

    /**
     * @notice Places a bid on an auction using ETH or an approved ERC20 token.
     * If using an ERC20 token, the bidder must approve the token to this contract address beforehand.
     */
    function placeBid(
        uint256 _auctionId,
        address _token,
        uint256 _amount
    ) public payable {
        Auction storage auction = auctions[_auctionId];
        // Determine the actual bid amount (msg.value for ETH)
        uint256 bidAmount = (_token == ETH_ADDRESS) ? msg.value : _amount;

        // 1. Validate auction state
        require(auction.seller != address(0), "Auction does not exist.");
        require(!auction.ended, "Auction has already ended.");
        require(
            block.timestamp < auction.endTime,
            "Auction time is over. Call endAuction."
        );
        require(msg.sender != auction.seller, "Seller cannot bid.");
        require(bidAmount > 0, "Bid amount must be positive.");

        // 2. Handle ETH vs ERC20 input checks
        if (_token == ETH_ADDRESS) {
            require(msg.value == bidAmount, "ETH amount mismatch.");
        } else {
            require(msg.value == 0, "Do not send ETH with token bid.");
        }

        // 3. Get the new bid's USD value (Core logic)
        uint256 newBidUSDValue = getTokenUSDValue(_token, bidAmount);

        // 4. Validate the new bid's USD value
        // Bid must be >= current highest USD value + minimum increment USD
        uint256 minRequiredUSD = auction.highestBidUSDValue +
            minBidIncrementUSD;
        require(
            newBidUSDValue >= minRequiredUSD,
            "Bid must increase by at least the min USD increment."
        );

        // 5. Refund previous bidder (record to fundsToWithdraw)
        address previousBidder = auction.highestBidder;
        address previousToken = auction.highestBidToken;
        uint256 previousAmount = auction.highestBidAmount;

        if (previousBidder != address(0)) {
            // Credit the previous bidder's funds for withdrawal
            fundsToWithdraw[previousBidder][previousToken] += previousAmount;
        }

        // 6. Transfer funds to the contract
        if (_token != ETH_ADDRESS) {
            // ERC-20 token requires transferFrom
            IERC20(_token).safeTransferFrom(
                msg.sender,
                address(this),
                bidAmount
            );
        }
        // If ETH, funds are already in the contract via msg.value

        // 7. Update auction state
        auction.highestBidder = payable(msg.sender);
        auction.highestBidToken = _token;
        auction.highestBidAmount = bidAmount;
        auction.highestBidUSDValue = newBidUSDValue;

        // 8. Emit event
        emit BidPlaced(
            _auctionId,
            msg.sender,
            newBidUSDValue,
            _token,
            bidAmount
        );
    }

    /**
     * @notice Allows users to withdraw their outbid funds (ETH or ERC20).
     */
    function withdraw(address _token) public {
        uint256 amount = fundsToWithdraw[msg.sender][_token];
        require(amount > 0, "No funds to withdraw for this token.");

        fundsToWithdraw[msg.sender][_token] = 0; // Clear record

        if (_token == ETH_ADDRESS) {
            // ETH withdrawal (low-level call)
            (bool success, ) = payable(msg.sender).call{value: amount}("");
            require(success, "ETH transfer failed during withdrawal.");
        } else {
            // ERC20 withdrawal (SafeERC20)
            IERC20(_token).safeTransfer(msg.sender, amount);
        }
    }

    /**
     * @notice Ends the auction and settles the result.
     * Transfers the NFT to the winner and the funds to the seller, or returns the NFT to the seller.
     */
    function endAuction(uint256 _auctionId) public {
        Auction storage auction = auctions[_auctionId];

        // 1. Validate state and time
        require(auction.seller != address(0), "Auction does not exist.");
        require(block.timestamp >= auction.endTime, "Auction is still active.");
        require(!auction.ended, "Auction has already ended.");

        // Mark as ended
        auction.ended = true;

        if (auction.highestBidder != address(0)) {
            // Case A: Successful auction
            address winner = auction.highestBidder;
            uint256 finalPriceUSD = auction.highestBidUSDValue;
            address paymentToken = auction.highestBidToken;
            uint256 paymentAmount = auction.highestBidAmount;

            // 2.1. Transfer NFT to the winner
            auction.nftContract.safeTransferFrom(
                address(this),
                winner,
                auction.tokenId
            );

            // 2.2. Transfer funds to the seller
            if (paymentToken == ETH_ADDRESS) {
                // ETH transfer
                (bool success, ) = payable(auction.seller).call{
                    value: paymentAmount
                }("");
                require(success, "ETH transfer failed to seller.");
            } else {
                // ERC20 transfer
                IERC20(paymentToken).safeTransfer(
                    auction.seller,
                    paymentAmount
                );
            }

            // 2.3. Emit success event
            emit AuctionEnded(
                _auctionId,
                winner,
                finalPriceUSD, // Final price in USD value
                auction.seller,
                block.timestamp
            );
        } else {
            // Case B: Failed auction (no bids)

            // 2.1. Return NFT to the seller
            auction.nftContract.safeTransferFrom(
                address(this),
                auction.seller,
                auction.tokenId
            );

            // 2.2. Emit failure event
            emit AuctionEnded(
                _auctionId,
                address(0), // No winner
                0, // Final price is 0
                auction.seller,
                block.timestamp
            );
        }
    }
}
