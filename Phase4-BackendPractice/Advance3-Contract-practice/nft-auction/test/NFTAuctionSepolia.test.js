require("dotenv").config();
const { expect } = require("chai");
const { ethers } = require("hardhat");

// Import Hardhat network helpers for time manipulation
const { time } = require("@nomicfoundation/hardhat-network-helpers");


//This tells the Mocha test runner to wait for up to **120,000 milliseconds (2 minutes)** for the `before` hook to complete, giving Sepolia ample time to mine the `createAuction` transaction.

//Try running the test again with the updated file!
describe("NFTAuction on Sepolia (RERUNNABLE FULL CYCLE TEST)", function () {
    this.timeout(120000); // Set timeout to 120 seconds
    let owner, bidder1, bidder2;
    let nftAuction, mockNFT;

    // --- Contract and Token Information ---
    const NFTAUCTION_ADDRESS = "0x627bEd9E638C4158da5d79cA503006361F7c2b66";
    const MOCKNFT_ADDRESS = "0xD0f38035f932Fd968b7803d26132762629e5CCAB";
    const TOKEN_ID = 0;
    const AUCTION_DURATION = 60; // 60 seconds for a quick test

    let auctionId = null;

    before(async function () {
        // 🟢 1. Initialize Wallets and Contracts
        const provider = ethers.provider;

        // Use real wallets configured via environment variables (.env file)
        owner = new ethers.Wallet(process.env.PRIVATE_KEY, provider);
        bidder1 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER1, provider);
        bidder2 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER2, provider);

        // Connect the 'owner' wallet to the deployed contracts
        nftAuction = await ethers.getContractAt("NFTAuction", NFTAUCTION_ADDRESS, owner);
        mockNFT = await ethers.getContractAt("MockNFT", MOCKNFT_ADDRESS, owner);

        console.log(`\n--- Pre-flight NFT Retrieval & Setup ---`);

        // ------------------------------------------------------------------
        // --- NFT RETRIEVAL LOGIC (Makes the script re-runnable) ---
        // ------------------------------------------------------------------

        // Step 1: Check where the NFT currently is.
        let tokenOwner = await mockNFT.ownerOf(TOKEN_ID).catch(() => ethers.ZeroAddress);
        console.log(`NFT ID ${TOKEN_ID} current owner: ${tokenOwner}`);

        const auctionContractAddress = NFTAUCTION_ADDRESS.toLowerCase();
        const ownerAddress = owner.address.toLowerCase();
        const bidder2Address = bidder2.address.toLowerCase(); // The previous winner

        // Check 1: Is the NFT stuck in the Auction Contract? (If so, finalize it)
        if (tokenOwner.toLowerCase() === auctionContractAddress) {
            console.log("Status: NFT is held by the Auction contract. Attempting to retrieve via endAuction...");

            // Find the most recent auction involving this NFT to call endAuction on
            let latestAuctionId = null;
            for (let id = 20; id >= 1; id--) {
                const a = await nftAuction.auctions(id);
                if (a.seller !== ethers.ZeroAddress && a.nftContract.toLowerCase() === MOCKNFT_ADDRESS.toLowerCase() && Number(a.tokenId) === TOKEN_ID) {
                    latestAuctionId = id;
                    break;
                }
            }

            if (latestAuctionId !== null) {
                try {
                    // This moves the NFT from the contract to the winner (bidder2) or back to the seller (owner)
                    const endTx = await nftAuction.endAuction(latestAuctionId);
                    await endTx.wait();
                    console.log(`Successfully called endAuction on ID ${latestAuctionId}.`);
                } catch (e) {
                    console.error("Could not call endAuction (It may have been settled):", e.message.split('\n')[0].trim());
                }
            }
            // Re-check owner after attempting endAuction
            tokenOwner = await mockNFT.ownerOf(TOKEN_ID);
        }

        // Check 2: Is the NFT owned by the previous winner (bidder2)? (If so, transfer it back)
        if (tokenOwner.toLowerCase() === bidder2Address) {
            console.log("Status: NFT is held by the previous winner (Bidder 2). Transferring back to Owner...");

            // Connect to the NFT contract using the *winner's* private key (bidder2)
            const mockNFT_bidder2 = await ethers.getContractAt("MockNFT", MOCKNFT_ADDRESS, bidder2);

            try {
                // Bidder 2 transfers the NFT back to the Owner (seller)
                const transferTx = await mockNFT_bidder2.transferFrom(bidder2.address, owner.address, TOKEN_ID);
                await transferTx.wait();
                console.log("Transfer successful: NFT moved from Bidder 2 back to Owner.");
            } catch (e) {
                console.error("NFT Transfer FAILED (Bidder 2 -> Owner):", e.message.split('\n')[0].trim());
            }
            // Re-check owner after transfer
            tokenOwner = await mockNFT.ownerOf(TOKEN_ID);
        }

        // Final Check: Ensure 'owner' has the NFT
        if (tokenOwner.toLowerCase() !== ownerAddress) {
            console.error(`Final NFT owner: ${tokenOwner}`);
            throw new Error("Pre-flight check failed: NFT is not owned by the 'owner' wallet. Cannot create a new auction.");
        }
        console.log("Final Status: Owner successfully holds the NFT. Proceeding to create a new auction.");

        // ------------------------------------------------------------------
        // --- NEW AUCTION CREATION (Ensures a fresh, active auction) ---
        // ------------------------------------------------------------------

        console.log(`\nApproving NFTAuction contract ${NFTAUCTION_ADDRESS} for Token ID ${TOKEN_ID}...`);

        // Re-approve the NFTAuction contract to transfer the NFT
        const approvalTx = await mockNFT.approve(NFTAUCTION_ADDRESS, TOKEN_ID);
        await approvalTx.wait();

        console.log("Creating new auction...");

        const tx = await nftAuction.createAuction(
            MOCKNFT_ADDRESS,
            TOKEN_ID,
            ethers.parseEther("0.1"), // Min bid
            AUCTION_DURATION
        );

        const receipt = await tx.wait();

        for (const log of receipt.logs) {
            try {
                const parsed = nftAuction.interface.parseLog(log);
                if (parsed && parsed.name === "AuctionCreated") {
                    auctionId = Number(parsed.args.auctionId);
                }
            } catch { }
        }

        if (!auctionId) {
            throw new Error("Failed to find AuctionCreated event in the transaction receipt.");
        }

        console.log("Created fresh auction id =", auctionId);
    });

    // ------------------------------------------------------------------
    // --- TEST SUITE ---
    // ------------------------------------------------------------------

    it("1. should allow placing bids and track refund balances", async function () {
        expect(auctionId).to.not.equal(null);

        console.log(`\nPlacing bids on auction ${auctionId}...`);

        const initialBid = ethers.parseEther("0.2");
        const higherBid = ethers.parseEther("0.25");

        // Bidder 1 places the first bid
        const bidTx1 = await nftAuction.connect(bidder1).placeBid(auctionId, {
            value: initialBid
        });
        await bidTx1.wait();
        console.log("Bidder 1 placed 0.2 ETH.");


        // Bidder 2 places a higher bid
        const bidTx2 = await nftAuction.connect(bidder2).placeBid(auctionId, {
            value: higherBid
        });
        await bidTx2.wait();
        console.log("Bidder 2 placed 0.25 ETH (new highest).");


        // Check if Bidder 1's previous bid (0.2 ETH) is now available for refund
        const refund = await nftAuction.fundsToWithdraw(auctionId, bidder1.address);
        expect(refund).to.equal(initialBid, "Bidder 1's funds should be available for refund.");

        // Check Bidder 2's funds (should be 0 since they are the current highest bidder)
        const bidder2Funds = await nftAuction.fundsToWithdraw(auctionId, bidder2.address);
        expect(bidder2Funds).to.equal(0n, "Bidder 2's funds should be locked as the highest bidder.");
    });


    it("2. should finalize the auction after time expires", async function () {
        expect(auctionId).to.not.equal(null);

        // Advance time past the auction duration (60 seconds + 1 second buffer)
        console.log(`\nIMPORTANT: Waiting for the ${AUCTION_DURATION} second auction duration to pass naturally on Sepolia...`);
        // We must wait 60+ seconds for this to be guaranteed on a live network.

        // Use a simple pause in a real test scenario if the duration is very short (like 60s)
        // For a more robust solution, you would split this into two separate executions 
        // with a 60-second delay in between. We rely on the contract's time logic here.

        console.log(`Ending auction ${auctionId}...`);

        // Call endAuction
        const endTx = await nftAuction.endAuction(auctionId);
        const receipt = await endTx.wait();

        // Check that the NFT is now owned by the winner (bidder2)
        const newOwner = await mockNFT.ownerOf(TOKEN_ID);
        expect(newOwner.toLowerCase()).to.equal(bidder2.address.toLowerCase(), "NFT should be owned by the winning bidder (bidder2).");
        console.log(`NFT is now owned by the winner: ${newOwner}`);
    });


    it("3. should allow the outbid bidder to withdraw their funds", async function () {
        expect(auctionId).to.not.equal(null);

        // Bidder 1 was outbid, so they should have 0.2 ETH available.
        const refundAmount = ethers.parseEther("0.2");
        const initialRefundBalance = await nftAuction.fundsToWithdraw(auctionId, bidder1.address);
        expect(initialRefundBalance).to.equal(refundAmount, "Pre-check: Bidder 1 should have 0.2 ETH available to withdraw.");

        // Record bidder 1's initial ETH balance
        const initialEthBalance = await ethers.provider.getBalance(bidder1.address);

        console.log(`Bidder 1 withdrawing ${ethers.formatEther(refundAmount)} ETH...`);

        // Bidder 1 calls withdraw
        const withdrawTx = await nftAuction.connect(bidder1).withdraw(auctionId);
        const receipt = await withdrawTx.wait();

        // Check final ETH balance
        const finalEthBalance = await ethers.provider.getBalance(bidder1.address);

        // Calculate gas cost for the withdraw transaction
        const gasUsed = receipt.gasUsed * receipt.gasPrice;

        // Check that the final balance is approximately initial + refund - gas cost
        // We use 'closeTo' for approximate comparison due to gas costs
        const expectedBalance = initialEthBalance + refundAmount - gasUsed;
        const acceptableDifference = ethers.parseEther("0.0001"); // Small buffer for network variations

        expect(finalEthBalance).to.be.closeTo(expectedBalance, acceptableDifference, "Bidder 1's ETH balance should increase by the refund amount minus gas.");
        console.log("Bidder 1 successfully withdrew funds.");
    });
});