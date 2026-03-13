const { ethers } = require("hardhat");

async function main() {
  const routerAddress = "0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008";
  const token0 = "0x36043011d9e0d40625b8bf37DD0bF9BccC1d75a9"; // TestWETH
  const token1 = "0x7638A54c381bF1207d8c0A1F6351292F94233154"; // TestUSDC

  const router = await ethers.getContractAt(
    [
      "function getAmountsOut(uint amountIn, address[] calldata path) view returns (uint[] memory amounts)"
    ],
    routerAddress
  );

  console.log("Testing getAmountsOut...");
  console.log("Token0 (TestWETH):", token0);
  console.log("Token1 (TestUSDC):", token1);
  console.log("Router:", routerAddress);

  try {
    // Test with 1 token (18 decimals for WETH)
    const amountIn = ethers.parseUnits("1", 18);
    const path = [token0, token1];
    
    console.log("\nCalling getAmountsOut with 1 token0 (18 decimals)...");
    const amounts = await router.getAmountsOut(amountIn, path);
    
    console.log("Success! Amounts returned:");
    console.log("  Amount[0] (input):", ethers.formatUnits(amounts[0], 18));
    console.log("  Amount[1] (output):", ethers.formatUnits(amounts[1], 6)); // USDC has 6 decimals
    
    const price = parseFloat(ethers.formatUnits(amounts[1], 6)) / parseFloat(ethers.formatUnits(amounts[0], 18));
    console.log("\nPrice: 1 token0 =", price, "token1");
    
  } catch (error) {
    console.error("Error calling getAmountsOut:", error.message);
    if (error.reason) {
      console.error("Reason:", error.reason);
    }
  }
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
