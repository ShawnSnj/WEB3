## Day 1 – DeFi & Lending Overview

**Goal**: Understand what DeFi is, why lending protocols like Compound exist, and the core concepts you’ll use for the rest of the week.

---

### 1. What is DeFi?

- **DeFi (Decentralized Finance)**: Financial applications built on blockchains (mostly Ethereum) that:
  - Run on smart contracts instead of centralized servers.
  - Let anyone with a wallet interact (no KYC by default).
  - Are transparent (code and state are on-chain).

- **Key properties**:
  - **Permissionless**: Anyone can supply, borrow, trade.
  - **Non-custodial**: You keep control of your funds through your private keys.
  - **Composability**: Protocols are “money legos” that can be combined.

---

### 2. What is a lending protocol?

A **lending protocol** is a smart contract where:
- Users **supply** assets to earn interest.
- Other users **borrow** assets by posting **collateral**.
- The protocol automatically:
  - Tracks each user’s **deposits (supplied)** and **debts (borrowed)**.
  - Calculates **interest** over time.
  - Handles **liquidations** when a borrower is too risky.

You can think of it as an **on-chain bank**, but:
- There is **no bank**; just smart contracts.
- The rules are **code** (open source, verifiable).

---

### 3. Why Compound-style protocols are important

Compound popularized:
- **Pooled liquidity**: Everyone supplies to a single pool per asset.
- **Algorithmic interest rates**: Rates change automatically based on supply/borrow demand.
- **Tokenized deposit receipts (cTokens)**: When you supply assets, you receive a token that represents your share in the pool.

For this 7‑day mini-course, we will build a **simplified Compound-style lending protocol** called `SimpleLending`:
- No complex interest rate models.
- No multiple assets (we’ll focus on a single token or ETH-like asset).
- Clear, commented code so you can map **every function** to the lending concepts.

---

### 4. Core concepts you must know

You’ll see these terms repeatedly:

- **Collateral**:
  - Assets locked by a user to secure a loan.
  - If your collateral value isn’t high enough vs your debt, you can be **liquidated**.

- **Borrow**:
  - Taking assets from the pool, increasing your **debt**.
  - Requires enough collateral.

- **Health factor / collateral ratio** (simplified):
  - Measures how safe a borrower is.
  - Typical idea: `collateral_value / borrowed_value` must stay above some threshold (e.g. 150%).

- **Interest accrual**:
  - Over time, both:
    - Suppliers earn more.
    - Borrowers owe more.
  - In smart contracts this is usually done via:
    - Per-block or per-second **interest rate**.
    - Updating user balances based on **time elapsed** since last update.

- **Liquidation**:
  - If a borrower becomes too risky (collateral too low vs debt):
    - A **liquidator** repays some of the debt.
    - In return, they get some of the borrower’s collateral at a **discount**.

---

### 5. What we’ll build this week

We will implement a **single-asset, overcollateralized lending protocol** with:
- **Supply / withdraw** functions.
- **Borrow / repay** functions.
- A simple **interest model**.
- A minimal **liquidation** mechanism.

All of this will live in `contracts/SimpleLending.sol` and be explained day by day.

---

### 6. Mapping concepts to functions (high level)

In `SimpleLending.sol` you will eventually see functions like:
- `deposit()` / `withdraw()`: Manage supplied collateral.
- `borrow()` / `repay()`: Manage debt.
- `accrueInterest()`: Update interest based on elapsed time.
- `liquidate()` (optional, later in the week): Allow others to liquidate unsafe positions.

On later days we’ll refine the exact function names, but the key idea is:
- **Every concept** above will map to **one or more concrete functions** in the contract.

---

### 7. Your tasks for Day 1

1. **Read this file once slowly.**
2. **Write your own 3–5 sentence summary** (in your own words) of:
   - What a lending protocol is.
   - Why collateral and liquidations exist.
3. Open `SimpleLending.sol` (it will be filled in during later steps) and skim its comments once they appear, mapping:
   - High-level ideas (deposit, borrow, interest, liquidation)
   - To concrete functions.

Starting tomorrow, we’ll go deeper into AMMs, flash loans, and then zoom back into Compound and our own `SimpleLending` implementation.
