# Deployment Troubleshooting Guide

Common deployment issues and solutions.

---

## ❌ Compilation Errors

### Error: "Undeclared identifier: requestLiquidationLoan"

**Cause:** Function called before declaration.

**Fix:** Functions are now in correct order. If you see this, ensure `requestLiquidationLoan` is defined before `requestLiquidationLoanSimple`.

---

### Error: "Wrong argument count for modifier invocation: Ownable"

**Cause:** OpenZeppelin v4's `Ownable` constructor doesn't take parameters.

**Fix:** Changed to `Ownable()` and use `_transferOwnership(msg.sender)` in constructor.

---

### Error: "Different number of components on the left hand side (12) than on the right hand side (1)"

**Cause:** `getReserveData()` returns a struct, not individual values.

**Fix:** Use struct access:
```solidity
DataTypes.ReserveData memory reserveData = POOL.getReserveData(collateralAsset);
address aTokenAddress = reserveData.aTokenAddress;
```

---

### Error: "Stack too deep"

**Cause:** Too many local variables in a function.

**Fix:** Enabled `viaIR: true` in `hardhat.config.js`:
```javascript
solidity: {
  settings: {
    viaIR: true, // Fixes stack too deep errors
  }
}
```

---

## ❌ Deployment Errors

### Error: "Insufficient funds for gas"

**Solution:**
- Get more ETH/network tokens from faucet
- Check wallet balance: `npx hardhat run scripts/checkBalance.js --network sepolia`
- Reduce gas limit if needed

---

### Error: "Network not found" or "Unknown network"

**Solution:**
- Check network name matches exactly: `sepolia`, `mainnet`, `arbitrum`
- Verify network is in `config/networks.js`
- Check `hardhat.config.js` has network configured

---

### Error: "Contract deployment failed: nonce too high"

**Solution:**
- Wait a few seconds and try again
- Check if previous transaction is still pending
- Use a fresh wallet if needed

---

### Error: "Invalid address" or "Invalid swap router"

**Solution:**
- Verify addresses in `config/networks.js` are correct for your network
- Check network matches (Sepolia vs Mainnet addresses differ)
- Ensure addresses are checksummed correctly

---

## ❌ Runtime Errors

### Error: "Transaction reverted"

**Common Causes:**
1. **Insufficient balance** - Contract needs tokens for edge cases
2. **Invalid parameters** - Check victim address, token addresses
3. **Position already liquidated** - Someone else liquidated first
4. **Network mismatch** - Contract on wrong network

**Debug:**
- Check transaction on explorer
- Review contract events
- Verify all addresses are correct

---

### Error: "Swap failed" or "Insufficient liquidity"

**Solution:**
- Check DEX has liquidity for token pair
- Increase slippage tolerance
- Try different swap path
- Verify router address is correct

---

## 🔧 Quick Fixes

### Recompile Contract
```bash
npx hardhat clean
npx hardhat compile
```

### Check Configuration
```bash
# Verify network config
node -e "const {getNetworkConfig} = require('./config/networks'); console.log(getNetworkConfig('sepolia'))"
```

### Verify Deployment
```bash
# Check contract on explorer
# Verify constructor parameters match
npx hardhat verify --network sepolia <CONTRACT> <ARGS...>
```

---

## 📝 Common Issues Checklist

- [ ] Contract compiles successfully
- [ ] Network configured in `hardhat.config.js`
- [ ] Network addresses correct in `config/networks.js`
- [ ] Private key set in `.env`
- [ ] RPC URL working and accessible
- [ ] Sufficient balance for gas
- [ ] All addresses are checksummed
- [ ] Network name matches exactly

---

## 🆘 Still Having Issues?

1. **Check Error Message** - Read the full error output
2. **Verify Network** - Ensure you're on the right network
3. **Check Balance** - Ensure sufficient funds
4. **Review Logs** - Check transaction on explorer
5. **Test Locally** - Try Hardhat fork first

---

**Most issues are resolved by:**
- ✅ Recompiling with `viaIR: true`
- ✅ Checking network configuration
- ✅ Verifying addresses are correct
- ✅ Ensuring sufficient balance
