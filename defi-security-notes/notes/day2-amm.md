## Day 2 – AMMs & Price Oracles for Lending

**Goal**: Understand AMMs well enough to know where prices come from for lending protocols like Compound.

---

### 1. AMM basics

- **AMM (Automated Market Maker)**: A smart contract that lets people trade tokens against a **pool**, instead of an order book.
- The most common simple AMM is a **constant product** AMM (Uniswap v2 style):
  - Two token reserves: $x$ and $y$.
  - Invariant: $x \cdot y = k$ (constant).
  - Swaps move along this curve, updating $x$ and $y$, but $k$ stays roughly constant (ignoring fees).

The **price** of token X in terms of token Y is:
- $price = y / x$ (simplified).

---

### 2. Why lending protocols care about prices

Lending protocols need **prices** to:
- Measure the **value of collateral**.
- Measure the **value of debt**.
- Decide if a position is **safe** or needs **liquidation**.

If price data is wrong or manipulated:
- Borrowers might **steal funds** by borrowing too much.
- Honest users might be **unfairly liquidated**.

---

### 3. Oracles and AMMs

An **oracle** provides prices to the lending protocol.
- Can be:
  - **Off-chain** (Chainlink, Pyth, etc.).
  - **On-chain** using AMM data (e.g., Uniswap TWAP).

For this 7‑day course, to keep things simple:
- We’ll **assume** a stable, external price feed (or even `1:1` vs a stable asset).
- Our `SimpleLending` will treat 1 unit as “$1” for intuition.

You should still understand that **real Compound** uses secure oracles and is very careful about price manipulation.

---

### 4. Connecting AMMs to lending

Lending protocols often:
- Use **AMM prices** (or a smoothed version) as input for oracles.
- Are affected by trades:
  - If huge trades move prices in AMMs, oracle prices move.
  - This can change **collateral ratios** and trigger **liquidations**.

This is why flash loans + AMM price manipulation + weak oracles can be **dangerous** for lending protocols.

---

### 5. Security ideas to remember

- **Price manipulation**:
  - Attacker uses a big swap to push AMM price.
  - Oracle reads this manipulated price.
  - Attacker borrows too much or liquidates others unfairly.

- **Mitigations** (real protocols):
  - Use **TWAPs** (time-weighted average price).
  - Use **off-chain oracles** with many data sources.
  - Limit how quickly oracle prices can change.

In this repo we focus on understanding the **lending logic**, but always remember:
- **Bad price feeds = broken lending protocol**.

---

### 6. Your tasks for Day 2

1. Draw a simple AMM with reserves `x` and `y`, and write out $x \cdot y = k$.
2. Explain (in your own words) how moving along the curve changes price.
3. Write 3–5 sentences on why a **secure price oracle** is critical for a Compound-style protocol.
