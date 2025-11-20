const { ethers } = require("hardhat");

async function main() {
    const [owner, bidder1, bidder2] = await ethers.getSigners();

    console.log("Deploying MockNFT...");
    const MockNFT = await ethers.getContractFactory("MockNFT");
    const mockNFT = await MockNFT.deploy();
    await mockNFT.waitForDeployment();
    console.log("MockNFT deployed to:", await mockNFT.getAddress());

    console.log("Deploying NFTAuction...");
    const NFTAuction = await ethers.getContractFactory("NFTAuction");
    const nftAuction = await NFTAuction.deploy();
    await nftAuction.waitForDeployment();
    console.log("NFTAuction deployed to:", await nftAuction.getAddress());

    // Mint NFT to owner
    console.log("Minting NFT to owner...");
    const txMint = await mockNFT.mintTo(owner.address);
    await txMint.wait();
    console.log("NFT minted with tokenId 0");

    // Approve NFTAuction to transfer NFT
    const txApprove = await mockNFT.approve(await nftAuction.getAddress(), 0);
    await txApprove.wait();
    console.log("NFT approved to NFTAuction");

    // Create auction
    const startPrice = ethers.parseEther("0.1");
    const duration = 60; // 60 seconds
    console.log("Creating auction...");
    const txAuction = await nftAuction.createAuction(
        await mockNFT.getAddress(),
        0,
        startPrice,
        duration
    );
    await txAuction.wait();
    console.log("Auction created for tokenId 0");

    // Place bids
    console.log("Bidder1 placing bid 0.2 ETH...");
    await nftAuction.connect(bidder1).placeBid(1, { value: ethers.parseEther("0.2") });

    console.log("Bidder2 placing bid 0.25 ETH...");
    await nftAuction.connect(bidder2).placeBid(1, { value: ethers.parseEther("0.25") });

    // Check funds to withdraw for bidder1
    const refund = await nftAuction.fundsToWithdraw(1, bidder1.address);
    console.log("Bidder1 refund amount:", ethers.formatEther(refund));

    console.log(
        `Waiting ${duration + 5} seconds for auction to end (cannot simulate time on Sepolia)...`
    );
    await new Promise((resolve) => setTimeout(resolve.duration + 5000));

    console.log("Ending auction...");
    const txEnd = await nftAuction.endAuction(1);
    await txEnd.wait();

    const finalOwner = await mockNFT.ownerOf(0);
    console.log("NFT final owner:", finalOwner);
}

main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
