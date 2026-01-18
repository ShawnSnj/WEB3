const { ethers, network } = require("hardhat");

/**
 * Complete Flash Loan Liquidation Demo Script
 * 
 * This script demonstrates a full flash loan liquidation:
 * 1. Sets up a victim position with unhealthy health factor
 * 2. Funds the flash loan contract with tokens for premium
 * 3. Executes flash loan liquidation
 * 4. Withdraws profits
 */

async function main() {
    console.log("=== Flash Loan Liquidation Demo ===\n");

    // Configuration - Ethereum Mainnet addresses
    const USDC_ADDR = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48";
    const WETH_ADDR = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2";
    const AAVE_POOL_ADDR = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2";
    const POOL_ADDRESSES_PROVIDER = "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e";

    // Deploy or use existing contract
    const [deployer] = await ethers.getSigners();
    console.log("Deployer:", deployer.address);
    console.log("Balance:", ethers.formatEther(await ethers.provider.getBalance(deployer.address)), "ETH\n");

    // Deploy FlashLoan contract
    console.log("1. Deploying FlashLoanExample contract...");
    const FlashLoan = await ethers.getContractFactory("FlashLoanExample");
    const flashLoanContract = await FlashLoan.deploy(POOL_ADDRESSES_PROVIDER);
    await flashLoanContract.waitForDeployment();
    const flashLoanAddress = await flashLoanContract.getAddress();
    console.log("✓ FlashLoan deployed at:", flashLoanAddress, "\n");

    // Get Aave Pool instance
    // Try to use IPool ABI from @aave package, fallback to manual decode
    let aWETH_ADDR, debtTokenAddr;
    let pool; // Pool instance for later use

    console.log("2. Fetching Aave reserve data...");

    try {
        // Try using the IPool interface from artifacts
        const IPoolArtifact = require("@aave/core-v3/artifacts/contracts/interfaces/IPool.sol/IPool.json");
        pool = await ethers.getContractAt(IPoolArtifact.abi, AAVE_POOL_ADDR);

        const reserveDataWETH = await pool.getReserveData(WETH_ADDR);
        const reserveDataUSDC = await pool.getReserveData(USDC_ADDR);
        aWETH_ADDR = reserveDataWETH.aTokenAddress;
        debtTokenAddr = reserveDataUSDC.variableDebtTokenAddress;
        console.log("✓ Successfully fetched reserve data using IPool ABI");
    } catch (error) {
        console.log("   ⚠️  Could not use IPool ABI, using low-level calls...");

        // Create minimal pool interface for getUserAccountData later
        const poolABI = [
            "function getUserAccountData(address user) view returns (uint256 totalCollateralBase, uint256 totalDebtBase, uint256 availableBorrowsBase, uint256 currentLiquidationThreshold, uint256 ltv, uint256 healthFactor)"
        ];
        pool = await ethers.getContractAt(poolABI, AAVE_POOL_ADDR);

        // Ethereum Mainnet addresses (from Aave docs) - fallback
        aWETH_ADDR = "0x4d5F47FA6A74757f35C14fD3a6Ef8E3C9BC514E8"; // aWETH
        debtTokenAddr = "0x72E95b8931767C79bA4EeE721354d6E99a61D004"; // vDebtUSDC

        // For Hardhat fork, we can query directly using low-level call
        try {
            const poolInterface = new ethers.Interface([
                "function getReserveData(address) view returns (address,address,address)"
            ]);

            // Get WETH aToken
            const dataWETH = poolInterface.encodeFunctionData("getReserveData", [WETH_ADDR]);
            const resultWETH = await ethers.provider.call({ to: AAVE_POOL_ADDR, data: dataWETH });
            const decodedWETH = poolInterface.decodeFunctionResult("getReserveData", resultWETH);
            aWETH_ADDR = decodedWETH[0]; // First return value is aTokenAddress

            // Get USDC debt token
            const dataUSDC = poolInterface.encodeFunctionData("getReserveData", [USDC_ADDR]);
            const resultUSDC = await ethers.provider.call({ to: AAVE_POOL_ADDR, data: dataUSDC });
            const decodedUSDC = poolInterface.decodeFunctionResult("getReserveData", resultUSDC);
            debtTokenAddr = decodedUSDC[2]; // Third return value is variableDebtTokenAddress

            console.log("✓ Successfully fetched addresses via low-level call");
        } catch (lowLevelError) {
            console.log("   Using hardcoded addresses (safe for Ethereum Mainnet testing)");
        }
    }

    console.log("✓ aWETH Address:", aWETH_ADDR);
    console.log("✓ USDC Debt Token:", debtTokenAddr, "\n");

    // Setup victim position (only for local hardhat fork)
    if (network.name === "hardhat") {
        console.log("3. Setting up victim position (Hardhat fork only)...");
        const victim = deployer.address; // Using deployer as victim for demo

        // Set scaled balances (slot 52 for aTokens and debt tokens)
        const mappingSlot = 52;
        const victimSlot = ethers.solidityPackedKeccak256(
            ["uint256", "uint256"],
            [ethers.zeroPadValue(victim, 32), mappingSlot]
        );

        // Give victim collateral: ~20 WETH
        const collateralAmount = ethers.parseEther("20");
        await network.provider.send("hardhat_setStorageAt", [
            aWETH_ADDR,
            ethers.toBeHex(victimSlot, 32),
            ethers.toBeHex(collateralAmount, 32)
        ]);

        // Give victim debt: ~100,000 USDC (insolvent position)
        const debtAmount = ethers.parseUnits("100000", 6);
        await network.provider.send("hardhat_setStorageAt", [
            debtTokenAddr,
            ethers.toBeHex(victimSlot, 32),
            ethers.toBeHex(debtAmount, 32)
        ]);
        console.log("✓ Victim position configured\n");
    }

    // Fund flash loan contract with USDC for premium
    console.log("4. Funding contract with USDC for flash loan premium...");
    if (network.name === "hardhat") {
        // For hardhat fork, we can set storage directly
        const usdcSlot = 9; // Standard ERC20 balance slot
        const contractSlot = ethers.solidityPackedKeccak256(
            ["uint256", "uint256"],
            [ethers.zeroPadValue(flashLoanAddress, 32), usdcSlot]
        );

        // Fund with 10,000 USDC (more than enough for premium)
        const fundingAmount = ethers.parseUnits("10000", 6);
        await network.provider.send("hardhat_setStorageAt", [
            USDC_ADDR,
            ethers.toBeHex(contractSlot, 32),
            ethers.toBeHex(fundingAmount, 32)
        ]);
        console.log("✓ Contract funded with", ethers.formatUnits(fundingAmount, 6), "USDC\n");
    } else {
        // For real network, you need to actually send USDC
        console.log("⚠️  On real network, you must send USDC to the contract to cover premium");
        console.log("   Contract address:", flashLoanAddress);
        console.log("   Recommended: Send at least 1,000 USDC\n");
    }

    // Check victim health factor
    const victim = deployer.address;
    console.log("5. Checking victim health factor...");
    try {
        const userData = await pool.getUserAccountData(victim);
        const healthFactor = userData.healthFactor;
        console.log("   Health Factor:", ethers.formatUnits(healthFactor, 27));

        if (healthFactor >= ethers.parseUnits("1", 27)) {
            console.log("   ⚠️  Health factor is above 1.0 - position is not liquidatable");
            console.log("   This is normal if victim position wasn't properly set up\n");
        } else {
            console.log("   ✓ Position is liquidatable!\n");
        }
    } catch (error) {
        console.log("   ⚠️  Could not fetch user data:", error.message, "\n");
    }

    // Execute flash loan liquidation
    const debtToCover = ethers.parseUnits("5000", 6); // 5,000 USDC
    console.log("6. Executing flash loan liquidation...");
    console.log("   Debt to cover:", ethers.formatUnits(debtToCover, 6), "USDC");
    console.log("   Victim:", victim);

    try {
        const tx = await flashLoanContract.requestLiquidationLoanWithWETH(
            USDC_ADDR,
            debtToCover,
            victim
        );
        console.log("   Transaction sent:", tx.hash);
        const receipt = await tx.wait();
        console.log("   ✓ Transaction confirmed in block", receipt.blockNumber);
        console.log("   Gas used:", receipt.gasUsed.toString(), "\n");
    } catch (error) {
        console.log("   ✗ Transaction failed:", error.message);

        if (error.message.includes("insufficient balance")) {
            console.log("\n   💡 Tip: Make sure the contract has enough USDC to repay the flash loan premium");
            console.log("   The premium is typically 0.05% - 0.09% of the loan amount\n");
        }
        return;
    }

    // Check balances after liquidation
    console.log("7. Checking contract balances...");
    const usdc = await ethers.getContractAt("IERC20", USDC_ADDR);
    const aWETH = await ethers.getContractAt("IERC20", aWETH_ADDR);

    const usdcBalance = await usdc.balanceOf(flashLoanAddress);
    const aWETHBalance = await aWETH.balanceOf(flashLoanAddress);

    console.log("   USDC Balance:", ethers.formatUnits(usdcBalance, 6), "USDC");
    console.log("   aWETH Balance:", ethers.formatEther(aWETHBalance), "aWETH");

    if (aWETHBalance > 0) {
        console.log("   ✓ Profit captured as aWETH!\n");

        // Withdraw profits
        console.log("8. Withdrawing aWETH profits...");
        try {
            const withdrawTx = await flashLoanContract.withdrawAToken(aWETH_ADDR);
            await withdrawTx.wait();
            console.log("   ✓ Profits withdrawn to deployer wallet\n");
        } catch (error) {
            console.log("   ⚠️  Withdrawal failed:", error.message, "\n");
        }
    } else {
        console.log("   ⚠️  No aWETH received - liquidation may not have been profitable or position wasn't liquidatable\n");
    }

    console.log("=== Demo Complete ===");
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });