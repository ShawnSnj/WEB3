## Day 7 – Walking Through `SimpleLending.sol` Line by Line

**Goal**: Connect each important Solidity function to the DeFi concepts you learned and understand the full flow end-to-end.

> Note: This day assumes `contracts/SimpleLending.sol` is implemented with the functions described in Day 6.

---

### 1. Contract setup

Key pieces you will see at the top of the contract:
- **Pragma + imports** – Solidity version and ERC20 interface.
- **State variables**:
  - `IERC20 public immutable asset;`
  - `uint256 public totalDeposits;`
  - `uint256 public totalBorrows;`
  - `uint256 public interestRatePerSecond;`
  - `uint256 public lastAccrualTimestamp;`
  - `uint256 public collateralFactor;`
  - `uint256 public liquidationBonus;`

- **Mappings**:
  - `mapping(address => uint256) public deposits;`
  - `mapping(address => uint256) public borrows;`

**Concept mapping**:
- State variables = protocol state.
- Mappings = per-user positions (collateral + debt).

---

### 2. `accrueInterest()`

This function:
- Updates **global** borrow totals based on time.
- Ensures that all subsequent actions use **fresh interest**.

Typical logic:
1. If `block.timestamp == lastAccrualTimestamp`, do nothing.
2. Compute `timeElapsed = block.timestamp - lastAccrualTimestamp`.
3. Compute `interest = totalBorrows * interestRatePerSecond * timeElapsed / 1e18`.
4. Increase `totalBorrows` by `interest`.
5. Update `lastAccrualTimestamp`.

**Concept mapping**:
- Models **continuous borrowing cost**.
- Suppliers earn implicitly because the pool’s assets have interest embedded in them.

---

### 3. `deposit()` and `withdraw()`

**`deposit(uint256 amount)`**:
- User sends tokens to the protocol.
- `deposits[user]` and `totalDeposits` go up.

**`withdraw(uint256 amount)`**:
- User asks to remove some of their supplied assets.
- Contract:
  1. Accrues interest.
  2. Decreases `deposits[user]`.
  3. Checks if user is still **safe** given their borrow.
  4. Transfers tokens out if safe; revert otherwise.

**Concept mapping**:
- **Deposit** = add collateral to your account.
- **Withdraw** = remove collateral, but only if your **health factor** stays OK.

---

### 4. `borrow()` and `repay()`

**`borrow(uint256 amount)`**:
- Steps:
  1. Accrue interest.
  2. Increase `borrows[user]` and `totalBorrows`.
  3. Check that `collateral * collateralFactor >= borrow`.
  4. Transfer tokens to the user.

**`repay(uint256 amount)`**:
- Steps:
  1. Accrue interest.
  2. Transfer tokens from user to protocol.
  3. Reduce `borrows[user]` and `totalBorrows`.

**Concept mapping**:
- **Borrow**: turns some of your collateral power into a loan.
- **Repay**: reduces your debt and improves your health factor.

---

### 5. Health checks & `getHealthFactor()`

`getHealthFactor(user)` (or equivalent internal logic) will:
- Compute:
  - `collateralValue = deposits[user]`
  - `borrowValue = borrows[user]`
  - `maxBorrowAllowed = collateralValue * collateralFactor / 1e18`
- Return a ratio or boolean:
  - e.g. `health = maxBorrowAllowed * 1e18 / borrowValue`
  - Or simply “isSafe = maxBorrowAllowed >= borrowValue”.

**Concept mapping**:
- Health factor > 1 ⇒ safe.
- Health factor < 1 ⇒ liquidatable.

---

### 6. `liquidate()`

`liquidate(address borrower, uint256 repayAmount)`:
- Called by a **liquidator**.
- Flow (simplified):
  1. Accrue interest.
  2. Check borrower is **unsafe**.
  3. Transfer `repayAmount` from liquidator to protocol.
  4. Reduce borrower’s debt.
  5. Compute collateral to seize:
     - `seize = repayAmount * (1e18 + liquidationBonus) / 1e18`
  6. Move collateral from `deposits[borrower]` to `deposits[liquidator]`.

**Concept mapping**:
- Liquidators stabilize the system by closing unsafe positions.
- They are rewarded with **discounted collateral**.

---

### 7. Your final tasks for Day 7

1. Open `SimpleLending.sol` and for **every external function** write:
   - One line: **“This function represents X concept from Compound / lending theory”**.
2. Simulate (on paper or Hardhat/Foundry):
   - User A deposits.
   - User A borrows.
   - Time passes, interest accrues.
   - User A becomes unsafe.
   - User B liquidates user A.
3. Note any questions where the toy design differs from real Compound; those are great areas to study next.
