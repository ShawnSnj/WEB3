## Day 4 – Compound Overview (Architecture & Flows)

**Goal**: See how a real protocol like Compound is structured, and connect high-level ideas to contract functions.

---

### 1. High-level architecture

In (simplified) Compound:
- There is a **cToken contract per asset** (e.g. `cDAI`, `cETH`).
- There is a central **Comptroller** contract that:
  - Knows which markets exist.
  - Tracks which assets each user has supplied/borrowed.
  - Enforces **risk parameters** (collateral factors, close factors, etc.).

Each **cToken**:
- Holds the underlying asset in its balance.
- Lets users **mint** (supply) and **redeem** (withdraw) via cTokens.
- Implements **borrow**, **repay**, **liquidateBorrow**, and **interest accrual**.

---

### 2. Key data tracked per user

For each user, the protocol conceptually tracks:
- **Supplied balances** per asset.
- **Borrowed balances** per asset.
- **Account liquidity**:
  - How much they can still safely borrow.
  - Or how much shortfall they have (if unsafe).

Core idea:
- The Comptroller aggregates:
  - `sum(collateral_value * collateral_factor)` across all markets
  - `sum(borrow_value)` across all markets
- And enforces:
  - `collateral_value_adjusted >= borrow_value`

---

### 3. Key functions (simplified)

In actual Compound (cTokens + Comptroller), you’ll find functions like:
- **Supply-side**:
  - `mint()` – supply underlying, receive cTokens.
  - `redeem()` – burn cTokens, receive underlying.

- **Borrow-side**:
  - `borrow()` – take underlying out, increasing your borrow balance.
  - `repayBorrow()` – pay back some/all of your borrow.

- **Risk & interest**:
  - `accrueInterest()` – update total borrows and indexes over time.
  - `liquidateBorrow()` – liquidate an unsafe borrower.
  - `getAccountLiquidity()` – check if an account is safe.

Our `SimpleLending` will **compress** these ideas into a **single contract** with fewer moving parts, but mapped concepts will be similar.

---

### 4. Interest rate model (very high level)

Compound uses:
- A **separate interest rate model contract** per market.
- That model takes:
  - `utilization = total_borrow / total_supply`
  - And returns:
  - `borrowRate`, `supplyRate`.

Rates are usually:
- **Piecewise-linear** curves.
- Higher utilization ⇒ higher rates.

For `SimpleLending`, we’ll:
- Use a **fixed rate** or a very simple formula.
- Focus on how interest **accrues over time** into balances.

---

### 5. How this maps to `SimpleLending`

In `SimpleLending.sol` we will:
- Combine:
  - Pool + user accounting + interest into one contract.
- Implement:
  - `deposit()` / `withdraw()`.
  - `borrow()` / `repay()`.
  - `accrueInterest()` / simple per-second interest.
  - `liquidate()` in a very simplified way.

You should mentally map:
- **cToken + Comptroller** ⇒ **SimpleLending single contract**.
- **mint/redeem** ⇒ `deposit/withdraw`.
- **borrow/repayBorrow** ⇒ `borrow/repay`.
- **getAccountLiquidity** ⇒ internal health check logic.

---

### 6. Your tasks for Day 4

1. Skim official Compound docs or a summary and list:
   - cToken responsibilities.
   - Comptroller responsibilities.
2. Draw a simple diagram with:
   - User ↔ cToken ↔ Comptroller.
3. Write a short paragraph explaining how this will become a **single-contract** design in `SimpleLending`.
