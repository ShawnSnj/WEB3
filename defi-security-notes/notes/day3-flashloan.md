## Day 3 – Flash Loans & Why They Matter for Lending

**Goal**: Understand what flash loans are and how they are used in attacks (and arbitrage) around lending protocols.

---

### 1. What is a flash loan?

- **Flash loan**: A loan that:
  - Is taken and repaid **within the same transaction**.
  - Requires **no collateral** because if it’s not repaid by the end of the transaction, the whole transaction **reverts**.

- Typical flow:
  1. Borrow X tokens from flash-loan provider.
  2. Do some operations (trade on AMMs, interact with lending protocols, etc.).
  3. Repay X + fee.
  4. If step 3 fails, everything reverts as if nothing happened.

---

### 2. How flash loans interact with lending protocols

Because flash loans let you temporarily control **large amounts of capital**, they are used to:
- **Exploit weak oracles**:
  - Manipulate AMM prices using large swaps funded by a flash loan.
  - Make oracles read fake prices.
  - Borrow too much or liquidate others at a discount.

- **Perform arbitrage**:
  - Take advantage of price differences between AMMs and lending protocol liquidations.

---

### 3. Example attack pattern (high level)

1. Take a flash loan of a large asset amount.
2. Use it to push AMM price up/down.
3. Oracle (if badly designed) reads this **temporary** manipulated price.
4. Use lending protocol functions:
   - Borrow underpriced asset cheaply.
   - Or liquidate a victim’s collateral at a discount.
5. Reverse the AMM manipulation.
6. Repay the flash loan, keep the profit.

This is why lending protocols must:
- Use **robust oracles**,
- Apply **conservative collateral factors**,
- And carefully reason about **atomic composability** in DeFi.

---

### 4. Why we won’t implement flash loans in `SimpleLending`

Our `SimpleLending` contract will:
- Focus on core **lending mechanics**: deposit, borrow, repay, interest, liquidation.
- Not implement its own flash loans.

But when you see **atomic sequences of calls** in Solidity (all in one transaction), remember:
- Attackers can chain many protocols together in a single transaction.
- So you must design lending logic as if attackers always have access to **huge capital** via flash loans.

---

### 5. Your tasks for Day 3

1. Write a 5–10 line pseudocode of a simple flash-loan-based price manipulation attack.
2. Highlight in your pseudocode:
   - Where the **oracle** is read.
   - Where **borrow/repay** happens.
3. In your own words, explain why “needing a lot of money” is **not** a defense against DeFi attacks anymore.
