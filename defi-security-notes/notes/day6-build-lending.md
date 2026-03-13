## Day 6 – Designing `SimpleLending` (Data Structures & Functions)

**Goal**: Design the storage and function interfaces for your own simple Compound-style lending contract.

---

### 1. Data we need to track

For a **single-asset** lending protocol, we need at least:

- **Global state**:
  - `totalDeposits` – total amount supplied.
  - `totalBorrows` – total amount borrowed.
  - `interestRatePerSecond` – fixed borrow interest rate.
  - `lastAccrualTimestamp` – last time we updated interest.

- **Per-user state**:
  - `deposits[user]` – how much the user has supplied.
  - `borrows[user]` – how much the user has borrowed (principal; we’ll apply interest).

- **Risk parameters**:
  - `collateralFactor` – e.g. 75% (stored as a scaled factor).
  - `liquidationBonus` – extra collateral a liquidator gets (e.g. 10%).

All of these will be simple Solidity variables in `SimpleLending.sol`.

---

### 2. Core functions (high level)

We’ll implement the following **core external functions**:

- **Supply-side**:
  - `deposit(uint256 amount)` – user deposits tokens into the pool.
  - `withdraw(uint256 amount)` – user withdraws tokens (if they are not over-borrowed).

- **Borrow-side**:
  - `borrow(uint256 amount)` – user borrows from the pool (if enough collateral).
  - `repay(uint256 amount)` – user repays their debt.

- **Protocol mechanics**:
  - `accrueInterest()` – updates `totalBorrows` based on time elapsed.
  - `getHealthFactor(address user)` – views whether user is safe.
  - `liquidate(address borrower, uint256 repayAmount)` – liquidate unsafe borrowers.

We will also need internal helpers:
- `_updateInterest()` – logic for `accrueInterest()`.
- `_isSafe(user)` – internal check used after operations.

---

### 3. Flow for each major function

**`deposit(amount)`**:
1. Call `accrueInterest()` to keep state fresh.
2. Transfer tokens from user to contract.
3. Increase `deposits[user]` and `totalDeposits`.

**`withdraw(amount)`**:
1. Call `accrueInterest()`.
2. Decrease `deposits[user]` by `amount`.
3. Check if user remains **safe** (`collateral * collateralFactor >= borrow`).
4. If safe, transfer tokens to user; else revert.

**`borrow(amount)`**:
1. Call `accrueInterest()`.
2. Increase `borrows[user]` and `totalBorrows`.
3. Check that user is still **within collateral limits**.
4. Transfer tokens to user.

**`repay(amount)`**:
1. Call `accrueInterest()`.
2. Transfer tokens from user to contract.
3. Reduce `borrows[user]` and `totalBorrows` (not going below zero).

**`liquidate(borrower, repayAmount)`**:
1. Call `accrueInterest()`.
2. Check borrower is **unsafe**.
3. Transfer `repayAmount` from liquidator to protocol.
4. Reduce borrower’s debt.
5. Calculate collateral to seize = `repayAmount * (1 + liquidationBonus)`.
6. Move seized collateral from borrower’s deposits to liquidator.

---

### 4. Security considerations (even for a toy project)

Even in this simple design, watch out for:
- **Reentrancy**:
  - Use checks-effects-interactions pattern.
  - Consider a simple reentrancy guard if extending this.

- **Integer math**:
  - Use Solidity 0.8+ built-in overflow checks (we will).
  - Carefully handle scaling factors (e.g. 1e18).

- **Access control**:
  - For learning, we’ll make most functions public/externally callable.
  - In real lending protocols, some parameters are only settable by governance.

---

### 5. Your tasks for Day 6

1. On paper or in a note, write the Solidity-like struct of state:
   - All global variables.
   - All mappings.
2. Write the function signatures you expect to see in `SimpleLending.sol`.
3. Trace one full example:
   - User deposits 100.
   - Borrows 50.
   - Time passes, interest accrues.
   - User repays and withdraws.
