const hre = require("hardhat");

/**
 * Deploy Test Tokens for Sepolia
 * 
 * Deploys simple ERC20 tokens for testing Uniswap pools
 */

async function main() {
  console.log("=== Deploying Test Tokens ===\n");

  const [deployer] = await hre.ethers.getSigners();
  console.log("Deployer:", deployer.address);
  const balance = await hre.ethers.provider.getBalance(deployer.address);
  console.log("Balance:", hre.ethers.formatEther(balance), "ETH\n");

  // Deploy Test USDC (6 decimals, like real USDC)
  console.log("Deploying TestUSDC...");
  const TestToken = await hre.ethers.getContractFactory("TestToken");
  const testUSDC = await TestToken.deploy(
    "Test USDC",
    "tUSDC",
    6,
    hre.ethers.parseUnits("1000000", 6) // 1M tokens
  );
  await testUSDC.waitForDeployment();
  const usdcAddress = await testUSDC.getAddress();
  console.log("✓ TestUSDC deployed:", usdcAddress);

  // Deploy Test WETH (18 decimals)
  console.log("\nDeploying TestWETH...");
  const testWETH = await TestToken.deploy(
    "Test WETH",
    "tWETH",
    18,
    hre.ethers.parseEther("1000") // 1000 tokens
  );
  await testWETH.waitForDeployment();
  const wethAddress = await testWETH.getAddress();
  console.log("✓ TestWETH deployed:", wethAddress);

  console.log("\n========================================");
  console.log("TEST TOKENS DEPLOYED!");
  console.log("========================================");
  console.log("TestUSDC:", usdcAddress);
  console.log("TestWETH:", wethAddress);
  console.log("\nNext steps:");
  console.log("1. Update your bot's MONITOR_PAIRS with these addresses");
  console.log("2. Run setupTestPool.js to create a pool");
  console.log("3. Add liquidity to the pool");
  console.log("========================================\n");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
