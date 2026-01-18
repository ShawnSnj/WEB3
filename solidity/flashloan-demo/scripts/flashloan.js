async function main() {
    const FLASHLOAN_ADDRESS = "PASTE_DEPLOYED_ADDRESS";
    const USDC = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48";

    const flashLoan = await ethers.getContractAt(
        "FlashLoanExample",
        FLASHLOAN_ADDRESS
    );

    const amount = ethers.utils.parseUnits("1000000", 6); // 1M USDC

    await flashLoan.requestFlashLoan(USDC, amount);
    console.log("Flash loan executed");
}

main();
