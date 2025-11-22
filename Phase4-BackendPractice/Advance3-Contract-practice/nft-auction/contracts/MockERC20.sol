// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/**
 * @title MockERC20
 * @dev Simple ERC20 for testing.
 */
contract MockERC20 is ERC20 {
    constructor(
        string memory name,
        string memory symbol,
        uint8 tokenDecimals
    ) ERC20(name, symbol) {
        // 設定小數點位數
        _decimals = tokenDecimals;
    }

    // 擴展ERC20以允許設置小數點位數
    uint8 private _decimals;
    function decimals() public view override returns (uint8) {
        return _decimals;
    }

    /**
     * @notice Mints tokens to a specified recipient (for test setup).
     */
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}
