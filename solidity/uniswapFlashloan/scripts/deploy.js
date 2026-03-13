const hre = require("hardhat");
const fs = require("fs");
const path = require("path");

// Network configurations
const NETWORK_CONFIG = {
  mainnet: {
    aavePoolAddressesProvider: "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e",
    uniswapV2Router: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
    uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
  },
  sepolia: {
    aavePoolAddressesProvider: "0x012bAC54348C0E635dCAc9D5FB99f06F24136C9A", // Correct checksummed address
    uniswapV2Router: "0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008",
    uniswapV3Router: "0x3bFA4769FB09eefC5a80d6E87c3B9C650f7Ae48E",
  },
  arbitrum: {
    aavePoolAddressesProvider: "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
    uniswapV2Router: "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24",
    uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
  },
  polygon: {
    aavePoolAddressesProvider: "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
    uniswapV2Router: "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24",
    uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
  },
};

async function main() {
  const network = hre.network.name;
  console.log(`Deploying to ${network}...`);

  const config = NETWORK_CONFIG[network];
  if (!config) {
    throw new Error(`Network ${network} not configured`);
  }

  // Get deployer
  const [deployer] = await hre.ethers.getSigners();
  console.log("Deploying with account:", deployer.address);

  const balance = await hre.ethers.provider.getBalance(deployer.address);
  console.log("Account balance:", hre.ethers.formatEther(balance), "ETH");

  // Deployment parameters
  const useV3Router = process.env.USE_V3_ROUTER === "true" || false;
  const defaultPoolFee = parseInt(process.env.DEFAULT_POOL_FEE || "3000"); // 0.3%
  const maxSlippageBps = parseInt(process.env.MAX_SLIPPAGE_BPS || "50"); // 0.5%

  // Checksum addresses
  const aaveProvider = hre.ethers.getAddress(config.aavePoolAddressesProvider);
  const v2Router = hre.ethers.getAddress(config.uniswapV2Router);
  const v3Router = hre.ethers.getAddress(config.uniswapV3Router);

  console.log("\nDeployment parameters:");
  console.log("  Aave Pool Addresses Provider:", aaveProvider);
  console.log("  Uniswap V2 Router:", v2Router);
  console.log("  Uniswap V3 Router:", v3Router);
  console.log("  Use V3 Router:", useV3Router);
  console.log("  Default Pool Fee:", defaultPoolFee);
  console.log("  Max Slippage (BPS):", maxSlippageBps);

  // Deploy contract
  const UniswapFlashLoanArbitrage = await hre.ethers.getContractFactory(
    "UniswapFlashLoanArbitrage"
  );

  console.log("\nDeploying contract...");
  const contract = await UniswapFlashLoanArbitrage.deploy(
    aaveProvider,
    v2Router,
    v3Router,
    useV3Router,
    defaultPoolFee,
    maxSlippageBps
  );

  await contract.waitForDeployment();
  const contractAddress = await contract.getAddress();

  console.log("\n========================================");
  console.log("DEPLOYMENT SUCCESSFUL!");
  console.log("========================================");
  console.log("Contract address:", contractAddress);
  console.log("Network:", network);
  console.log("Deployer:", deployer.address);
  console.log("========================================\n");

  // Save deployment info
  const deploymentDir = path.join(__dirname, "..", "deployments");
  if (!fs.existsSync(deploymentDir)) {
    fs.mkdirSync(deploymentDir, { recursive: true });
  }

  const deploymentInfo = {
    network,
    contractAddress,
    deployer: deployer.address,
    blockNumber: await hre.ethers.provider.getBlockNumber(),
    timestamp: new Date().toISOString(),
    config: {
        aavePoolAddressesProvider: aaveProvider,
        uniswapV2Router: v2Router,
        uniswapV3Router: v3Router,
      useV3Router,
      defaultPoolFee,
      maxSlippageBps,
    },
  };

  const deploymentFile = path.join(deploymentDir, `${network}.json`);
  fs.writeFileSync(
    deploymentFile,
    JSON.stringify(deploymentInfo, null, 2)
  );

  console.log(`Deployment info saved to: ${deploymentFile}`);

  // Verify on Etherscan (if API key is set)
  if (process.env.ETHERSCAN_API_KEY || process.env.ARBISCAN_API_KEY) {
    console.log("\nWaiting for block confirmations before verification...");
    await contract.deploymentTransaction().wait(5);

    try {
      await hre.run("verify:verify", {
        address: contractAddress,
        constructorArguments: [
          aaveProvider,
          v2Router,
          v3Router,
          useV3Router,
          defaultPoolFee,
          maxSlippageBps,
        ],
      });
      console.log("Contract verified on Etherscan!");
    } catch (error) {
      console.log("Verification failed:", error.message);
    }
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
