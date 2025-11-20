require("dotenv").config();
const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("NFTAuction 資金餘額狀態檢查 (Sepolia)", function () {
    // 設置較長的超時時間以適應 Sepolia 網絡延遲
    this.timeout(120000);

    let owner, bidder1, bidder2;
    let nftAuction;

    // --- 合約資訊 ---
    const NFTAUCTION_ADDRESS = "0x627bEd9E638C4158da5d79cA503006361F7c2b66";
    // 檢查最近幾次拍賣的 ID，您可以根據實際情況調整此列表
    const AUCTION_IDS_TO_CHECK = [1, 2, 3, 4, 5, 6, 7];

    before(async function () {
        // 🟢 1. 初始化錢包和合約連接
        const provider = ethers.provider;

        // 使用環境變數中的真實錢包
        owner = new ethers.Wallet(process.env.PRIVATE_KEY, provider);
        bidder1 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER1, provider);
        bidder2 = new ethers.Wallet(process.env.PRIVATE_KEY_BIDDER2, provider);

        // 連接到已部署的合約
        nftAuction = await ethers.getContractAt("NFTAuction", NFTAUCTION_ADDRESS, owner);

        console.log(`\n--- 開始檢查合約及參與者餘額 ---`);
        console.log(`合約地址: ${NFTAUCTION_ADDRESS}`);
        console.log(`檢查的拍賣 ID 範圍: ${AUCTION_IDS_TO_CHECK.join(', ')}`);
    });

    it("應正確記錄並列印合約與所有參與者的資金餘額", async function () {

        const participants = [
            { signer: owner, role: "賣家 (Owner)", address: owner.address },
            { signer: bidder1, role: "競標者 1", address: bidder1.address },
            { signer: bidder2, role: "競標者 2", address: bidder2.address }
        ];

        // 1. 檢查合約總餘額
        const contractBalance = await ethers.provider.getBalance(NFTAUCTION_ADDRESS);
        console.log(`\n=================================================`);
        console.log(`🏠 合約總餘額 (NFTAuction): ${ethers.formatEther(contractBalance)} ETH`);
        console.log(`=================================================`);


        // 2. 迭代檢查每個參與者和每個拍賣的餘額
        for (const { role, address } of participants) {
            console.log(`\n--- 👥 參與者: ${role} (${address.substring(0, 10)}...) ---`);

            // 獲取參與者鏈上 ETH 餘額
            const onChainBalance = await ethers.provider.getBalance(address);
            console.log(`   [鏈上餘額]: ${ethers.formatEther(onChainBalance)} ETH`);

            // 檢查在每個拍賣中可提取的資金
            let totalWithdrawable = 0n;

            for (const auctionId of AUCTION_IDS_TO_CHECK) {
                const withdrawableFunds = await nftAuction.fundsToWithdraw(auctionId, address);

                if (withdrawableFunds > 0n) {
                    console.log(`   [拍賣 ID ${auctionId} 可提取]: ${ethers.formatEther(withdrawableFunds)} ETH`);
                    totalWithdrawable += withdrawableFunds;
                }
            }

            if (totalWithdrawable > 0n) {
                console.log(`   [總計可提取資金]: ${ethers.formatEther(totalWithdrawable)} ETH`);
            } else {
                console.log("   [總計可提取資金]: 0.0 ETH (無退款或收入待提取)");
            }
        }

        console.log(`\n--- 檢查完成 ---`);

        // 最終斷言：確保測試通過，即使只是讀取數據
        expect(true).to.be.true;
    });
});