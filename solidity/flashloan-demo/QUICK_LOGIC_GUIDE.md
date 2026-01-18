# Quick Logic Guide - Flash Loan Execution

## 🚀 The Main Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    THE FLASH LOAN CYCLE                     │
└─────────────────────────────────────────────────────────────┘

USER CALLS: requestLiquidationLoan()
    │
    ├─> Encodes parameters (victim, collateral)
    │
    └─> POOL.flashLoanSimple()
            │
            ├─> Aave transfers USDC to contract
            │
            └─> Aave calls: executeOperation()
                    │
                    ┌──────────────────────────────────┐
                    │  executeOperation() - ATOMIC     │
                    │  (All steps must succeed!)       │
                    └──────────────────────────────────┘
                    │
                    ├─> Step 1: Approve USDC for liquidation
                    │
                    ├─> Step 2: Liquidate position
                    │   └─> Pay debt → Get collateral + bonus
                    │
                    ├─> Step 3: Calculate repayment (amount + premium)
                    │
                    ├─> Step 4: Repay flash loan
                    │   └─> Must repay in same transaction!
                    │
                    └─> Return true (success)
                            │
                            └─> Transaction completes
                                └─> Profit remains in contract
```

---

## 📝 Code Walkthrough

### Entry Point: `requestLiquidationLoan()`

```solidity
function requestLiquidationLoan(...) public {
    bytes memory params = abi.encode(_victim, _collateralAsset);
    POOL.flashLoanSimple(address(this), _token, _amount, params, 0);
    //                                  ↑
    //                         Our contract receives the loan
    //                         and must implement executeOperation()
}
```

**What it does:**
- Prepares parameters
- Requests flash loan from Aave
- **Returns immediately** - execution happens in `executeOperation()`

---

### Core Logic: `executeOperation()`

This function is called **by Aave** after giving us the flash loan.

```solidity
function executeOperation(
    address asset,      // ← USDC (what we borrowed)
    uint256 amount,     // ← 10,000 USDC
    uint256 premium,    // ← ~50 USDC (fee)
    address initiator,  
    bytes calldata params  // ← Encoded (victim, collateralAsset)
) external override returns (bool) {
```

#### 🔹 Step 1: Decode Parameters

```solidity
(address victim, address collateralAsset) = abi.decode(params, (address, address));
```

**Extracts:**
- `victim`: Who to liquidate
- `collateralAsset`: What we'll receive (WETH)

---

#### 🔹 Step 2: Approve for Liquidation

```solidity
IERC20(asset).approve(address(POOL), amount);
//      ↑              ↑
//   USDC token    Allow Aave Pool to spend our USDC
```

**Current state:**
- ✅ Contract has: **10,000 USDC** (from flash loan)
- ✅ Approved POOL to spend: **10,000 USDC**

---

#### 🔹 Step 3: Execute Liquidation

```solidity
POOL.liquidationCall(
    collateralAsset,  // WETH (victim's collateral)
    asset,            // USDC (victim's debt)  
    victim,           // Who to liquidate
    amount,           // How much debt to cover (10,000 USDC)
    true              // Receive aToken (aWETH) not raw WETH
);
```

**What happens:**
```
Input:  10,000 USDC
        ↓
        Pay victim's debt
        ↓
Output: ~10,500 aWETH (10,000 + 5% liquidation bonus)
```

**New state:**
- ✅ Contract has: **~10,500 aWETH**
- ❌ Contract has: **0 USDC** (used for liquidation)

---

#### 🔹 Step 4: Calculate Repayment

```solidity
uint256 totalAmount = amount + premium;
//                  10,000 + 50 = 10,050 USDC
```

**Problem:** We need **10,050 USDC** but we only have **aWETH**!

---

#### 🔹 Step 5: Repay Flash Loan

```solidity
uint256 contractBalance = IERC20(asset).balanceOf(address(this));
// Check if we have enough USDC

if (contractBalance >= totalAmount) {
    // ✅ We have enough (from pre-funding)
    IERC20(asset).approve(address(POOL), totalAmount);
    // When function returns, Aave automatically takes this
} else {
    // ❌ Not enough USDC → Revert entire transaction
    revert("Insufficient balance...");
}
```

**Key Point:**
- When `executeOperation()` returns `true`, Aave automatically transfers the approved amount
- If we revert here, the **entire flash loan transaction reverts**
- No funds are lost - everything happens atomically

---

## 💰 Money Flow Example

```
BEFORE FLASH LOAN:
Contract: 10,100 USDC (pre-funded)
Victim:   100,000 USDC debt, 20 WETH collateral

DURING FLASH LOAN:
1. Borrow:      +10,000 USDC (from Aave)
   Balance:     20,100 USDC

2. Liquidate:   -10,000 USDC → +10,500 aWETH
   Balance:     10,100 USDC + 10,500 aWETH

3. Repay:       -10,050 USDC (to Aave)
   Balance:     50 USDC + 10,500 aWETH

AFTER FLASH LOAN:
Contract: 50 USDC + 10,500 aWETH
Profit:   ~10,500 aWETH worth (~$26,250 at $2,500/WETH)
          Minus gas costs
```

---

## 🔑 Key Concepts

### 1. Atomic Execution
- **All steps** happen in **one transaction**
- If **any step fails**, **everything reverts**
- No partial state - it's all or nothing

### 2. Flash Loan Requirements
- Must **repay** in the **same transaction**
- If you don't repay → transaction **reverts**
- This is enforced by Aave's smart contract

### 3. Current Implementation Limitation
- Requires **pre-funding** with USDC
- Can't swap aWETH → USDC in same transaction (without DEX)
- For production, add Uniswap swap inside `executeOperation()`

---

## 🎯 Why This Works

1. **No upfront capital needed** - borrow via flash loan
2. **Liquidation bonus** - receive more collateral than debt paid
3. **Atomic execution** - safe and risk-free (reverts if fails)
4. **Capital efficient** - use borrowed funds, keep profit

---

## 🔄 Production Improvements

To make it fully automatic:

```solidity
function executeOperation(...) {
    // ... liquidation code ...
    
    // After liquidation, we have aWETH
    // Need to swap to USDC to repay
    
    // Option 1: Use Uniswap V3 swap
    swapOnUniswapV3(aWETH, USDC, amountNeededForRepayment);
    
    // Option 2: Use aToken directly (if Aave allows)
    // Some versions allow using collateral to repay
    
    // Now we have USDC
    IERC20(USDC).approve(address(POOL), totalAmount);
    return true;
}
```

This would eliminate the need for pre-funding!
