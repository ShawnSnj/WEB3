## DeFi Lending – 7 Day Mini Course (Compound-Style)

This repo is a **guided 7‑day journey** to understand a simple Compound-style lending protocol, with:
- Daily **markdown notes**.
- An annotated **Solidity contract**: `contracts/SimpleLending.sol`.

The goal is to go from **high-level DeFi concepts** to reading and understanding a real (but simplified) lending protocol implementation.

---

### Folder structure

- `notes/`
  - `day1-defi-overview.md` – What is DeFi, what is a lending protocol, why Compound matters.
  - `day2-amm.md` – AMMs and price/oracle intuition for lending.
  - `day3-flashloan.md` – Flash loans and how they are used around lending protocols.
  - `day4-compound-overview.md` – How Compound is structured (cTokens, Comptroller).
  - `day5-compound-deep-dive.md` – Interest, indexes, collateral factors, liquidations.
  - `day6-build-lending.md` – Designing your own `SimpleLending` contract.
  - `day7-how-to-build-lending.md` – Walking through `SimpleLending.sol` line by line.

- `contracts/`
  - `SimpleLending.sol` – Single-asset, overcollateralized lending protocol (educational).

- `diagrams/`
  - `lending-architecture.md` – Space for your own diagrams and flow notes.

---

### How to use this repo

- **Day 1–3**: Focus on **concepts**:
  - Read the notes slowly.
  - Write your own summaries and attack scenarios (especially for flash loans).

- **Day 4–5**: Connect **real Compound** ideas to the simplified model:
  - Map each concept (cTokens, Comptroller, indexes, liquidation) to the simplified pieces we’ll use.

- **Day 6–7**: Dive into **code**:
  - Read `day6-build-lending.md`, then open `SimpleLending.sol`.
  - Use `day7-how-to-build-lending.md` to connect each function (deposit/withdraw, borrow/repay, accrueInterest, liquidate) back to the theory.

Recommended approach:
1. Each day, read the corresponding note.
2. Answer the “Your tasks for Day X” section in your own words.
3. Revisit `SimpleLending.sol` after Day 6–7 and try to re-implement parts of it yourself without looking.

---

### Important disclaimer

This code is:
- **For learning only**.
- Not audited, not optimized, and **not safe for production / mainnet**.

Use it to build intuition about:
- How lending protocols track deposits and borrows.
- How interest is accrued.
- How collateral, health factors, and liquidation fit together.
