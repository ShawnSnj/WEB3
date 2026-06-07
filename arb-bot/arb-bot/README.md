# arb-bot (minimal Flashbots arbitrage demo)

This repo contains:

- `SimpleDEX.sol`: constant-product AMM with a 0.3% fee
- `FlashArb.sol`: executes `dex1.swap(amount)` then `dex2.swapReverse(result)` and reverts unless the trade is strictly profitable
- `bot/main.go`: a Go loop that fetches reserves, computes an `OptimalDx`, simulates the bundle with `eth_callBundle`, and only sends via `eth_sendBundle` when the simulation does not revert

## Deploy contracts

1. Install dependencies:
   - `cd arb-bot`
   - `npm install`
2. Compile:
   - `npx hardhat compile`
3. Deploy on Hardhat (default):
   - `npx hardhat run scripts/deploy.js --network hardhat`

The deploy script prints the three deployed addresses. Copy them into:

- `config/config.json` (`dex1_address`, `dex2_address`, `arb_contract`)

## Configure

Edit `config/config.json`:

- `rpc_url`: JSON-RPC endpoint
- `private_key`: the EOA private key used to sign both the bundle transaction and Flashbots request header
- `flashbots_url`: typically `https://relay.flashbots.net`

Optional:
- `gas_limit` (default: `350000`)
- `loop_seconds` (default: `5`)
- `min_profit_bps` (default: `0`) - bot skips bundles unless estimated profit is at least `amountIn * min_profit_bps / 10000`

## Run the bot

From the repo root:

- `cd arb-bot`
- `go build ./...`
- `go run ./bot -config config/config.json`

The bot repeats every `loop_seconds`:

1. Reads `reserveX/reserveY` from both DEXes
2. Computes `dx` via `OptimalDx`
3. Estimates profit off-chain (using the same integer AMM math)
4. Builds a signed call to `FlashArb.executeArbitrage(...)`
5. Simulates it with `eth_callBundle`
6. Sends the bundle with `eth_sendBundle` only if the simulation does not revert

## Notes

This is intentionally minimal/toy:
- There are no ERC20 tokens; reserves are updated internally.
- Flashbots simulation is the final on-chain profitability guard (the contract reverts on `no profit`).

