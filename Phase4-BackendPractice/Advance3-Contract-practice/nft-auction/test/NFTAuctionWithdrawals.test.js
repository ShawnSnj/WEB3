require("dotenv").config();
const { expect } = require("chai");
const { ethers } = require("hardhat");

// We don't need Hardhat network helpers for this cleanup script on Sepolia,
// but we keep the import line commented out for reference.
// const { time } = require("@nomicfoundation/hardhat-network-helpers");

describe("NFTAuction Global Withdrawal and Cleanup on Sepolia", function () {

    let owner, bidder1, bidder2;
    let nftAuction;

    // --- Contract Information ---
    const NFTAUCTION_ADDRESS = "0x627bEd9E638C4158da5d79cA503006361F7c2b66";
    // Define a range of recent auction IDs to attempt cleanup on
    const AUCTION_IDS_TO_CLEAN = [1, 2, 3, 4, 5];

    before(async function () {
        // 🟢 1. Initialize Wallets and Contracts
        const provider = ethers.provider;

        owner = new ethers.Wallet(process.env.PRIVATE_KEY, provider);
        bidder1 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER1, provider);
        bidder2 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER2, provider);

        // Connect the 'owner' wallet to the deployed contract
        nftAuction = await ethers.getContractAt("NFTAuction", NFTAUCTION_ADDRESS, owner);

        console.log(`\n--- Starting Cleanup Routine for Contract: ${NFTAUCTION_ADDRESS} ---`);
    });

    // We combine the withdrawal logic into a single test to perform the cleanup loop.
    it("should attempt to end and withdraw all available balances across multiple recent auctions", async function () {

        const participants = [
            { signer: owner, role: "Owner (Seller)" },
            { signer: bidder1, role: "Bidder 1" },
            { signer: bidder2, role: "Bidder 2 (Winner)" }
        ];

        let totalWithdrawals = 0n;

        for (const auctionId of AUCTION_IDS_TO_CLEAN) {
            console.log(`\n[Auction ID ${auctionId}] - Starting cleanup...`);

            // 1. Attempt to end the auction (Non-critical, allows test to proceed if it reverts)
            try {
                const auctionDetails = await nftAuction.auctions(auctionId);
                // Only try to end if the auction exists (seller is not zero address) and it's not settled
                if (auctionDetails.seller !== ethers.ZeroAddress && !auctionDetails.settled) {
                    // Attempt to end auction using the original seller (owner)
                    console.log(`[Auction ID ${auctionId}] Attempting to end auction...`);
                    const endTx = await nftAuction.connect(owner).endAuction(auctionId);
                    await endTx.wait();
                    console.log(`[Auction ID ${auctionId}] Successfully settled.`);
                }
            } catch (e) {
                // This catches the "Auction has already ended" error or other settlement errors
                console.log(`[Auction ID ${auctionId}] Settlement skipped/failed: ${e.message.split('\n')[0].trim()}`);
            }

            // 2. Attempt withdrawals for all participants
            for (const { signer, role } of participants) {
                const availableFunds = await nftAuction.fundsToWithdraw(auctionId, signer.address);

                if (availableFunds > 0n) {
                    console.log(`[Auction ID ${auctionId}] ${role} has ${ethers.formatEther(availableFunds)} ETH available.`);

                    try {
                        // Record pre-withdrawal balance for verification
                        const initialEthBalance = await ethers.provider.getBalance(signer.address);

                        const withdrawTx = await nftAuction.connect(signer).withdraw(auctionId);
                        const receipt = await withdrawTx.wait();

                        // Verification (optional but good practice)
                        const finalEthBalance = await ethers.provider.getBalance(signer.address);
                        const gasUsed = receipt.gasUsed * receipt.gasPrice;
                        const actualReceived = finalEthBalance - initialEthBalance + gasUsed;

                        // Ensure the received amount is very close to the available funds
                        const acceptableDifference = ethers.parseEther("0.0001");
                        expect(actualReceived).to.be.closeTo(availableFunds, acceptableDifference, `${role} did not withdraw the correct amount.`);

                        totalWithdrawals += availableFunds;
                        console.log(`[Auction ID ${auctionId}] ${role} successfully withdrew.`);

                        // Verify balance is now zero in the contract
                        const postWithdrawBalance = await nftAuction.fundsToWithdraw(auctionId, signer.address);
                        expect(postWithdrawBalance).to.equal(0n, `${role}'s internal balance should be zero.`);

                    } catch (e) {
                        console.error(`[Auction ID ${auctionId}] ${role} withdrawal FAILED: ${e.message.split('\n')[0].trim()}`);
                    }
                } else {
                    // console.log(`[Auction ID ${auctionId}] ${role} has 0.0 ETH available. Skipping withdrawal.`);
                }
            }
        }

        console.log(`\n--- Cleanup Summary ---`);
        console.log(`Total funds successfully processed: ${ethers.formatEther(totalWithdrawals)} ETH`);

        // 3. Final Assertion: Check if the entire contract balance is now zero (or near-zero dust)
        const finalContractBalance = await ethers.provider.getBalance(NFTAUCTION_ADDRESS);
        const acceptableDust = ethers.parseEther("0.000001");

        expect(finalContractBalance).to.be.at.most(acceptableDust, "The final contract balance must be near zero after comprehensive withdrawal.");
        console.log(`Final NFTAuction Contract Balance: ${ethers.formatEther(finalContractBalance)} ETH.`);
    });
});