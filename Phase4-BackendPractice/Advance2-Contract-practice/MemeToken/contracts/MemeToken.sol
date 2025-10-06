// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract MemeToken is ERC20, Ownable {

    uint256 public taxFee = 5; // 5% tax fee for each transaction
    uint256 public maxTxAmount = 1000000 * 10 ** 18; // Maximum transaction amount
    uint256 public maxWalletAmount = 10000000 * 10 ** 18; // Maximum wallet size

    mapping(address => bool) private _isExcludedFromFee;

    // Constructor to set initial supply
    constructor(uint256 initialSupply) ERC20("MemeToken", "MTK") {
        // Explicitly calling the ERC20 constructor to pass the name and symbol
        super._mint(msg.sender, initialSupply);
        _isExcludedFromFee[msg.sender] = true; // Owner is excluded from fee
    }

    // Override _transfer function to add tax logic
    function _transfer(address sender, address recipient, uint256 amount) internal override {
        require(amount <= maxTxAmount, "Transaction amount exceeds the maximum limit");
        require(balanceOf(recipient) + amount <= maxWalletAmount, "Recipient wallet exceeds max limit");

        uint256 feeAmount = 0;

        // Apply tax fee only if sender or recipient is not excluded
        if (!_isExcludedFromFee[sender] && !_isExcludedFromFee[recipient]) {
            feeAmount = amount * taxFee / 100;
        }

        uint256 amountAfterFee = amount - feeAmount;

        super._transfer(sender, recipient, amountAfterFee); // Transfer the amount after fee

        if (feeAmount > 0) {
            // Transfer the fee to the contract owner (or liquidity pool)
            super._transfer(sender, owner(), feeAmount);
        }
    }

    // Function to exclude addresses from fee (like owner or specific accounts)
    function excludeFromFee(address account) external onlyOwner {
        _isExcludedFromFee[account] = true;
    }

    // Function to include addresses back to fee logic
    function includeInFee(address account) external onlyOwner {
        _isExcludedFromFee[account] = false;
    }

    // Set tax fee percentage
    function setTaxFee(uint256 _taxFee) external onlyOwner {
        taxFee = _taxFee;
    }

    // Set max transaction amount
    function setMaxTxAmount(uint256 _maxTxAmount) external onlyOwner {
        maxTxAmount = _maxTxAmount;
    }

    // Set max wallet amount
    function setMaxWalletAmount(uint256 _maxWalletAmount) external onlyOwner {
        maxWalletAmount = _maxWalletAmount;
    }
}
