# Cross-DEX Arbitrage Bot (MEV-Aware)

Go bot that monitors two constant-product AMM pools for price discrepancies, computes optimal trade size off-chain, **simulates execution via Flashbots `CallBundle`**, and submits bundles only when the simulated trade is profitable.

Includes toy Solidity contracts (`SimpleDEX`, `FlashArb`) for local Hardhat testing.

Part of the [WEB3](../README.md) portfolio.

---

## How it works

```
┌─────────┐     poll reserves      ┌──────────┐
│  Go bot │ ─────────────────────► │  DEX 1   │
│         │ ─────────────────────► │  DEX 2   │
└────┬────┘                        └──────────┘
     │
     │ 1. OptimalDx sizing + off-chain profit estimate
     │ 2. Build signed tx → FlashArb.executeArbitrage()
     │ 3. flashbots_callBundle (simulate)
     │ 4. flashbots_sendBundle (only if sim succeeds)
     ▼
┌─────────────┐
│  Flashbots  │
│   relay     │
└─────────────┘
```

1. Fetch `reserveX` / `reserveY` from both pools at the latest block
2. Compute optimal input size with constant-product math (`OptimalDx`)
3. Skip if estimated profit is below `min_profit_bps`
4. Simulate the bundle against block `latest + 1`
5. Submit only when simulation does not revert

---

## Stack

| Concern | Choice |
| ------- | ------ |
| Bot | Go 1.22 · go-ethereum v1.14 |
| Contracts | Solidity 0.8.20 · Hardhat |
| MEV | Flashbots relay (`CallBundle` → `SendBundle`) |
| AMM model | Constant product, 0.3% fee (30 bps) |

---

## Project layout

```
arb-bot/
├── bot/
│   ├── main.go       Polling loop, reserve fetch, bundle build/submit
│   ├── eth.go        RPC client, tx signing (EIP-1559)
│   └── flashbots.go  Flashbots JSON-RPC client
├── contracts/
│   ├── SimpleDEX.sol Toy constant-product AMM (no ERC20)
│   └── FlashArb.sol  Two-leg arb executor with profit tracking
├── config/
│   └── config.json   RPC, keys, contract addresses, loop tuning
└── hardhat.config.js
```

---

## Quick start

### Prerequisites

- Go 1.22+
- Node.js + npm (Hardhat)
- An Ethereum RPC endpoint (local Hardhat node or testnet)

### 1. Deploy contracts (local)

```bash
cd arb-bot
npm install
npx hardhat node          # terminal 1
npx hardhat run scripts/deploy.js --network localhost   # terminal 2
```

Copy deployed `SimpleDEX` (×2) and `FlashArb` addresses into `config/config.json`.

### 2. Configure

Edit [`arb-bot/config/config.json`](arb-bot/config/config.json):

```json
{
  "rpc_url": "http://127.0.0.1:8545",
  "private_key": "0xYOUR_PRIVATE_KEY",
  "flashbots_url": "https://relay.flashbots.net",
  "dex1_address": "0x…",
  "dex2_address": "0x…",
  "arb_contract": "0x…",
  "gas_limit": 350000,
  "loop_seconds": 5,
  "fee_bps": 30,
  "min_profit_bps": 0
}
```

| Field | Description |
| ----- | ----------- |
| `loop_seconds` | Poll interval (default 5) |
| `fee_bps` | Assumed pool fee in basis points (default 30 = 0.3%) |
| `min_profit_bps` | Minimum profit as bps of input amount (0 = any positive profit) |
| `flashbots_url` | Flashbots relay endpoint (mainnet); use local sim for dev |

> **Never commit real private keys.** Use a dedicated hot wallet with minimal funds.

### 3. Run the bot

```bash
cd arb-bot
go run ./bot -config config/config.json
```

The bot logs startup addresses and emits `sent bundle=…` when a profitable bundle is submitted.

---

## Contracts

### `SimpleDEX`

Minimal AMM with internal reserves (no token transfers). Supports `swap(X→Y)` and `swapReverse(Y→X)` with a 0.3% fee.

### `FlashArb`

Executes a two-leg arbitrage: swap on `dex1`, reverse-swap on `dex2`. Reverts with `"no profit"` if the round trip loses money. Tracks cumulative `totalProfit` and emits `Arbitrage` events.

---

## Safety notes

- **Simulation first** — every bundle is simulated before submission
- **Off-chain guard** — profit estimate and `min_profit_bps` filter unprofitable attempts
- **Educational scope** — toy AMM without ERC20 transfers; not production-ready for mainnet DEXes
- **MEV competition** — real arbitrage on mainnet requires latency, capital, and private order flow beyond this demo

---

## License

Private / portfolio use.
