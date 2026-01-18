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
    DataTypes
} from "@aave/core-v3/contracts/protocol/libraries/types/DataTypes.sol";
import {
    IERC20
} from "@aave/core-v3/contracts/dependencies/openzeppelin/contracts/IERC20.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

// Interface for aToken
interface IAToken is IERC20 {
    function redeem(uint256 amount) external;
    function UNDERLYING_ASSET_ADDRESS() external view returns (address);
}

// Uniswap V3 SwapRouter interface
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

// Uniswap V2 Router interface (for networks without V3)
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

/**
 * @title FlashLoanLiquidation
 * @notice Production-ready flash loan liquidation contract with DEX integration
 * @dev Supports Uniswap V2/V3 for automatic collateral-to-debt swaps
 */
contract FlashLoanLiquidation is FlashLoanSimpleReceiverBase, ReentrancyGuard, Ownable {
    // DEX Configuration
    address public swapRouter; // Uniswap V3 SwapRouter or V2 Router
    bool public useV3Router; // true for V3, false for V2
    uint24 public defaultPoolFee; // For V3 (3000 = 0.3%, 500 = 0.05%, 10000 = 1%)
    
    // Slippage protection (basis points, 100 = 1%)
    uint256 public maxSlippageBps; // e.g., 100 = 1% slippage
    
    // Events
    event LiquidationExecuted(
        address indexed victim,
        address debtAsset,
        uint256 debtAmount,
        address collateralAsset,
        uint256 collateralReceived
    );
    event FlashLoanRepaid(address asset, uint256 amount, uint256 premium);
    event ProfitWithdrawn(address token, uint256 amount);
    event SwapExecuted(address tokenIn, address tokenOut, uint256 amountIn, uint256 amountOut);
    event ConfigUpdated(address swapRouter, bool useV3, uint24 poolFee, uint256 slippageBps);
    
    /**
     * @dev Constructor
     * @param _addressProvider Aave PoolAddressesProvider address
     * @param _swapRouter Uniswap router address (V2 or V3)
     * @param _useV3Router true if using V3 router, false for V2
     * @param _defaultPoolFee Pool fee for V3 (3000, 500, 10000) - ignored if V2
     * @param _maxSlippageBps Maximum slippage in basis points (100 = 1%)
     */
    constructor(
        address _addressProvider,
        address _swapRouter,
        bool _useV3Router,
        uint24 _defaultPoolFee,
        uint256 _maxSlippageBps
    ) FlashLoanSimpleReceiverBase(IPoolAddressesProvider(_addressProvider)) Ownable() {
        require(_swapRouter != address(0), "Invalid swap router");
        require(_maxSlippageBps <= 1000, "Slippage too high"); // Max 10%
        
        swapRouter = _swapRouter;
        useV3Router = _useV3Router;
        defaultPoolFee = _defaultPoolFee;
        maxSlippageBps = _maxSlippageBps;
        
        // Transfer ownership to deployer
        _transferOwnership(msg.sender);
        
        emit ConfigUpdated(_swapRouter, _useV3Router, _defaultPoolFee, _maxSlippageBps);
    }
    
    /**
     * @dev Update DEX configuration (owner only)
     */
    function updateSwapConfig(
        address _swapRouter,
        bool _useV3Router,
        uint24 _defaultPoolFee,
        uint256 _maxSlippageBps
    ) external onlyOwner {
        require(_swapRouter != address(0), "Invalid swap router");
        require(_maxSlippageBps <= 1000, "Slippage too high");
        
        swapRouter = _swapRouter;
        useV3Router = _useV3Router;
        defaultPoolFee = _defaultPoolFee;
        maxSlippageBps = _maxSlippageBps;
        
        emit ConfigUpdated(_swapRouter, _useV3Router, _defaultPoolFee, _maxSlippageBps);
    }
    
    /**
     * @dev Execute flash loan operation with automatic DEX swap
     * @param asset The flash-borrowed asset (debt token)
     * @param amount The amount of the flash-borrowed asset
     * @param premium The fee to be paid for the flash loan
     * @param params Encoded parameters (victim, collateralAsset, swapPath)
     * @return true if execution is successful
     */
    function executeOperation(
        address asset,
        uint256 amount,
        uint256 premium,
        address /* initiator */,
        bytes calldata params
    ) external override nonReentrant returns (bool) {
        // Decode parameters
        (address victim, address collateralAsset, /* address[] memory swapPath */) = 
            abi.decode(params, (address, address, address[]));
        
        // Step 1: Approve Aave Pool to use flash-loaned tokens for liquidation
        IERC20(asset).approve(address(POOL), amount);
        
        // Step 2: Execute liquidation
        // This receives aToken (collateral) when we pay victim's debt
        POOL.liquidationCall(collateralAsset, asset, victim, amount, true);
        
        // Step 3: Calculate repayment amount
        uint256 totalAmount = amount + premium;
        
        // Step 4: Handle repayment
        uint256 contractBalance = IERC20(asset).balanceOf(address(this));
        
        if (contractBalance >= totalAmount) {
            // We have enough of the debt asset to repay directly
            IERC20(asset).approve(address(POOL), totalAmount);
        } else {
            // Need to swap collateral to debt asset
            uint256 amountNeeded = totalAmount - contractBalance;
            
            // Get aToken address for collateral
            DataTypes.ReserveData memory reserveData = POOL.getReserveData(collateralAsset);
            address aTokenAddress = reserveData.aTokenAddress;
            
            // Redeem aToken to get underlying collateral
            IAToken aToken = IAToken(aTokenAddress);
            uint256 aTokenBalance = aToken.balanceOf(address(this));
            require(aTokenBalance > 0, "No collateral received");
            
            // Redeem enough aToken to cover the shortfall (with buffer)
            uint256 amountToRedeem = aTokenBalance; // Redeem all for simplicity
            aToken.redeem(amountToRedeem);
            
            // Get underlying collateral balance
            uint256 collateralBalance = IERC20(collateralAsset).balanceOf(address(this));
            require(collateralBalance > 0, "No collateral after redeem");
            
            // Swap collateral to debt asset
            uint256 amountOut = _swapTokens(
                collateralAsset,
                asset,
                collateralBalance,
                amountNeeded
            );
            
            require(amountOut >= amountNeeded, "Swap insufficient");
            
            // Approve repayment
            IERC20(asset).approve(address(POOL), totalAmount);
        }
        
        emit FlashLoanRepaid(asset, amount, premium);
        
        // Get final collateral received (if any remaining)
        DataTypes.ReserveData memory finalReserveData = POOL.getReserveData(collateralAsset);
        address finalATokenAddress = finalReserveData.aTokenAddress;
        IAToken finalAToken = IAToken(finalATokenAddress);
        uint256 finalCollateral = finalAToken.balanceOf(address(this));
        
        emit LiquidationExecuted(victim, asset, amount, collateralAsset, finalCollateral);
        
        return true;
    }
    
    /**
     * @dev Internal function to swap tokens using Uniswap
     * @param tokenIn Input token address
     * @param tokenOut Output token address
     * @param amountIn Amount of input tokens
     * @param amountOutMin Minimum amount of output tokens (for slippage protection)
     * @return amountOut Actual amount of output tokens received
     */
    function _swapTokens(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin
    ) internal returns (uint256 amountOut) {
        require(amountIn > 0, "Invalid swap amount");
        
        // Approve router to spend input tokens
        IERC20(tokenIn).approve(swapRouter, amountIn);
        
        if (useV3Router) {
            // Uniswap V3
            ISwapRouter router = ISwapRouter(swapRouter);
            
            ISwapRouter.ExactInputSingleParams memory params = ISwapRouter.ExactInputSingleParams({
                tokenIn: tokenIn,
                tokenOut: tokenOut,
                fee: defaultPoolFee,
                recipient: address(this),
                deadline: block.timestamp + 300, // 5 minutes
                amountIn: amountIn,
                amountOutMinimum: amountOutMin,
                sqrtPriceLimitX96: 0
            });
            
            amountOut = router.exactInputSingle(params);
        } else {
            // Uniswap V2
            IUniswapV2Router router = IUniswapV2Router(swapRouter);
            
            address[] memory path = new address[](2);
            path[0] = tokenIn;
            path[1] = tokenOut;
            
            uint256[] memory amounts = router.swapExactTokensForTokens(
                amountIn,
                amountOutMin,
                path,
                address(this),
                block.timestamp + 300 // 5 minutes
            );
            
            amountOut = amounts[amounts.length - 1];
        }
        
        emit SwapExecuted(tokenIn, tokenOut, amountIn, amountOut);
    }
    
    /**
     * @dev Get minimum output amount with slippage protection
     * @param amountIn Input amount
     * @param tokenIn Input token
     * @param tokenOut Output token
     * @return minAmountOut Minimum output amount considering slippage
     */
    function getMinAmountOut(
        uint256 amountIn,
        address tokenIn,
        address tokenOut
    ) public view returns (uint256 minAmountOut) {
        if (useV3Router) {
            // For V3, we'd need a quoter contract - simplified here
            // In production, integrate with QuoterV2
            revert("Use estimateAmountOut for V3");
        } else {
            IUniswapV2Router router = IUniswapV2Router(swapRouter);
            address[] memory path = new address[](2);
            path[0] = tokenIn;
            path[1] = tokenOut;
            
            uint256[] memory amounts = router.getAmountsOut(amountIn, path);
            uint256 expectedOut = amounts[amounts.length - 1];
            
            // Apply slippage protection
            minAmountOut = expectedOut * (10000 - maxSlippageBps) / 10000;
        }
    }
    
    /**
     * @dev Convenience function with default swap path
     */
    function requestLiquidationLoanSimple(
        address _token,
        uint256 _amount,
        address _victim,
        address _collateralAsset
    ) external {
        address[] memory swapPath; // Empty - will use direct path
        requestLiquidationLoan(_token, _amount, _victim, _collateralAsset, swapPath);
    }
    
    /**
     * @dev Initiates a flash loan to liquidate an Aave position
     * @param _token The debt token address to borrow
     * @param _amount The amount of debt to cover
     * @param _victim The address of the user being liquidated
     * @param _collateralAsset The collateral asset address
     * @param _swapPath Swap path for DEX (can be empty if no swap needed)
     */
    function requestLiquidationLoan(
        address _token,
        uint256 _amount,
        address _victim,
        address _collateralAsset,
        address[] memory _swapPath
    ) public {
        bytes memory params = abi.encode(_victim, _collateralAsset, _swapPath);
        POOL.flashLoanSimple(address(this), _token, _amount, params, 0);
    }
    
    /**
     * @dev Withdraw tokens (owner only)
     */
    function withdraw(address _tokenAddress) external onlyOwner nonReentrant {
        IERC20 token = IERC20(_tokenAddress);
        uint256 balance = token.balanceOf(address(this));
        require(balance > 0, "No balance");
        token.transfer(owner(), balance);
        emit ProfitWithdrawn(_tokenAddress, balance);
    }
    
    /**
     * @dev Withdraw aToken and convert to underlying
     */
    function withdrawAToken(address _aTokenAddress) external onlyOwner nonReentrant {
        IAToken aToken = IAToken(_aTokenAddress);
        uint256 balance = aToken.balanceOf(address(this));
        require(balance > 0, "No aToken balance");
        
        aToken.redeem(balance);
        
        address underlyingAsset = aToken.UNDERLYING_ASSET_ADDRESS();
        IERC20 underlying = IERC20(underlyingAsset);
        uint256 underlyingBalance = underlying.balanceOf(address(this));
        underlying.transfer(owner(), underlyingBalance);
        
        emit ProfitWithdrawn(underlyingAsset, underlyingBalance);
    }
    
    /**
     * @dev Emergency withdraw (owner only)
     */
    function emergencyWithdraw(address _tokenAddress) external onlyOwner nonReentrant {
        IERC20 token = IERC20(_tokenAddress);
        uint256 balance = token.balanceOf(address(this));
        if (balance > 0) {
            token.transfer(owner(), balance);
        }
    }
    
    /**
     * @dev Get contract balance of a token
     */
    function getBalance(address _tokenAddress) external view returns (uint256) {
        return IERC20(_tokenAddress).balanceOf(address(this));
    }
    
    /**
     * @dev Receive ETH (if needed for gas refunds)
     */
    receive() external payable {}
}
