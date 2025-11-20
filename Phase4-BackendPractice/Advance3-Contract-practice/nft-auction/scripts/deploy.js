const { ethers } = require("hardhat");

async function main() {
    const [deployer] = await ethers.getSigners();
    console.log("🚀 Deploying contracts with:", deployer.address);

    // 1️⃣ 部署 MockNFT
    const MockNFT = await ethers.getContractFactory("MockNFT");
    const mockNFT = await MockNFT.deploy();
    await mockNFT.waitForDeployment();
    console.log("✅ MockNFT deployed to:", await mockNFT.getAddress());

    // 2️⃣ 部署 NFTAuction
    const NFTAuction = await ethers.getContractFactory("NFTAuction");
    const nftAuction = await NFTAuction.deploy();
    await nftAuction.waitForDeployment();
    console.log("✅ NFTAuction deployed to:", await nftAuction.getAddress());

    // 3️⃣ 測試 mint + approve
    const mintTx = await mockNFT.mintTo(deployer.address);
    await mintTx.wait();
    console.log("🖼️ Minted tokenId #0 to", deployer.address);

    const approveTx = await mockNFT.approve(await nftAuction.getAddress(), 0);
    await approveTx.wait();
    console.log("✅ Approved NFTAuction to transfer tokenId #0");

    // 4️⃣ 建立拍賣
    const startPrice = ethers.parseEther("0.1");
    const duration = 60 * 5; // 5 分鐘
    const createAuctionTx = await nftAuction.createAuction(
        await mockNFT.getAddress(),
        0,
        startPrice,
        duration
    );
    await createAuctionTx.wait();
    console.log("📦 Auction created successfully!");
}

main().catch((err) => {
    console.error(err);
    process.exitCode = 1;
});
