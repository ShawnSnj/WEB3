# Flash Loan Liquidation Logic Explained

## 📊 Overview Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    FLASH LOAN LIQUIDATION FLOW                   │
└─────────────────────────────────────────────────────────────────┘

1. USER CALLS
   requestLiquidationLoan()
   └─> Requests flash loan from Aave
       │
       ├─> Parameters:
       │   - _token: USDC (debt token to borrow)
       │   - _amount: 10,000 USDC
       │   - _victim: Address with unhealthy position
       │   - _collateralAsset: WETH
       │
       └─> Calls: POOL.flashLoanSimple()

2. AAVE POOL EXECUTES
   └─> Transfers 10,000 USDC to contract
       └─> Calls: executeOperation() on our contract
           │
           ├─> ASSETS RECEIVED:
           │   - 10,000 USDC (flash loaned)
           │
           └─> OPERATIONS INSIDE executeOperation():
               │
               ├─> Step 1: Approve USDC for liquidation
               │   └─> Approve POOL to spend 10,000 USDC
               │
               ├─> Step 2: Execute Liquidation
               │   └─> POOL.liquidationCall()
               │       ├─> Pays victim's 10,000 USDC debt
               │       └─> Receives: ~10,500 aWETH (collateral + bonus)
               │
               ├─> Step 3: Calculate Repayment
               │   └─> totalAmount = 10,000 + premium (~50 USDC)
               │
               └─> Step 4: Repay Flash Loan
                   └─> Approve & repay 10,050 USDC to Aave
                       └─> Uses contract's pre-funded USDC balance

3. RESULT
   └─> Contract has:
       ├─> ~10,500 aWETH (profit from liquidation)
       └─> ~9,950 USDC remaining (if we had 10,000 pre-funded)
           │
           └─> NET PROFIT: ~500 aWETH - gas costs
```

---

## 🔍 Step-by-Step Breakdown

### Phase 1: Initiation (`requestLiquidationLoan`)

```solidity
function requestLiquidationLoan(
    address _token,        // USDC address
    uint256 _amount,       // 10,000 USDC
    address _victim,       // User with HF < 1.0
    address _collateralAsset  // WETH address
) public {
    // Encode parameters to pass to executeOperation
    bytes memory params = abi.encode(_victim, _collateralAsset);
    
    // Request flash loan from Aave Pool
    POOL.flashLoanSimple(
        address(this),  // Receiver (our contract)
        _token,         // Asset to borrow (USDC)
        _amount,        // Amount (10,000 USDC)
        params,         // Custom data
        0               // Referral code
    );
}
```

**What happens:**
1. Encodes `victim` and `collateralAsset` into bytes
2. Calls Aave's `flashLoanSimple()`
3. Aave transfers USDC to our contract
4. Aave calls our `executeOperation()` function

---

### Phase 2: Execution (`executeOperation`)

This is the **core logic** that runs atomically (all or nothing):

```solidity
function executeOperation(
    address asset,      // USDC (the borrowed token)
    uint256 amount,     // 10,000 USDC
    uint256 premium,    // ~50 USDC (0.05% fee)
    address initiator,  // Our contract address
    bytes calldata params  // Encoded (victim, collateralAsset)
) external override returns (bool) {
```

#### Step 1: Decode Parameters

```solidity
(address victim, address collateralAsset) = abi.decode(params, (address, address));
```

**Extracts:**
- `victim`: The address with unhealthy position
- `collateralAsset`: WETH (what we'll receive as collateral)

---

#### Step 2: Approve Liquidation

```solidity
IERC20(asset).approve(address(POOL), amount);
```

**Why?**
- We received 10,000 USDC from the flash loan
- We need to approve Aave Pool to spend it for liquidation
- This allows `liquidationCall()` to transfer our USDC

**State:**
- Contract balance: **10,000 USDC**
- Approved to POOL: **10,000 USDC**

---

#### Step 3: Execute Liquidation

```solidity
POOL.liquidationCall(
    collateralAsset,  // WETH (what victim used as collateral)
    asset,            // USDC (victim's debt)
    victim,           // Address to liquidate
    amount,           // 10,000 USDC (debt to cover)
    true              // Receive aToken (aWETH) instead of WETH
);
```

**What happens internally:**
1. Aave checks victim's position (must have HF < 1.0)
2. Takes our 10,000 USDC
3. Pays victim's debt (10,000 USDC)
4. Transfers collateral to us:
   - Base amount: ~10,000 USDC worth of WETH
   - Liquidation bonus: ~5% (500 USDC worth)
   - **Total received: ~10,500 aWETH** (as aToken)

**State after liquidation:**
- Contract balance: **~10,500 aWETH**
- Contract balance: **0 USDC** (used for liquidation)

**Problem:** We need USDC to repay the flash loan, but we only have aWETH!

---

#### Step 4: Calculate Repayment

```solidity
uint256 totalAmount = amount + premium;
// totalAmount = 10,000 + 50 = 10,050 USDC
```

**We need:**
- **10,050 USDC** to repay flash loan (10,000 + 50 premium)

**We have:**
- **10,500 aWETH** (collateral from liquidation)
- **0 USDC** (all used for liquidation)

---

#### Step 5: Repay Flash Loan

```solidity
uint256 contractBalance = IERC20(asset).balanceOf(address(this));

if (contractBalance >= totalAmount) {
    // We have enough USDC to repay
    IERC20(asset).approve(address(POOL), totalAmount);
} else {
    revert("Insufficient balance to repay flash loan...");
}
```

**Current Implementation:**
- Requires contract to be **pre-funded** with USDC
- Contract must have ≥ 10,050 USDC before liquidation
- Uses pre-funded USDC to repay flash loan

**What this means:**
```
Before flash loan:
  Contract balance: 10,100 USDC (pre-funded)
  
After liquidation:
  Contract balance: ~10,500 aWETH + 50 USDC (10,100 - 10,050)
  
After flash loan repayment:
  Contract balance: ~10,500 aWETH + 50 USDC
  
Profit: ~10,500 aWETH - gas costs
```

---

### Phase 3: Profit Withdrawal

After the flash loan transaction completes:

```solidity
// Option 1: Withdraw aToken directly
withdraw(aWETH_ADDRESS);
// Sends aWETH to owner

// Option 2: Redeem aToken to underlying asset
withdrawAToken(aWETH_ADDRESS);
// 1. Redeems aWETH → WETH
// 2. Sends WETH to owner
```

---

## 🔐 Key Security Features

### 1. Atomic Execution
```
✅ If ANY step fails → ENTIRE transaction reverts
✅ Flash loan must be repaid in same transaction
✅ No risk of losing funds mid-execution
```

### 2. Aave's Flash Loan Safety
```
✅ Aave only gives loan if executeOperation() succeeds
✅ If we don't repay → entire transaction reverts
✅ No way to "steal" flash loaned funds
```

### 3. Owner-Only Withdrawals
```solidity
require(msg.sender == owner, "Only owner");
// Only contract owner can withdraw profits
```

---

## 💡 Current Limitation & Solution

### ⚠️ Current Issue

The contract requires **pre-funding** with USDC because:
- Flash loan must be repaid in **same transaction**
- After liquidation, we have **aWETH** (not USDC)
- Can't swap aWETH → USDC within the same transaction (without DEX integration)

### ✅ Production Solution

To make it fully automatic, you'd add a **DEX swap** inside `executeOperation`:

```solidity
// Pseudo-code for production version:
function executeOperation(...) {
    // ... liquidation code ...
    
    // After receiving aWETH:
    uint256 totalAmount = amount + premium;
    
    // Swap aWETH to USDC using Uniswap
    swapOnUniswap(
        aWETH,           // From
        USDC,            // To
        enoughForRepayment // Amount
    );
    
    // Now we have USDC to repay
    IERC20(USDC).approve(address(POOL), totalAmount);
    
    return true;
}
```

---

## 📈 Profit Calculation Example

```
Scenario:
- Flash loan: 10,000 USDC
- Premium: 50 USDC (0.5%)
- Liquidation bonus: 5%
- Gas cost: ~$50 (0.02 ETH at $2,500/ETH)

Step-by-step:
1. Borrow: 10,000 USDC
2. Liquidate: Pay 10,000 USDC → Receive ~10,500 aWETH
3. Repay: 10,050 USDC (10,000 principal + 50 premium to Aave)
4. Gross profit: 10,500 aWETH (liquidation bonus)
5. Net profit: 10,500 aWETH - 50 USDC (premium) - gas costs (~$50 in ETH)
   = ~$450-500 worth of aWETH

Note: Premium (paid to Aave) and gas (paid to network) are separate costs.

Note: Actual profit depends on:
- Liquidation bonus percentage
- Gas prices
- Token prices
```

---

## 🎯 Summary

**The Flash Loan Magic:**

1. **Borrow** funds without collateral (flash loan)
2. **Use** borrowed funds to liquidate position
3. **Receive** collateral + bonus from liquidation
4. **Repay** flash loan + premium (in same transaction)
5. **Keep** the profit (bonus - premium - gas)
   - **Premium**: Fee paid to Aave for the flash loan (e.g., 0.05% of borrowed amount)
   - **Gas**: Network transaction fee (paid separately in ETH, not included in premium)

**Key Insight:**
Flash loans enable capital-efficient arbitrage because you don't need to own the capital upfront - you borrow, use it, and repay all in one atomic transaction!
