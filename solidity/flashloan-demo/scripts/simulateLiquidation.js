const { ethers, network } = require("hardhat");

async function main() {
    const USDC_ADDR = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48";
    const WETH_ADDR = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2";
    const AAVE_POOL_ADDR = "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2";
    const flashLoanAddress = "0xb50C0B2FA70d7C07829e8479245F00576278771B";

    const [deployer] = await ethers.getSigners();
    const victim = deployer.address;

    console.log("--- Executing Final Liquidation Integration ---");

    const pool = await ethers.getContractAt("IPool", AAVE_POOL_ADDR);
    const flashLoanContract = await ethers.getContractAt("FlashLoanExample", flashLoanAddress);

    // 1. Get Aave Reserve Data
    const reserveDataWETH = await pool.getReserveData(WETH_ADDR);
    const reserveDataUSDC = await pool.getReserveData(USDC_ADDR);
    const aWETH_ADDR = reserveDataWETH.aTokenAddress;
    const debtTokenAddr = reserveDataUSDC.variableDebtTokenAddress;

    // 2. Setup Victim "Scaled" Balances
    const mappingSlot = 52;
    const victimSlot = ethers.solidityPackedKeccak256(["uint256", "uint256"], [victim, mappingSlot]);

    // Give victim ~20 ETH collateral (Enough to pay bonus)
    // and ~$100k debt (Insolvent)
    console.log("Injecting Scaled Balances into Victim Account...");
    await network.provider.send("hardhat_setStorageAt", [
        aWETH_ADDR,
        victimSlot,
        ethers.toBeHex(ethers.parseEther("20"), 32)
    ]);
    await network.provider.send("hardhat_setStorageAt", [
        debtTokenAddr,
        victimSlot,
        ethers.toBeHex(ethers.parseUnits("100000", 6), 32)
    ]);

    // 3. Fund your contract for the Flash Loan Premium
    // Instead of just 1,000, let's give it 100,000 USDC to be safe
    const usdcAmount = ethers.toBeHex(ethers.parseUnits("100000", 6), 32);

    // We will try slot 9 (Standard) AND slot 0 (Common for some USDC versions)
    console.log("Funding Flash Loan contract with USDC for fees...");
    const slots = [9, 0];
    for (const slot of slots) {
        const slotHash = ethers.solidityPackedKeccak256(["uint256", "uint256"], [flashLoanAddress, slot]);
        await network.provider.send("hardhat_setStorageAt", [USDC_ADDR, slotHash, usdcAmount]);
    }
    // 4. Verify HF and Liquidate
    const data = await pool.getUserAccountData(victim);
    console.log("Confirmed Health Factor:", ethers.formatUnits(data.healthFactor, 18));

    const debtToCover = ethers.parseUnits("5000", 6);
    console.log(`Triggering Liquidation for ${ethers.formatUnits(debtToCover, 6)} USDC...`);

    const tx = await flashLoanContract.requestLiquidationLoan(USDC_ADDR, debtToCover, victim);
    await tx.wait();

    // 5. Final Results and Withdraw
    const awethToken = await ethers.getContractAt(
        "@aave/core-v3/contracts/dependencies/openzeppelin/contracts/IERC20.sol:IERC20",
        aWETH_ADDR
    );
    const bonus = await awethToken.balanceOf(flashLoanAddress);
    console.log("Bonus aWETH Captured in Contract:", ethers.formatUnits(bonus, 18));

    if (bonus > 0n) {
        console.log("Withdrawing profits to deployer wallet...");
        await flashLoanContract.withdraw(aWETH_ADDR);
        const walletBal = await awethToken.balanceOf(deployer.address);
        console.log("Final Wallet Balance (aWETH):", ethers.formatUnits(walletBal, 18));
    }
    console.log("-----------------------------------------");
}

main().catch(console.error);