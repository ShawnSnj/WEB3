// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title SimpleLending
 * @notice Educational, simplified Compound-style single-asset lending protocol.
 *
 *  - Single ERC20 asset as both collateral and borrow asset.
 *  - Fixed interest rate per second.
 *  - Overcollateralized borrowing with a collateral factor.
 *  - Simple liquidation with a fixed liquidation bonus.
 *
 *  SECURITY WARNING:
 *  This contract is for LEARNING ONLY.
 *  It is NOT audited, NOT optimized, and NOT safe for mainnet use.
 */
contract SimpleLending {
    /// -----------------------------------------------------------------------
    /// Dependencies
    /// -----------------------------------------------------------------------

    interface IERC20 {
        function totalSupply() external view returns (uint256);
        function balanceOf(address account) external view returns (uint256);
        function transfer(address to, uint256 value) external returns (bool);
        function transferFrom(address from, address to, uint256 value) external returns (bool);
        function approve(address spender, uint256 value) external returns (bool);
        function allowance(address owner, address spender) external view returns (uint256);
    }

    /// -----------------------------------------------------------------------
    /// Events
    /// -----------------------------------------------------------------------

    event Deposit(address indexed user, uint256 amount);
    event Withdraw(address indexed user, uint256 amount);
    event Borrow(address indexed user, uint256 amount);
    event Repay(address indexed user, uint256 amount);
    event Liquidate(address indexed liquidator, address indexed borrower, uint256 repayAmount, uint256 seizedCollateral);
    event InterestAccrued(uint256 interest, uint256 newTotalBorrows, uint256 newLastAccrualTimestamp);

    /// -----------------------------------------------------------------------
    /// Immutable configuration
    /// -----------------------------------------------------------------------

    IERC20 public immutable asset;

    // All fixed-point params are scaled by 1e18 (WAD).
    uint256 public immutable interestRatePerSecond; // e.g. 5% per year / secondsPerYear
    uint256 public immutable collateralFactor;      // e.g. 0.75e18 = 75%
    uint256 public immutable liquidationBonus;      // e.g. 0.10e18 = 10% extra collateral

    /// -----------------------------------------------------------------------
    /// Global state
    /// -----------------------------------------------------------------------

    uint256 public totalDeposits;
    uint256 public totalBorrows;
    uint256 public lastAccrualTimestamp;

    /// -----------------------------------------------------------------------
    /// Per-user state
    /// -----------------------------------------------------------------------

    mapping(address => uint256) public deposits;
    mapping(address => uint256) public borrows;

    /// -----------------------------------------------------------------------
    /// Constructor
    /// -----------------------------------------------------------------------

    /**
     * @param _asset ERC20 token used for both collateral and borrowing.
     * @param _annualInterestRateWad Annual borrow rate scaled by 1e18 (e.g. 0.05e18 = 5%).
     * @param _collateralFactorWad   Collateral factor scaled by 1e18 (e.g. 0.75e18 = 75%).
     * @param _liquidationBonusWad   Liquidation bonus scaled by 1e18 (e.g. 0.10e18 = 10%).
     */
    constructor(
        IERC20 _asset,
        uint256 _annualInterestRateWad,
        uint256 _collateralFactorWad,
        uint256 _liquidationBonusWad
    ) {
        require(address(_asset) != address(0), "asset zero");
        require(_collateralFactorWad <= 1e18, "collateralFactor > 1");

        asset = _asset;

        // Convert annual rate to per-second: r_year / secondsPerYear
        uint256 secondsPerYear = 365 days;
        interestRatePerSecond = _annualInterestRateWad / secondsPerYear;

        collateralFactor = _collateralFactorWad;
        liquidationBonus = _liquidationBonusWad;

        lastAccrualTimestamp = block.timestamp;
    }

    /// -----------------------------------------------------------------------
    /// Core external functions
    /// -----------------------------------------------------------------------

    /**
     * @notice Deposit `amount` of tokens as collateral.
     *         User must have approved this contract to spend `amount`.
     */
    function deposit(uint256 amount) external {
        require(amount > 0, "amount = 0");

        _accrueInterest();

        // Transfer tokens from user to protocol.
        require(asset.transferFrom(msg.sender, address(this), amount), "transferFrom failed");

        deposits[msg.sender] += amount;
        totalDeposits += amount;

        emit Deposit(msg.sender, amount);
    }

    /**
     * @notice Withdraw `amount` of collateral, as long as the position stays safe.
     */
    function withdraw(uint256 amount) external {
        require(amount > 0, "amount = 0");
        require(deposits[msg.sender] >= amount, "insufficient deposit");

        _accrueInterest();

        deposits[msg.sender] -= amount;
        totalDeposits -= amount;

        require(_isSafe(msg.sender), "would become unsafe");

        require(asset.transfer(msg.sender, amount), "transfer failed");

        emit Withdraw(msg.sender, amount);
    }

    /**
     * @notice Borrow `amount` of tokens against your collateral.
     */
    function borrow(uint256 amount) external {
        require(amount > 0, "amount = 0");
        require(asset.balanceOf(address(this)) >= amount, "insufficient liquidity");

        _accrueInterest();

        borrows[msg.sender] += amount;
        totalBorrows += amount;

        require(_isSafe(msg.sender), "insufficient collateral");

        require(asset.transfer(msg.sender, amount), "transfer failed");

        emit Borrow(msg.sender, amount);
    }

    /**
     * @notice Repay `amount` of your debt.
     *         If `amount` > current debt, we just repay all.
     */
    function repay(uint256 amount) external {
        require(amount > 0, "amount = 0");

        _accrueInterest();

        uint256 debt = borrows[msg.sender];
        require(debt > 0, "no debt");

        uint256 repayAmount = amount > debt ? debt : amount;

        require(asset.transferFrom(msg.sender, address(this), repayAmount), "transferFrom failed");

        borrows[msg.sender] = debt - repayAmount;
        totalBorrows -= repayAmount;

        emit Repay(msg.sender, repayAmount);
    }

    /**
     * @notice Liquidate an unsafe borrower by repaying `repayAmount` of their debt,
     *         receiving their collateral plus a liquidation bonus.
     */
    function liquidate(address borrower, uint256 repayAmount) external {
        require(borrower != address(0), "borrower zero");
        require(repayAmount > 0, "amount = 0");

        _accrueInterest();

        require(!_isSafe(borrower), "borrower safe");

        uint256 borrowerDebt = borrows[borrower];
        require(borrowerDebt > 0, "no debt");

        uint256 actualRepay = repayAmount > borrowerDebt ? borrowerDebt : repayAmount;

        // Transfer repayment from liquidator to protocol.
        require(asset.transferFrom(msg.sender, address(this), actualRepay), "transferFrom failed");

        // Update borrow state.
        borrows[borrower] = borrowerDebt - actualRepay;
        totalBorrows -= actualRepay;

        // Calculate collateral to seize: repay * (1 + liquidationBonus).
        uint256 seizeAmount = (actualRepay * (1e18 + liquidationBonus)) / 1e18;
        require(deposits[borrower] >= seizeAmount, "not enough collateral");

        deposits[borrower] -= seizeAmount;
        deposits[msg.sender] += seizeAmount;

        emit Liquidate(msg.sender, borrower, actualRepay, seizeAmount);
    }

    /**
     * @notice Public wrapper to manually trigger interest accrual.
     */
    function accrueInterest() external {
        _accrueInterest();
    }

    /// -----------------------------------------------------------------------
    /// View functions
    /// -----------------------------------------------------------------------

    /**
     * @notice Returns true if `user` is safe under current collateral factor.
     */
    function isSafe(address user) external view returns (bool) {
        return _isSafe(user);
    }

    /**
     * @notice Returns a simple health factor scaled by 1e18.
     *         > 1e18  => safe
     *         == 1e18 => exactly at limit
     *         < 1e18  => unsafe
     */
    function getHealthFactor(address user) external view returns (uint256) {
        uint256 collateralValue = deposits[user];
        uint256 borrowValue = borrows[user];
        if (borrowValue == 0) {
            // Infinite health if no debt.
            return type(uint256).max;
        }

        uint256 maxBorrowAllowed = (collateralValue * collateralFactor) / 1e18;
        return (maxBorrowAllowed * 1e18) / borrowValue;
    }

    /// -----------------------------------------------------------------------
    /// Internal helpers
    /// -----------------------------------------------------------------------

    function _accrueInterest() internal {
        uint256 currentTimestamp = block.timestamp;
        if (currentTimestamp == lastAccrualTimestamp) {
            return;
        }

        uint256 timeElapsed = currentTimestamp - lastAccrualTimestamp;

        if (totalBorrows > 0 && interestRatePerSecond > 0) {
            // Simple interest: totalBorrows += totalBorrows * rate * dt
            uint256 interest = (totalBorrows * interestRatePerSecond * timeElapsed) / 1e18;
            totalBorrows += interest;

            emit InterestAccrued(interest, totalBorrows, currentTimestamp);
        }

        lastAccrualTimestamp = currentTimestamp;
    }

    function _isSafe(address user) internal view returns (bool) {
        uint256 collateralValue = deposits[user];
        uint256 borrowValue = borrows[user];

        if (borrowValue == 0) {
            return true;
        }

        uint256 maxBorrowAllowed = (collateralValue * collateralFactor) / 1e18;
        return maxBorrowAllowed >= borrowValue;
    }
}

