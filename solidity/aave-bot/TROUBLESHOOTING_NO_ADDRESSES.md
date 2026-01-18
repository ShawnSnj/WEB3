# Troubleshooting: "No addresses to monitor yet..."

## Problem

The bot shows "No addresses to monitor yet..." repeatedly, meaning it hasn't found any Aave positions to monitor.

---

## Why This Happens

### On Testnets (Sepolia, etc.)

**Testnets have very little Aave activity:**
- Few users borrow/lend on testnets
- Historical blocks may have zero Aave events
- This is **normal** for testnets

### On Mainnet

- Could indicate a configuration issue
- Or genuinely low activity in the lookback period

---

## Solutions

### 1. Increase Historical Lookback

Increase the number of blocks to scan:

```env
# In aave-bot/.env
HISTORICAL_BLOCKS_LOOKBACK=10000  # Scan last 10k blocks (was 1000)
```

**For Sepolia:** Try 50,000+ blocks to find any activity.

### 2. Wait for Real-Time Discovery

The bot automatically discovers new addresses when:
- Someone supplies collateral
- Someone borrows
- Someone withdraws/repays

**Just wait** - the bot will find addresses as they appear.

### 3. Manually Add Addresses (For Testing)

You can manually add addresses to monitor by modifying the code temporarily, or wait for the real-time discovery to catch new events.

### 4. Check Pool Address

Verify your `POOL_ADDRESS` is correct for your network:

**Sepolia:**
```
POOL_ADDRESS=0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951
```

**Mainnet:**
```
POOL_ADDRESS=0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2
```

---

## Enhanced Logging

The bot now shows:
- How many event logs were found in historical scan
- How many unique addresses discovered
- Real-time event discovery notifications
- New address discoveries

**Example output:**
```
Scanning historical events from block 5000000 to 5001000
Pool address: 0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951
Found 0 event logs in historical scan
Found 0 unique addresses with Aave positions
⚠️  No addresses found. This could mean:
   - No Aave activity in the last 1000 blocks
   - Try increasing HISTORICAL_BLOCKS_LOOKBACK in .env
   - Or wait for real-time event discovery to find new positions
```

---

## Quick Fix for Sepolia

```env
# Scan much further back
HISTORICAL_BLOCKS_LOOKBACK=50000

# Or just wait - real-time discovery will catch new positions
```

---

## Verify It's Working

1. **Check logs** - You should see:
   - "Found X event logs in historical scan"
   - "Found X unique addresses"
   - Real-time discovery messages

2. **Wait for activity** - On testnets, you may need to wait for someone to interact with Aave

3. **Test with known address** - If you know an address with an Aave position, the bot will discover it when that address interacts with Aave

---

## Summary

**For Testnets:**
- ✅ This is normal - very little activity
- ✅ Increase `HISTORICAL_BLOCKS_LOOKBACK` to 50,000+
- ✅ Or wait for real-time discovery

**For Mainnet:**
- ✅ Check `POOL_ADDRESS` is correct
- ✅ Increase lookback if needed
- ✅ Real-time discovery will catch new positions

The bot is working correctly - it's just waiting for Aave activity to discover! 🎯
