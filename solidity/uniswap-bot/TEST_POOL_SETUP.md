# Test Pool Setup Complete! ✅

## Your Test Pool Details

**Pool Address:** `0x5bcd905f69bf16D8C36cd276c7FA1b857169F938`

**Token Addresses:**
- **TestWETH (tWETH):** `0x36043011d9e0d40625b8bf37DD0bF9BccC1d75a9`
- **TestUSDC (tUSDC):** `0x7638A54c381bF1207d8c0A1F6351292F94233154`

## Update Bot Configuration

Update your `uniswap-bot/.env`:

```env
# Use your test token addresses
MONITOR_PAIRS=0x36043011d9e0d40625b8bf37DD0bF9BccC1d75a9,0x7638A54c381bF1207d8c0A1F6351292F94233154

# Sepolia configuration
RPC_URL=https://eth-sepolia.g.alchemy.com/v2/YOUR_API_KEY
UNISWAP_V2_ROUTER=0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008
```

## Test the Bot

```bash
cd uniswap-bot
go run .
```

The bot should now:
- ✅ Detect the pool
- ✅ Get prices successfully
- ✅ Monitor for arbitrage opportunities

## What Was Created

1. **Test Tokens:**
   - TestWETH: 1000 tokens (18 decimals)
   - TestUSDC: 1,000,000 tokens (6 decimals)

2. **Uniswap Pool:**
   - Pair: tWETH/tUSDC
   - Initial liquidity: 500 tWETH + 500 tUSDC
   - Pool address: `0x5bcd905f69bf16D8C36cd276c7FA1b857169F938`

## Next Steps

1. ✅ Update bot's `.env` with test token addresses
2. ✅ Restart the bot
3. ✅ Verify prices are being detected
4. ✅ Test swap execution (with `ENABLE_AUTO_SWAP=true`)

## Notes

- Pool has initial liquidity, so prices should be detectable
- You can add more liquidity later if needed
- These are test tokens - only use on Sepolia!
- For mainnet, use real token addresses
