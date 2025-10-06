// A Hardhat script to deploy MemeTaxToken.sol

const { ethers } = require("hardhat"); // Using require for CommonJS module compatibility

async function main() {
  // ----------------------------------------------------
  // 1. DEFINE CONSTRUCTOR ARGUMENTS
  // ----------------------------------------------------
  
  // Example 1: 100,000,000 (100 Million) tokens with 18 decimals
  const initialSupply = ethers.parseUnits("100000000", 18); 

  // Example 2: Sepolia Uniswap V2 Router address (This may need confirmation)
  // NOTE: A common one for Sepolia is a SushiSwap deployment or a V2 clone.
  // Using a placeholder address here. **YOU MUST REPLACE THIS.**
  const routerAddress = "0xeE567Fe1712Faf6149d80dA1E6934E354124CfE3"; 
  
  // Example 3: Your designated wallet for marketing funds. **YOU MUST REPLACE THIS.**
  // The address used for deployment is often the marketing wallet.
  const marketingWalletAddress = "0x1a41a486130B3f75ed350e9873177B1A75Ac9c33"; 

  // ----------------------------------------------------
  // 2. DEPLOYMENT
  // ----------------------------------------------------

  const MemeTaxToken = await ethers.getContractFactory("MemeTaxToken");
  
  // *** THE FIX IS HERE: Pass the three arguments to deploy() ***
  const memeToken = await MemeTaxToken.deploy(
    initialSupply, 
    routerAddress, 
    marketingWalletAddress
  );

  // Wait for the deployment transaction to be mined
  await memeToken.waitForDeployment(); 

  const deployedAddress = await memeToken.getAddress();
  console.log(`MemeTaxToken deployed to: ${deployedAddress}`);
  
  // Optional: Verify the contract immediately after deployment
  // NOTE: This assumes you have the ETHERSCAN_API_KEY set up
  console.log("Waiting 30 seconds before verification...");
  await new Promise(resolve => setTimeout(resolve, 30000));
  
  try {
    await hre.run("verify:verify", {
      address: deployedAddress,
      constructorArguments: [
        initialSupply,
        routerAddress,
        marketingWalletAddress,
      ],
    });
    console.log("Contract verified successfully!");
  } catch (e) {
    console.log("Verification failed:", e.message);
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});