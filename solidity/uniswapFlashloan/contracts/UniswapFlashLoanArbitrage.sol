// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import {
    FlashLoanSimpleReceiverBase
} from "@aave/core-v3/contracts/flashloan/base/FlashLoanSimpleReceiverBase.sol";
import {
    IPoolAddressesProvider
} from "@aave/core-v3/contracts/interfaces/IPoolAddressesProvider.sol";
import {
    IPool
} from "@aave/core-v3/contracts/interfaces/IPool.sol";
import {
    IERC20
} from "@aave/core-v3/contracts/dependencies/openzeppelin/contracts/IERC20.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

// Uniswap V2 Router interface
interface IUniswapV2Router {
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external returns (uint256[] memory amounts);

    function getAmountsOut(uint256 amountIn, address[] calldata path)
        external
        view
        returns (uint256[] memory amounts);
}

// Uniswap V3 Router interface
interface ISwapRouter {
    struct ExactInputSingleParams {
        address tokenIn;
        address tokenOut;
        uint24 fee;
        address recipient;
        uint256 deadline;
        uint256 amountIn;
        uint256 amountOutMinimum;
        uint160 sqrtPriceLimitX96;
    }

    function exactInputSingle(ExactInputSingleParams calldata params)
        external
        payable
        returns (uint256 amountOut);
}

/**
 * @title UniswapFlashLoanArbitrage
 * @dev Contract for executing arbitrage opportunities using Aave flash loans
 *      and Uniswap swaps. Can detect price differences between Uniswap pools
 *      and execute profitable trades.
 */
contract UniswapFlashLoanArbitrage is
    FlashLoanSimpleReceiverBase,
    ReentrancyGuard,
    Ownable
{
    // Uniswap routers
    IUniswapV2Router public immutable UNISWAP_V2_ROUTER;
    ISwapRouter public immutable UNISWAP_V3_ROUTER;
    
    // Configuration
    bool public useV3Router;
    uint24 public defaultPoolFee; // For V3 (3000 = 0.3%)
    uint256 public maxSlippageBps; // Basis points (50 = 0.5%)

    // Events
    event FlashLoanExecuted(
        address indexed asset,
        uint256 amount,
        uint256 premium,
        uint256 profit
    );
    event ArbitrageExecuted(
        address indexed tokenIn,
        address indexed tokenOut,
        uint256 amountIn,
        uint256 amountOut,
        uint256 profit
    );
    event ProfitWithdrawn(address indexed token, uint256 amount);

    /**
     * @dev Constructor
     * @param _addressesProvider Aave Pool Addresses Provider
     * @param _v2Router Uniswap V2 Router address
     * @param _v3Router Uniswap V3 Router address
     * @param _useV3Router Whether to use V3 router by default
     * @param _defaultPoolFee Default pool fee for V3 (3000 = 0.3%)
     * @param _maxSlippageBps Maximum slippage in basis points
     */
    constructor(
        IPoolAddressesProvider _addressesProvider,
        address _v2Router,
        address _v3Router,
        bool _useV3Router,
        uint24 _defaultPoolFee,
        uint256 _maxSlippageBps
    ) FlashLoanSimpleReceiverBase(_addressesProvider) Ownable() {
        UNISWAP_V2_ROUTER = IUniswapV2Router(_v2Router);
        UNISWAP_V3_ROUTER = ISwapRouter(_v3Router);
        useV3Router = _useV3Router;
        defaultPoolFee = _defaultPoolFee;
        maxSlippageBps = _maxSlippageBps;
    }

    /**
     * @dev Execute flash loan for arbitrage
     * @param _token Token to borrow
     * @param _amount Amount to borrow
     * @param _tokenIn Input token for swap
     * @param _tokenOut Output token for swap
     * @param _expectedProfit Minimum expected profit (in wei)
     */
    function requestArbitrageLoan(
        address _token,
        uint256 _amount,
        address _tokenIn,
        address _tokenOut,
        uint256 _expectedProfit
    ) external {
        bytes memory params = abi.encode(
            _tokenIn,
            _tokenOut,
            _expectedProfit,
            useV3Router,
            defaultPoolFee
        );

        POOL.flashLoanSimple(
            address(this),
            _token,
            _amount,
            params,
            0
        );
    }

    /**
     * @dev Execute flash loan with custom swap path (V2)
     * @param _token Token to borrow
     * @param _amount Amount to borrow
     * @param _swapPath Array of token addresses for swap path
     * @param _expectedProfit Minimum expected profit (in wei)
     */
    function requestArbitrageLoanWithPath(
        address _token,
        uint256 _amount,
        address[] memory _swapPath,
        uint256 _expectedProfit
    ) external {
        bytes memory params = abi.encode(
            _swapPath,
            _expectedProfit,
            false, // Use V2
            uint24(0) // Not used for V2
        );

        POOL.flashLoanSimple(
            address(this),
            _token,
            _amount,
            params,
            0
        );
    }

    /**
     * @dev Aave callback - executes arbitrage logic
     */
    function executeOperation(
        address asset,
        uint256 amount,
        uint256 premium,
        address, // initiator
        bytes calldata params
    ) external override returns (bool) {
        // Decode parameters
        (
            address tokenIn,
            address tokenOut,
            uint256 expectedProfit,
            bool useV3,
            uint24 poolFee
        ) = abi.decode(params, (address, address, uint256, bool, uint24));

        // Calculate total amount to repay
        uint256 totalAmount = amount + premium;

        // Execute arbitrage swap
        uint256 amountOut = _executeSwap(
            asset,
            amount,
            tokenIn,
            tokenOut,
            useV3,
            poolFee
        );

        // Calculate profit
        uint256 profit = amountOut > totalAmount
            ? amountOut - totalAmount
            : 0;

        require(profit >= expectedProfit, "Insufficient profit");

        // Approve repayment
        IERC20(asset).approve(address(POOL), totalAmount);

        emit FlashLoanExecuted(asset, amount, premium, profit);
        emit ArbitrageExecuted(tokenIn, tokenOut, amount, amountOut, profit);

        return true;
    }

    /**
     * @dev Execute swap using Uniswap V2 or V3
     */
    function _executeSwap(
        address asset,
        uint256 amountIn,
        address tokenIn,
        address tokenOut,
        bool useV3,
        uint24 poolFee
    ) internal returns (uint256 amountOut) {
        // Approve router to spend tokens
        IERC20(asset).approve(
            useV3 ? address(UNISWAP_V3_ROUTER) : address(UNISWAP_V2_ROUTER),
            amountIn
        );

        if (useV3) {
            // Uniswap V3 swap
            ISwapRouter.ExactInputSingleParams memory swapParams =
                ISwapRouter.ExactInputSingleParams({
                    tokenIn: tokenIn,
                    tokenOut: tokenOut,
                    fee: poolFee,
                    recipient: address(this),
                    deadline: block.timestamp + 300,
                    amountIn: amountIn,
                    amountOutMinimum: _calculateMinAmountOut(amountIn),
                    sqrtPriceLimitX96: 0
                });

            amountOut = UNISWAP_V3_ROUTER.exactInputSingle(swapParams);
        } else {
            // Uniswap V2 swap
            address[] memory path = new address[](2);
            path[0] = tokenIn;
            path[1] = tokenOut;

            uint256[] memory amounts = UNISWAP_V2_ROUTER.swapExactTokensForTokens(
                amountIn,
                _calculateMinAmountOut(amountIn),
                path,
                address(this),
                block.timestamp + 300
            );

            amountOut = amounts[amounts.length - 1];
        }
    }

    /**
     * @dev Calculate minimum amount out with slippage protection
     */
    function _calculateMinAmountOut(uint256 amountIn)
        internal
        view
        returns (uint256)
    {
        uint256 slippageAmount = (amountIn * maxSlippageBps) / 10000;
        return amountIn - slippageAmount;
    }

    /**
     * @dev Get quote for swap (V2)
     */
    function getQuoteV2(
        uint256 amountIn,
        address[] calldata path
    ) external view returns (uint256[] memory amounts) {
        return UNISWAP_V2_ROUTER.getAmountsOut(amountIn, path);
    }

    /**
     * @dev Withdraw profits (owner only)
     */
    function withdrawProfit(address token) external onlyOwner {
        uint256 balance = IERC20(token).balanceOf(address(this));
        require(balance > 0, "No balance to withdraw");

        IERC20(token).transfer(owner(), balance);
        emit ProfitWithdrawn(token, balance);
    }

    /**
     * @dev Get contract balance of a token
     */
    function getBalance(address token) external view returns (uint256) {
        return IERC20(token).balanceOf(address(this));
    }

    /**
     * @dev Update configuration (owner only)
     */
    function updateConfig(
        bool _useV3Router,
        uint24 _defaultPoolFee,
        uint256 _maxSlippageBps
    ) external onlyOwner {
        useV3Router = _useV3Router;
        defaultPoolFee = _defaultPoolFee;
        maxSlippageBps = _maxSlippageBps;
    }

    /**
     * @dev Emergency withdraw (owner only)
     */
    function emergencyWithdraw(address token) external onlyOwner {
        uint256 balance = IERC20(token).balanceOf(address(this));
        IERC20(token).transfer(owner(), balance);
    }
}
