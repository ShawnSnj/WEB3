// scripts/activate.js

const { ethers } = require("hardhat");

// --- 1. REPLACE THESE WITH YOUR EXACT DEPLOYMENT VALUES ---
const MEME_TOKEN_ADDRESS = "0x95D9D5e1f7Ad87e10d241b966b924eB93F2b4b31"; 
const ROUTER_ADDRESS = "0xeE567Fe1712Faf6149d80dA1E6934E354124CfE3"; // Use your confirmed Sepolia Router address!
const INITIAL_LIQUIDITY_TOKENS = ethers.parseUnits("10000000", 18); // Example: 10 Million Tokens for LP
const INITIAL_LIQUIDITY_ETH = ethers.parseEther("1"); // Example: 1 ETH for LP
// ----------------------------------------------------------------

async function main() {
    const [owner] = await ethers.getSigners();
    
    console.log(`\nActivating MemeTaxToken from Owner: ${owner.address}`);

    // Get contract instances
    const memeToken = await ethers.getContractAt("MemeTaxToken", MEME_TOKEN_ADDRESS);
    const router = await ethers.getContractAt("IUniswapV2Router02", ROUTER_ADDRESS);

    // --- STEP 1: INITIALIZE THE TOKEN PAIR (setSwapAndLiquifyEnabled) ---
    console.log("\n1. Initializing Token Pair...");
    let tx = await memeToken.setSwapAndLiquifyEnabled(true);
    await tx.wait();
    
    const pairAddress = await memeToken.uniswapV2Pair();
    console.log(`   ✅ Pair Created! (Address: ${pairAddress})`);
    
    // --- STEP 2: APPROVE THE ROUTER TO SPEND TOKENS ---
    console.log("\n2. Approving Router to spend tokens...");
    
    // Approve the router to spend the exact amount of tokens we plan to add to LP
    tx = await memeToken.approve(ROUTER_ADDRESS, INITIAL_LIQUIDITY_TOKENS);
    await tx.wait();
    
    console.log(`   ✅ Approval granted for ${ethers.formatUnits(INITIAL_LIQUIDITY_TOKENS, 18)} tokens.`);

    // --- STEP 3: ADD INITIAL LIQUIDITY ---
    console.log("\n3. Adding Initial Liquidity (MTT/ETH)...");
    
    // The ETH is passed in the value field, and the tokens are spent via the approval in Step 2.
    tx = await router.addLiquidityETH(
        MEME_TOKEN_ADDRESS,
        INITIAL_LIQUIDITY_TOKENS, // amountTokenDesired
        0, // amountTokenMin
        0, // amountETHMin
        owner.address, // LP tokens go to the owner
        Math.floor(Date.now() / 1000) + (60 * 10), // Deadline 10 minutes from now
        { value: INITIAL_LIQUIDITY_ETH } // The ETH sent with the transaction
    );
    
    const receipt = await tx.wait();
    
    // Check for success event or simply confirm transaction receipt
    if (receipt.status === 1) {
        console.log(`   ✅ Initial Liquidity Added! (TX Hash: ${receipt.hash})`);
        console.log(`\n🎉 Token is now live and tradable on the Uniswap V2 Pair!`);
    } else {
        console.error("   ❌ Failed to add liquidity. Check transaction details.");
    }
}

main().catch((error) => {
    console.error("Activation Failed:", error);
    process.exitCode = 1;
});