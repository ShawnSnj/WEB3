import { ethers, upgrades } from "hardhat";
import { expect } from "chai";



describe("NFT Auction Marketplace", function () {
  let nft: any;
  let factory: any;
  let auction: any;
  let owner: any, addr1: any, addr2: any;

  beforeEach(async function () {
    [owner, addr1, addr2] = await ethers.getSigners();

    const NFT = await ethers.getContractFactory("MyNFT");
    nft = await NFT.deploy();
    await nft.waitForDeployment();

    const Factory = await ethers.getContractFactory("AuctionFactory");
    factory = await upgrades.deployProxy(Factory, [], {
      initializer: "initialize",
      kind: "uups",
    });
    await factory.waitForDeployment();
  });

  it("should mint NFT to a user", async function () {
    await nft.mint(addr1.address, 1);
    expect(await nft.ownerOf(1)).to.equal(addr1.address);
  });

  it("should create an auction via factory", async function () {
    await nft.mint(owner.address, 1);
    await nft.approve(await factory.getAddress(), 1);

    const tx = await factory.createAuction(
      nft.getAddress(),
      1,
      ethers.utils.parseEther("0.1"),
      3600
    );
    const receipt = await tx.wait();
    const auctionAddress = receipt?.logs[0]?.args?.auction;

    auction = await ethers.getContractAt("Auction", auctionAddress);
    expect(await auction.owner()).to.equal(owner.address);
  });

  // 更多測試案例可擴展：
  // - 出價 ETH，轉換成 USD 驗證
  // - 出價 ERC20（模擬 ERC20 + Chainlink price feed）
  // - 檢查低價 bid 被拒絕
  // - 拍賣結束後資產正確轉移
});
