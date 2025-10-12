const { ethers } = require("hardhat");

async function main() {
  // ✅ Step 1: Deploy the ERC-20 MetaNode token (if not yet deployed)
  const Token = await ethers.getContractFactory("MetaNodeToken");
  const initialSupply = ethers.parseUnits("100000000", 18); // 100 million tokens
  const token = await Token.deploy(initialSupply);
  await token.waitForDeployment();

  const metaNodeTokenAddress = await token.getAddress();
  console.log(`MetaNodeToken deployed to: ${metaNodeTokenAddress}`);

  // ✅ Step 2: Deploy MetaNodeStaking with the token address
  const Staking = await ethers.getContractFactory("MetaNodeStaking");
  const staking = await Staking.deploy(metaNodeTokenAddress);
  await staking.waitForDeployment();

  const stakingAddress = await staking.getAddress();
  console.log(`MetaNodeStaking deployed to: ${stakingAddress}`);
}


main().catch((error) => {
  console.error("❌ Deployment failed:", error);
  process.exitCode = 1;
});