# Common Issues & Solutions

Quick reference for troubleshooting deployment and runtime issues.

---

## 🔴 Compilation Errors

### "Undeclared identifier"
**Solution:** Run `npx hardhat clean && npx hardhat compile`

### "Stack too deep"
**Solution:** Already fixed with `viaIR: true` in `hardhat.config.js`

### "Cannot find module"
**Solution:** Run `npm install`

---

## 🔴 Deployment Errors

### "Insufficient funds for gas"
```bash
# Check balance
npx hardhat run scripts/diagnose.js --network sepolia

# Get testnet ETH from faucets
# Mainnet: Ensure wallet has ETH
```

### "Network not found"
```bash
# Check network name matches exactly
# Available: mainnet, arbitrum, polygon, optimism, base, sepolia

# Verify in config
node -e "console.log(require('./config/networks').getSupportedNetworks())"
```

### "Invalid address" or "Invalid swap router"
```bash
# Check addresses in config/networks.js are correct
# Verify network matches (Sepolia vs Mainnet differ)
```

### "Transaction reverted"
- Check transaction on explorer for details
- Verify all parameters are correct
- Ensure contract has proper permissions

---

## 🔴 Runtime Errors

### "Position not liquidatable"
- Health factor may have recovered
- Position already liquidated by someone else
- Verify victim address is correct

### "Swap failed"
- Check DEX has liquidity
- Increase slippage tolerance
- Verify router address

### "Insufficient balance to repay"
- Flash loans should handle this automatically
- Check if swap is working
- Verify contract configuration

---

## 🛠️ Diagnostic Tools

### Run Full Diagnostics
```bash
npx hardhat run scripts/diagnose.js --network sepolia
```

### Check Compilation
```bash
npx hardhat clean
npx hardhat compile
```

### Verify Configuration
```bash
# Check network config
node -e "const {getNetworkConfig} = require('./config/networks'); console.log(JSON.stringify(getNetworkConfig('sepolia'), null, 2))"
```

---

## 📞 Need More Help?

**Share these details:**
1. Exact error message
2. Network (sepolia/mainnet/etc.)
3. Command you ran
4. Output of: `npx hardhat run scripts/diagnose.js --network <network>`

---

**Run diagnostics first:**
```bash
npx hardhat run scripts/diagnose.js --network sepolia
```
