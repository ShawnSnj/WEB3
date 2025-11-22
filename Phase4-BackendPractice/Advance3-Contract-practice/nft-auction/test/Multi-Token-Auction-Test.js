const { expect } = require("chai");
const { ethers, upgrades } = require("hardhat");

describe("NFTAuctionUpgradeable Multi-Token Bidding", function () {
    let deployer, seller, bidder1, bidder2, bidder3, accounts;
    let nftAuction, nft, usdt, usdc, dai;
    let ethUsdOracle, usdtUsdOracle, usdcUsdOracle, daiUsdOracle;

    // 常數
    const ETH_PRICE = 3000e8; // $3000 USD (8 decimals for Oracle)
    const STABLECOIN_PRICE = 1e8; // $1 USD (8 decimals for Oracle)
    const STABLECOIN_DECIMALS = 6; // USDT/USDC common decimals
    const DAI_DECIMALS = 18; // DAI decimals
    const MIN_INCREMENT_USD = 10e8; // $10 USD (8 decimals)
    const NFT_TOKEN_ID = 1;
    const AUCTION_DURATION = 86400; // 1 day

    before(async function () {
        accounts = await ethers.getSigners();
        [deployer, seller, bidder1, bidder2, bidder3] = accounts;

        // 部署模擬預言機 (MockV3Aggregator)
        const MockV3Aggregator = await ethers.getContractFactory("MockV3Aggregator");
        ethUsdOracle = await MockV3Aggregator.deploy(ETH_PRICE);
        // 穩定幣預言機設置為 $1 USD
        usdtUsdOracle = await MockV3Aggregator.deploy(STABLECOIN_PRICE);
        usdcUsdOracle = await MockV3Aggregator.deploy(STABLECOIN_PRICE);
        daiUsdOracle = await MockV3Aggregator.deploy(STABLECOIN_PRICE);

        // 部署模擬代幣 (MockERC20)
        const MockERC20 = await ethers.getContractFactory("MockERC20");
        usdt = await MockERC20.deploy("Tether USD", "USDT", STABLECOIN_DECIMALS);
        usdc = await MockERC20.deploy("USD Coin", "USDC", STABLECOIN_DECIMALS);
        dai = await MockERC20.deploy("Dai Stablecoin", "DAI", DAI_DECIMALS);

        // 部署 NFT 模擬 (MockERC721)
        const MockERC721 = await ethers.getContractFactory("MockERC721");
        nft = await MockERC721.deploy();

        // 部署 NFTAuctionUpgradeable (UUPS 代理)
        const NFTAuction = await ethers.getContractFactory("NFTAuctionUpgradeable");
        nftAuction = await upgrades.deployProxy(
            NFTAuction,
            [await ethUsdOracle.getAddress(), MIN_INCREMENT_USD],
            { initializer: "initialize" }
        );
        await nftAuction.waitForDeployment();

        // 配置 ERC20 代幣預言機
        await nftAuction.setTokenOracle(await usdt.getAddress(), await usdtUsdOracle.getAddress());
        await nftAuction.setTokenOracle(await usdc.getAddress(), await usdcUsdOracle.getAddress());
        await nftAuction.setTokenOracle(await dai.getAddress(), await daiUsdOracle.getAddress());

        // 設置測試所需的代幣餘額
        // 鑄幣給競標者 (使用原始單位)
        const MINT_AMOUNT = 5000n * (10n ** BigInt(STABLECOIN_DECIMALS));
        await usdt.mint(await bidder1.getAddress(), MINT_AMOUNT);
        await usdc.mint(await bidder2.getAddress(), MINT_AMOUNT);
        await dai.mint(await bidder3.getAddress(), 5000n * (10n ** BigInt(DAI_DECIMALS))); // DAI 18 decimals

        // 競標者授權合約從他們的錢包中提取代幣
        await usdt.connect(bidder1).approve(await nftAuction.getAddress(), MINT_AMOUNT);
        await usdc.connect(bidder2).approve(await nftAuction.getAddress(), MINT_AMOUNT);
        await dai.connect(bidder3).approve(await nftAuction.getAddress(), 5000n * (10n ** BigInt(DAI_DECIMALS)));
    });

    it("1. Should allow the seller to create an auction and transfer the NFT", async function () {
        // 鑄造 NFT 給賣家
        const tokenId = await nft.connect(seller).mint(await seller.getAddress());
        expect(tokenId).to.equal(NFT_TOKEN_ID);

        // 賣家授權拍賣合約轉移 NFT
        await nft.connect(seller).approve(await nftAuction.getAddress(), NFT_TOKEN_ID);

        // 創建拍賣 (底價 $1000 USD)
        const START_PRICE_USD = 1000e8; // 1000 USD (8 decimals)
        await expect(
            nftAuction.connect(seller).createAuction(
                await nft.getAddress(),
                NFT_TOKEN_ID,
                START_PRICE_USD,
                AUCTION_DURATION
            )
        ).to.emit(nftAuction, "AuctionCreated")
            .withArgs(1, await seller.getAddress(), await nft.getAddress(), NFT_TOKEN_ID, START_PRICE_USD, ethers.anyValue);

        // 驗證 NFT 已轉移給拍賣合約
        expect(await nft.ownerOf(NFT_TOKEN_ID)).to.equal(await nftAuction.getAddress());

        // 驗證拍賣狀態
        const auction = await nftAuction.auctions(1);
        expect(auction.highestBidUSDValue).to.equal(START_PRICE_USD);
        expect(auction.highestBidder).to.equal(ethers.ZeroAddress);
    });

    it("2. Bidder 1 should place the first bid using USDT", async function () {
        // 競標 1: $1010 USD (需要高於底價 $1000 + 最小增量 $10)
        // USDT 6 位小數，所以是 1010 * 10^6
        const BID_AMOUNT_1_USDT = 1010n * (10n ** BigInt(STABLECOIN_DECIMALS));
        const EXPECTED_USD_VALUE = 1010n * 10n ** 8n; // 1010 USD (8 decimals)

        await expect(
            nftAuction.connect(bidder1).placeBid(
                1,
                await usdt.getAddress(),
                BID_AMOUNT_1_USDT
            )
        ).to.emit(nftAuction, "BidPlaced")
            .withArgs(1, await bidder1.getAddress(), EXPECTED_USD_VALUE, await usdt.getAddress(), BID_AMOUNT_1_USDT);

        // 驗證拍賣狀態
        const auction = await nftAuction.auctions(1);
        expect(auction.highestBidder).to.equal(await bidder1.getAddress());
        expect(auction.highestBidAmount).to.equal(BID_AMOUNT_1_USDT);
        expect(auction.highestBidUSDValue).to.equal(EXPECTED_USD_VALUE);

        // 驗證資金已轉移到合約
        expect(await usdt.balanceOf(await nftAuction.getAddress())).to.equal(BID_AMOUNT_1_USDT);
    });

    it("3. Bidder 2 should outbid Bidder 1 using USDC and allow Bidder 1 to withdraw", async function () {
        // 競標 2: $1025 USD (需要高於 $1010 + 最小增量 $10 = $1020)
        // USDC 6 位小數，所以是 1025 * 10^6
        const BID_AMOUNT_2_USDC = 1025n * (10n ** BigInt(STABLECOIN_DECIMALS));
        const EXPECTED_USD_VALUE = 1025n * 10n ** 8n; // 1025 USD (8 decimals)

        const previousBidAmount = await usdt.balanceOf(await nftAuction.getAddress());

        // 競標
        await nftAuction.connect(bidder2).placeBid(
            1,
            await usdc.getAddress(),
            BID_AMOUNT_2_USDC
        );

        // 驗證拍賣狀態
        const auction = await nftAuction.auctions(1);
        expect(auction.highestBidder).to.equal(await bidder2.getAddress());
        expect(auction.highestBidToken).to.equal(await usdc.getAddress());

        // 驗證競標 1 的資金已標記為可提取
        expect(await nftAuction.fundsToWithdraw(await bidder1.getAddress(), await usdt.getAddress())).to.equal(previousBidAmount);

        // Bidder 1 提取 USDT
        const bidder1InitialUSDT = await usdt.balanceOf(await bidder1.getAddress());
        await nftAuction.connect(bidder1).withdraw(await usdt.getAddress());

        // 驗證 Bidder 1 餘額已恢復 (初始餘額 + 退回的競標金額)
        // 由於我們給了 5000 tokens，花費了 1010，所以剩餘 3990。退回後應該是 5000。
        const expectedFinalBalance = 5000n * (10n ** BigInt(STABLECOIN_DECIMALS));
        expect(await usdt.balanceOf(await bidder1.getAddress())).to.equal(expectedFinalBalance);

        // 驗證合約中的 USDC 餘額
        expect(await usdc.balanceOf(await nftAuction.getAddress())).to.equal(BID_AMOUNT_2_USDC);
    });

    it("4. Bidder 3 should place the final bid using DAI (18 decimals) and allow Bidder 2 to withdraw", async function () {
        // 競標 3: $1040 USD (需要高於 $1025 + 最小增量 $10 = $1035)
        // DAI 18 位小數，所以是 1040 * 10^18
        const BID_AMOUNT_3_DAI = 1040n * (10n ** BigInt(DAI_DECIMALS));
        const EXPECTED_USD_VALUE = 1040n * 10n ** 8n; // 1040 USD (8 decimals)

        const previousBidAmountUSDC = await usdc.balanceOf(await nftAuction.getAddress());

        // 競標
        await nftAuction.connect(bidder3).placeBid(
            1,
            await dai.getAddress(),
            BID_AMOUNT_3_DAI
        );

        // 驗證拍賣狀態
        const auction = await nftAuction.auctions(1);
        expect(auction.highestBidder).to.equal(await bidder3.getAddress());
        expect(auction.highestBidToken).to.equal(await dai.getAddress());
        expect(auction.highestBidUSDValue).to.equal(EXPECTED_USD_VALUE);

        // 驗證競標 2 的資金已標記為可提取
        expect(await nftAuction.fundsToWithdraw(await bidder2.getAddress(), await usdc.getAddress())).to.equal(previousBidAmountUSDC);

        // Bidder 2 提取 USDC
        const bidder2InitialUSDC = 5000n * (10n ** BigInt(STABLECOIN_DECIMALS));
        await nftAuction.connect(bidder2).withdraw(await usdc.getAddress());
        expect(await usdc.balanceOf(await bidder2.getAddress())).to.equal(bidder2InitialUSDC);

        // 驗證合約中的 DAI 餘額
        expect(await dai.balanceOf(await nftAuction.getAddress())).to.equal(BID_AMOUNT_3_DAI);
    });

    it("5. Should successfully end the auction, transfer NFT to the winner and DAI to the seller", async function () {
        const auctionId = 1;
        const finalBidAmountDAI = 1040n * (10n ** BigInt(DAI_DECIMALS));
        const finalBidUSDValue = 1040n * 10n ** 8n;

        // 快進時間以結束拍賣
        await ethers.provider.send("evm_increaseTime", [AUCTION_DURATION]);
        await ethers.provider.send("evm_mine");

        // 檢查賣家的初始 DAI 餘額
        const sellerInitialDAIBalance = await dai.balanceOf(await seller.getAddress());

        // 結束拍賣
        await expect(
            nftAuction.endAuction(auctionId)
        ).to.emit(nftAuction, "AuctionEnded")
            .withArgs(auctionId, await bidder3.getAddress(), finalBidUSDValue, await seller.getAddress(), ethers.anyValue);

        // 驗證 NFT 已轉移給競標者 3
        expect(await nft.ownerOf(NFT_TOKEN_ID)).to.equal(await bidder3.getAddress());

        // 驗證賣家收到了 DAI
        const sellerFinalDAIBalance = await dai.balanceOf(await seller.getAddress());
        expect(sellerFinalDAIBalance - sellerInitialDAIBalance).to.equal(finalBidAmountDAI);

        // 驗證合約中的 DAI 餘額為 0
        expect(await dai.balanceOf(await nftAuction.getAddress())).to.equal(0);
    });
});