// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @notice Minimal constant-product AMM (toy; no ERC20). Swaps update reserves internally.
contract SimpleDEX {
    uint256 public reserveX;
    uint256 public reserveY;

    /// @dev 0.3% fee => 30 bps.
    uint256 public constant FEE_BPS = 30;

    constructor(uint256 _reserveX, uint256 _reserveY) {
        reserveX = _reserveX;
        reserveY = _reserveY;
    }

    function getReserves() external view returns (uint256, uint256) {
        return (reserveX, reserveY);
    }

    /// @notice Price of Y in terms of X (scaled by 1e18): reserveY / reserveX.
    function getPrice() external view returns (uint256) {
        require(reserveX > 0, "no price");
        return (reserveY * 1e18) / reserveX;
    }

    /// @notice Swap X in for Y out.
    /// @param dx Input amount of token X (integer units).
    function swap(uint256 dx) external returns (uint256 dy) {
        require(dx > 0, "dx=0");
        uint256 fee = (dx * FEE_BPS) / 10000;
        uint256 dxAfterFee = dx - fee;

        // Constant product invariant.
        uint256 k = reserveX * reserveY;
        uint256 newReserveX = reserveX + dxAfterFee;
        uint256 newReserveY = k / newReserveX;
        dy = reserveY - newReserveY;
        require(dy > 0, "dy=0");

        reserveX = newReserveX;
        reserveY = newReserveY;
    }

    /// @notice Swap Y in for X out (reverse direction).
    /// @param dy Input amount of token Y (integer units).
    function swapReverse(uint256 dy) external returns (uint256 dx) {
        require(dy > 0, "dy=0");
        uint256 fee = (dy * FEE_BPS) / 10000;
        uint256 dyAfterFee = dy - fee;

        uint256 k = reserveX * reserveY;
        uint256 newReserveY = reserveY + dyAfterFee;
        uint256 newReserveX = k / newReserveY;
        dx = reserveX - newReserveX;
        require(dx > 0, "dx=0");

        reserveX = newReserveX;
        reserveY = newReserveY;
    }
}

