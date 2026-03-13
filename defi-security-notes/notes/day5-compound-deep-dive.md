## Day 5 – Compound Deep Dive (Interest, Indexes, Liquidations)

**Goal**: Understand the more detailed mechanics of Compound so our simplified version still “feels right”.

---

### 1. Interest accrual using indexes

Compound doesn’t store a constantly-updating balance for every user on every block.
Instead it uses **indexes**:

- There is a **borrow index** that grows over time as interest accrues.
- Each user stores:
  - Their **principal borrow**.
  - The **borrow index** at the last time they changed their position.

When you need the current borrow balance:
- You compare:
  - Current global `borrowIndex`
  - Vs the user’s stored `borrowIndex`
- And scale the principal:
  - `currentBorrow = principal * (borrowIndex_now / borrowIndex_user)`

This pattern:
- Avoids looping over all users to update interest.
- Only updates per-user balances **on demand**.

In `SimpleLending`, we’ll implement a **simpler version**:
- We can store:
  - `lastAccrualTimestamp`.
  - Global `totalBorrow` and `totalSupply`.
- And whenever we call `accrueInterest()`:
  - Compute `timeElapsed = block.timestamp - lastAccrualTimestamp`.
  - Increase `totalBorrow` by `totalBorrow * ratePerSecond * timeElapsed`.
  - Optionally mirror that to users via indexes or simple per-user updates.

---

### 2. Collateral factors and account liquidity

Compound sets a **collateral factor** per asset (e.g. 75%).

For each user:
- It computes:
  - `collateral_value_adjusted = sum( collateral_value * collateral_factor )`
  - `borrow_value = sum( borrow_value )`
- If:
  - `collateral_value_adjusted >= borrow_value` ⇒ safe.
  - Else ⇒ unsafe (liquidatable).

In `SimpleLending` we’ll:
- Treat our single asset as its own collateral and borrow asset.
- Use:
  - A single `collateralFactor` (e.g. 75%).
  - A simple health check:
    - `collateral_value * collateralFactor >= borrow_value`.

---

### 3. Liquidation flow (simplified)

Real Compound:
- Allows a **liquidator** to repay part of a borrower’s debt.
- In return, the liquidator gets some of the borrower’s **collateral** at a **discount**.
- Has:
  - **Close factor**: max % of debt that can be repaid in one liquidation.
  - **Liquidation incentive**: discount on collateral.

In `SimpleLending`, we’ll implement a simpler version:
- Any caller can:
  - Call `liquidate(borrower, repayAmount)`.
- Logic (high-level):
  1. Check if `borrower` is **unsafe**.
  2. Transfer `repayAmount` from liquidator to the pool.
  3. Reduce borrower’s debt by `repayAmount`.
  4. Seize some amount of borrower’s collateral and send to liquidator.

We’ll pick fixed constants for:
- `collateralFactor`.
- `liquidationBonus` (e.g. 10% extra collateral).

---

### 4. How we’ll simplify in `SimpleLending`

To keep the contract short and readable, we will:
- Use one asset only.
- Use a **fixed interest rate** per year, converted to per-second.
- Store:
  - User `deposits[user]`.
  - User `borrows[user]`.
- Implement:
  - `accrueInterest()` that updates global totals and (for learning) may update user borrows directly for simplicity.
- Implement:
  - A simple `getHealthFactor(user)` or inline health check.

The focus is:
- Learn how the math & checks work.
- Not to perfectly match Compound’s gas-optimized design.

---

### 5. Your tasks for Day 5

1. Write a short summary (3–5 lines) of how **indexes** help avoid looping over all users.
2. For a fixed annual rate `r`, derive a simple `ratePerSecond = r / secondsPerYear`.
3. Sketch pseudocode for a `liquidate()` function based on:
   - Check unsafe.
   - Repay debt.
   - Seize collateral + bonus.
