// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ISimpleDEX {
    function swap(uint256 dx) external returns (uint256 dy);
    function swapReverse(uint256 dy) external returns (uint256 dx);
}

/// @notice Minimal arbitrage executor. Keeps profit inside the contract.
contract FlashArb {
    uint256 public totalProfit;

    event Arbitrage(address indexed dex1, address indexed dex2, uint256 amountIn, uint256 amountOut, uint256 profit);

    /// @dev Swaps on `dex1` (X->Y), then swaps back on `dex2` (Y->X).
    ///      Reverts if the trade is not strictly profitable.
    function executeArbitrage(address dex1, address dex2, uint256 amount) external {
        require(amount > 0, "amount=0");

        uint256 dy = ISimpleDEX(dex1).swap(amount);
        uint256 finalAmount = ISimpleDEX(dex2).swapReverse(dy);

        require(finalAmount > amount, "no profit");

        uint256 profit = finalAmount - amount;
        totalProfit += profit;

        emit Arbitrage(dex1, dex2, amount, finalAmount, profit);
    }
}

