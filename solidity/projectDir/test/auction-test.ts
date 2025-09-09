import { expect } from "chai";
import { ethers } from "hardhat";
import { Contract } from "ethers";

describe("Auction", function () {
  let nft: any;
  let auction: any;
  let priceFeedMock: any;
  let owner: any;
  let bidder1: any;
  let bidder2: any;

  beforeEach(async function () {
    [owner, bidder1, bidder2] = await ethers.getSigners();

    // ✅ 部署 Mock Price Feed (Chainlink)
    const PriceFeedMock = await ethers.getContractFactory("MockV3Aggregator");
    priceFeedMock = await PriceFeedMock.deploy(8, 2000 * 10 ** 8); // $2000 per ETH
    await priceFeedMock.deployed();

    // ✅ 部署 NFT 合约并铸造一个 NFT
    const NFT = await ethers.getContractFactory("NFT");
    nft = await NFT.deploy();
    await nft.deployed();

    const tokenURI = "ipfs://my-nft-uri";
    const mintTx = await nft.connect(owner).mint(tokenURI);
    await mintTx.wait();

    const tokenId = 0;

    // ✅ 部署 Auction 合约
    const Auction = await ethers.getContractFactory("Auction");
    auction = await Auction.deploy(
      nft.address,
      tokenId,
      60, // 60 秒拍卖时间
      //ethers.constants.AddressZero, // 不用 ERC20，表示 ETH 出价
      priceFeedMock.address
    );
    await auction.deployed();

    // ✅ 授权 NFT 转移
    await nft.connect(owner).approve(auction.address, tokenId);
  });

  it("should allow ETH bidding and end auction", async function () {
    const tokenId = 0;

    // ✅ 手动 transfer NFT 到拍卖合约（模拟 constructor 中的 transfer）
    await nft.connect(owner).transferFrom(owner.address, auction.address, tokenId);

    // ✅ bidder1 出价 1 ETH
    // await auction.connect(bidder1).bidWithETH({ value: ethers.utils.parseEther("1.0") });

    // ✅ bidder2 出价 1.5 ETH
    // await auction.connect(bidder2).bidWithETH({ value: ethers.utils.parseEther("1.5") });

    // ✅ 尝试较低出价（应失败）
    // await expect(
    //   auction.connect(bidder1).bidWithETH({ value: ethers.utils.parseEther("1.1") })
    // ).to.be.revertedWith("Bid too low");

    // ✅ 跳过时间：等待拍卖结束
    await ethers.provider.send("evm_increaseTime", [70]);
    await ethers.provider.send("evm_mine", []);

    // ✅ 结束拍卖（由卖家执行）
    await auction.connect(owner).endAuction();

    // ✅ NFT 应归属于 bidder2
    expect(await nft.ownerOf(tokenId)).to.equal(bidder2.address);
  });
});
