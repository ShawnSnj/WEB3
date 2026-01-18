const { ethers, network } = require("hardhat");

async function main() {
    // 1. CONFIGURATION - Update with your latest deployment address!
    const flashLoanAddress = "0x0be43FA2FB20662D0a086c7596679dd8CCac65f9";
    const USDC_ADDRESS = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48";

    console.log("--- Initializing Flash Loan Business Test ---");

    // 2. GOD MODE: MANUALLY SET CONTRACT BALANCE
    // We calculate the storage slot for your contract's balance in the USDC contract.
    // For USDC on Mainnet, the balance mapping is at Slot 9.
    const mappingSlot = 9;

    // Calculate the specific storage location: keccak256(address + slot)
    const storageSlot = ethers.solidityPackedKeccak256(
        ["uint256", "uint256"],
        [flashLoanAddress, mappingSlot]
    );

    // Write 1,000,000 USDC directly into the blockchain state for your contract
    const amountToSet = ethers.parseUnits("1000000", 6);
    await network.provider.send("hardhat_setStorageAt", [
        USDC_ADDRESS,
        storageSlot,
        ethers.toBeHex(amountToSet, 32),
    ]);

    console.log("God Mode: Contract forced to have 1,000,000 USDC.");

    // 3. EXECUTE THE LOAN
    const [deployer] = await ethers.getSigners();
    const flashLoanContract = await ethers.getContractAt("FlashLoanExample", flashLoanAddress);
    const usdc = await ethers.getContractAt("IERC20", USDC_ADDRESS);

    const borrowAmount = ethers.parseUnits("10000", 6);
    console.log(`Executing Flash Loan: Borrowing ${ethers.formatUnits(borrowAmount, 6)} USDC...`);

    // We use the deployer to trigger the loan
    const tx = await flashLoanContract.connect(deployer).requestFlashLoan(USDC_ADDRESS, borrowAmount);
    await tx.wait();

    // 4. CHECK RESULTS
    const finalBalance = await usdc.balanceOf(flashLoanAddress);
    console.log("-----------------------------------------");
    console.log("Final Contract Balance:", ethers.formatUnits(finalBalance, 6), "USDC");
    console.log("--- Test Complete ---");
}

main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
});