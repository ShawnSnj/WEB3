// SPDX-License-Identifier: MIT
pragma solidity ^0.8.10;

import {
    FlashLoanSimpleReceiverBase
} from "@aave/core-v3/contracts/flashloan/base/FlashLoanSimpleReceiverBase.sol";
import {
    IPoolAddressesProvider
} from "@aave/core-v3/contracts/interfaces/IPoolAddressesProvider.sol";
import {
    IERC20
} from "@aave/core-v3/contracts/dependencies/openzeppelin/contracts/IERC20.sol";

// Interface for aToken
interface IAToken is IERC20 {
    function redeem(uint256 amount) external;
    function UNDERLYING_ASSET_ADDRESS() external view returns (address);
}

contract FlashLoanExample is FlashLoanSimpleReceiverBase {
    address public owner;
    
    // Events for tracking
    event LiquidationExecuted(address indexed victim, address debtAsset, uint256 debtAmount, address collateralAsset, uint256 collateralReceived);
    event FlashLoanRepaid(address asset, uint256 amount, uint256 premium);
    event ProfitWithdrawn(address token, uint256 amount);

    constructor(
        address _addressProvider
    ) FlashLoanSimpleReceiverBase(IPoolAddressesProvider(_addressProvider)) {
        owner = msg.sender;
    }

    /**
     * @dev This function is called by the Aave Pool after the flash loan is initiated
     * @param asset The address of the flash-borrowed asset (debt token, e.g., USDC)
     * @param amount The amount of the flash-borrowed asset
     * @param premium The fee to be paid for the flash loan
     * @param initiator The address that initiated the flash loan
     * @param params Encoded parameters (victim address)
     * @return true if the execution is successful
     */
    function executeOperation(
        address asset,
        uint256 amount,
        uint256 premium,
        address initiator,
        bytes calldata params
    ) external override returns (bool) {
        // Decode parameters: victim address and collateral asset
        (address victim, address collateralAsset) = abi.decode(params, (address, address));
        
        // Step 1: Approve Aave Pool to use the flash-loaned tokens for liquidation
        IERC20(asset).approve(address(POOL), amount);

        // Step 2: Execute the liquidation
        // liquidationCall receives aWETH (collateral) when we pay victim's debt
        POOL.liquidationCall(collateralAsset, asset, victim, amount, true);

        // Step 3: Calculate what we need to repay (loan amount + premium)
        uint256 totalAmount = amount + premium;
        
        // Step 4: Handle repayment
        // Option A: If we have enough of the debt asset (asset) in the contract, use it
        // Option B: Convert collateral to debt asset (requires DEX swap - not implemented here)
        // For this demo, we assume the contract has been pre-funded with enough tokens
        // to cover the premium, or we handle it differently
        
        uint256 contractBalance = IERC20(asset).balanceOf(address(this));
        
        if (contractBalance >= totalAmount) {
            // We have enough balance to repay
            IERC20(asset).approve(address(POOL), totalAmount);
        } else {
            // This would require swapping collateral to debt asset
            // For demo purposes, we revert with a clear message
            // In production, you'd integrate a DEX like Uniswap here
            revert("Insufficient balance to repay flash loan. Need to swap collateral or pre-fund contract.");
        }

        emit FlashLoanRepaid(asset, amount, premium);
        emit LiquidationExecuted(victim, asset, amount, collateralAsset, 0);

        return true;
    }

    /**
     * @dev Initiates a flash loan to liquidate an Aave position
     * @param _token The debt token address to borrow (e.g., USDC)
     * @param _amount The amount of debt to cover
     * @param _victim The address of the user being liquidated
     * @param _collateralAsset The collateral asset address (e.g., WETH)
     */
    function requestLiquidationLoan(
        address _token,
        uint256 _amount,
        address _victim,
        address _collateralAsset
    ) public {
        bytes memory params = abi.encode(_victim, _collateralAsset);
        POOL.flashLoanSimple(address(this), _token, _amount, params, 0);
    }

    /**
     * @dev Convenience function that uses WETH as collateral (default)
     */
    function requestLiquidationLoanWithWETH(
        address _token,
        uint256 _amount,
        address _victim
    ) external {
        address weth = 0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2; // Mainnet WETH
        requestLiquidationLoan(_token, _amount, _victim, weth);
    }

    /**
     * @dev Withdraw tokens from the contract (for withdrawing profits)
     * @param _tokenAddress The token address to withdraw
     */
    function withdraw(address _tokenAddress) external {
        require(msg.sender == owner, "Only owner");
        IERC20 token = IERC20(_tokenAddress);
        uint256 balance = token.balanceOf(address(this));
        require(balance > 0, "No balance to withdraw");
        token.transfer(owner, balance);
        emit ProfitWithdrawn(_tokenAddress, balance);
    }

    /**
     * @dev Withdraw aToken and convert to underlying asset
     * @param _aTokenAddress The aToken address
     */
    function withdrawAToken(address _aTokenAddress) external {
        require(msg.sender == owner, "Only owner");
        IAToken aToken = IAToken(_aTokenAddress);
        uint256 balance = aToken.balanceOf(address(this));
        require(balance > 0, "No aToken balance");
        
        // Redeem aToken to get underlying asset
        aToken.redeem(balance);
        
        // Transfer underlying asset to owner
        address underlyingAsset = aToken.UNDERLYING_ASSET_ADDRESS();
        IERC20 underlying = IERC20(underlyingAsset);
        uint256 underlyingBalance = underlying.balanceOf(address(this));
        underlying.transfer(owner, underlyingBalance);
        
        emit ProfitWithdrawn(underlyingAsset, underlyingBalance);
    }

    /**
     * @dev Emergency function to withdraw any ERC20 token
     */
    function emergencyWithdraw(address _tokenAddress) external {
        require(msg.sender == owner, "Only owner");
        IERC20 token = IERC20(_tokenAddress);
        uint256 balance = token.balanceOf(address(this));
        if (balance > 0) {
            token.transfer(owner, balance);
        }
    }

    /**
     * @dev Get the contract's balance of a token
     */
    function getBalance(address _tokenAddress) external view returns (uint256) {
        return IERC20(_tokenAddress).balanceOf(address(this));
    }
}
