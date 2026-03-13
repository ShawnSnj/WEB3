const hre = require("hardhat");
const { ethers } = require("hardhat");

/**
 * Setup Test Pool on Sepolia
 * 
 * This script helps you:
 * 1. Deploy test ERC20 tokens (if needed)
 * 2. Create a Uniswap V2 pool
 * 3. Add initial liquidity
 * 
 * Note: On Sepolia, you may need to use existing test tokens
 * or deploy your own test tokens first.
 */

// Sepolia addresses
const SEPOLIA_CONFIG = {
  uniswapV2Factory: "0x7E0987E5b3a30e3f2828572Bb659A548460a3003", // Sepolia Uniswap V2 Factory
  uniswapV2Router: "0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008",
  WETH: "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", // Sepolia WETH
};

// Simple ERC20 for testing
const ERC20_ABI = [
  "function name() view returns (string)",
  "function symbol() view returns (string)",
  "function decimals() view returns (uint8)",
  "function totalSupply() view returns (uint256)",
  "function balanceOf(address) view returns (uint256)",
  "function transfer(address to, uint256 amount) returns (bool)",
  "function approve(address spender, uint256 amount) returns (bool)",
  "function allowance(address owner, address spender) view returns (uint256)",
  "function mint(address to, uint256 amount) returns (bool)",
];

// Uniswap V2 Factory ABI
const FACTORY_ABI = [
  "function getPair(address tokenA, address tokenB) view returns (address pair)",
  "function createPair(address tokenA, address tokenB) returns (address pair)",
];

// Uniswap V2 Router ABI
const ROUTER_ABI = [
  "function addLiquidity(address tokenA, address tokenB, uint amountADesired, uint amountBDesired, uint amountAMin, uint amountBMin, address to, uint deadline) returns (uint amountA, uint amountB, uint liquidity)",
  "function addLiquidityETH(address token, uint amountTokenDesired, uint amountTokenMin, uint amountETHMin, address to, uint deadline) payable returns (uint amountToken, uint amountETH, uint liquidity)",
];

async function main() {
  console.log("=== Setting Up Test Pool on Sepolia ===\n");

  const [deployer] = await hre.ethers.getSigners();
  console.log("Deployer:", deployer.address);
  const balance = await ethers.provider.getBalance(deployer.address);
  console.log("Balance:", ethers.formatEther(balance), "ETH\n");

  // Get token addresses from environment or use defaults
  const token0Address = process.env.TOKEN0_ADDRESS || SEPOLIA_CONFIG.WETH;
  const token1Address = process.env.TOKEN1_ADDRESS || "0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8"; // Sepolia USDC

  console.log("Token 0 (WETH):", token0Address);
  console.log("Token 1 (USDC):", token1Address);

  // Connect to tokens
  const token0 = new ethers.Contract(token0Address, ERC20_ABI, deployer);
  const token1 = new ethers.Contract(token1Address, ERC20_ABI, deployer);

  try {
    const name0 = await token0.name();
    const symbol0 = await token0.symbol();
    console.log(`Token 0: ${name0} (${symbol0})`);
  } catch (e) {
    console.log("⚠️  Token 0 may not be a standard ERC20 or doesn't exist");
  }

  try {
    const name1 = await token1.name();
    const symbol1 = await token1.symbol();
    console.log(`Token 1: ${name1} (${symbol1})`);
  } catch (e) {
    console.log("⚠️  Token 1 may not be a standard ERC20 or doesn't exist");
  }

  // Check balances and decimals first
  const decimals0 = await token0.decimals();
  const decimals1 = await token1.decimals();
  const balance0 = await token0.balanceOf(deployer.address);
  const balance1 = await token1.balanceOf(deployer.address);

  // Check if pool exists
  const factory = new ethers.Contract(SEPOLIA_CONFIG.uniswapV2Factory, FACTORY_ABI, deployer);
  const existingPair = await factory.getPair(token0Address, token1Address);
  
  let pairAddress = existingPair;
  
  if (existingPair !== ethers.ZeroAddress) {
    console.log("\n✓ Pool already exists:", existingPair);
    
    // Check if pool has liquidity
    const PAIR_ABI = ["function getReserves() view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast)"];
    const pair = new ethers.Contract(existingPair, PAIR_ABI, deployer);
    try {
      const reserves = await pair.getReserves();
      if (reserves[0] > 0n && reserves[1] > 0n) {
        console.log("✓ Pool has liquidity!");
        console.log(`  Reserve 0: ${ethers.formatUnits(reserves[0], decimals0)}`);
        console.log(`  Reserve 1: ${ethers.formatUnits(reserves[1], decimals1)}`);
        console.log("\nYou can use this pool for testing!");
        return;
      } else {
        console.log("⚠️  Pool exists but has no liquidity. Adding liquidity...");
      }
    } catch (e) {
      console.log("⚠️  Could not check reserves. Attempting to add liquidity...");
    }
  } else {
    console.log("\n⚠️  Pool does not exist. Creating pool...");

    console.log("\nToken info:");
    console.log(`  Token 0 decimals: ${decimals0}`);
    console.log(`  Token 1 decimals: ${decimals1}`);
    console.log(`  Token 0 balance: ${ethers.formatUnits(balance0, decimals0)}`);
    console.log(`  Token 1 balance: ${ethers.formatUnits(balance1, decimals1)}`);

    if (balance0 === 0n || balance1 === 0n) {
      console.log("\n❌ Insufficient token balance to create pool");
      console.log("You need both tokens to create a pool.");
      console.log("\nOptions:");
      console.log("1. Get test tokens from faucets");
      console.log("2. Deploy your own test tokens");
      console.log("3. Use existing pools that have liquidity");
      return;
    }

    // Create pair
    console.log("\nCreating pair...");
    const tx = await factory.createPair(token0Address, token1Address);
    await tx.wait();
    pairAddress = await factory.getPair(token0Address, token1Address);
    console.log("✓ Pair created:", pairAddress);
  }

  // Now add liquidity (whether pool was just created or already existed)
  console.log("\nToken info:");
  console.log(`  Token 0 decimals: ${decimals0}`);
  console.log(`  Token 1 decimals: ${decimals1}`);
  console.log(`  Token 0 balance: ${ethers.formatUnits(balance0, decimals0)}`);
  console.log(`  Token 1 balance: ${ethers.formatUnits(balance1, decimals1)}`);

  if (balance0 === 0n || balance1 === 0n) {
    console.log("\n❌ Insufficient token balance to add liquidity");
    return;
  }

  // Add liquidity
  console.log("\nAdding liquidity...");
  const router = new ethers.Contract(SEPOLIA_CONFIG.uniswapV2Router, ROUTER_ABI, deployer);

  // Calculate amounts - use a simple 1:1 ratio in "whole tokens"
  // For test tokens, we'll use equal "units" regardless of decimals
  let amount0 = balance0 / 2n; // Use half of balance
  let amount1 = balance1 / 2n; // Use half of balance
  
  // Adjust to maintain reasonable ratio if decimals differ
  // If token0 has more decimals, we need more of token1 to balance
  if (decimals0 > decimals1) {
    const ratio = 10n ** BigInt(decimals0 - decimals1);
    amount1 = amount0 / ratio;
    if (amount1 > balance1) {
      amount1 = balance1;
      amount0 = amount1 * ratio;
      if (amount0 > balance0) {
        amount0 = balance0;
      }
    }
  } else if (decimals1 > decimals0) {
    const ratio = 10n ** BigInt(decimals1 - decimals0);
    amount0 = amount1 / ratio;
    if (amount0 > balance0) {
      amount0 = balance0;
      amount1 = amount0 * ratio;
      if (amount1 > balance1) {
        amount1 = balance1;
      }
    }
  }
  
  // Ensure we have minimum amounts
  if (amount0 < 1000n || amount1 < 1000n) {
    console.log("\n❌ Amounts too small for liquidity");
    console.log("Need at least some minimum liquidity");
    return;
  }

  console.log(`Approving ${ethers.formatUnits(amount0, decimals0)} token0...`);
  const approve0 = await token0.approve(SEPOLIA_CONFIG.uniswapV2Router, amount0);
  await approve0.wait();
  console.log(`Approving ${ethers.formatUnits(amount1, decimals1)} token1...`);
  const approve1 = await token1.approve(SEPOLIA_CONFIG.uniswapV2Router, amount1);
  await approve1.wait();

  // Add liquidity
  const deadline = Math.floor(Date.now() / 1000) + 60 * 20; // 20 minutes
  const addLiqTx = await router.addLiquidity(
    token0Address,
    token1Address,
    amount0,
    amount1,
    amount0 * 95n / 100n, // 5% slippage
    amount1 * 95n / 100n,
    deployer.address,
    deadline
  );

  await addLiqTx.wait();
  console.log("✓ Liquidity added!");

  console.log("\n========================================");
  console.log("POOL SETUP COMPLETE!");
  console.log("========================================");
  console.log("Pair address:", pairAddress);
  console.log("You can now use this pool in your bot!");
  console.log("========================================\n");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
