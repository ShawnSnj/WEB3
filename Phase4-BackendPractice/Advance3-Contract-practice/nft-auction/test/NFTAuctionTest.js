const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("NFTAuction", function () {
    let owner, bidder1, bidder2;
    let mockNFT, nftAuction;

    beforeEach(async function () {
        [owner, bidder1, bidder2] = await ethers.getSigners();

        const MockNFT = await ethers.getContractFactory("MockNFT");
        mockNFT = await MockNFT.deploy();
        await mockNFT.waitForDeployment();

        const NFTAuction = await ethers.getContractFactory("NFTAuction");
        nftAuction = await NFTAuction.deploy();
        await nftAuction.waitForDeployment();

        // owner mint 一張 NFT
        await mockNFT.mintTo(owner.address);
        await mockNFT.approve(await nftAuction.getAddress(), 0);
    });

    it("should create auction and handle bids correctly", async function () {
        const startPrice = ethers.parseEther("0.1");
        const duration = 60; // 1 分鐘

        await nftAuction.createAuction(await mockNFT.getAddress(), 0, startPrice, duration);

        const auction = await nftAuction.auctions(1);
        expect(auction.startPrice).to.equal(startPrice);
        expect(await mockNFT.ownerOf(0)).to.equal(await nftAuction.getAddress());

        // bidder1 出價 0.2 ETH
        await nftAuction.connect(bidder1).placeBid(1, { value: ethers.parseEther("0.2") });

        // bidder2 出價太低會失敗
        await expect(
            nftAuction.connect(bidder2).placeBid(1, { value: ethers.parseEther("0.201") })
        ).to.be.revertedWith("Bid must increase by at least 0.01 Ether.");

        // bidder2 出價 0.25 ETH
        await nftAuction.connect(bidder2).placeBid(1, { value: ethers.parseEther("0.25") });

        const refund = await nftAuction.fundsToWithdraw(1, bidder1.address);
        expect(refund).to.equal(ethers.parseEther("0.2"));

        // 模擬時間流逝
        await ethers.provider.send("evm_increaseTime", [duration + 1]);
        await ethers.provider.send("evm_mine");

        // 結束拍賣
        await nftAuction.endAuction(1);

        expect(await mockNFT.ownerOf(0)).to.equal(bidder2.address);
    });
});
