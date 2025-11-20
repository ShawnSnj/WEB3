const { ethers } = require("hardhat");

async function main() {
    const provider = ethers.provider;

    const blocksToScan = 1000;
    const latestBlock = await provider.getBlockNumber();
    const startBlock = latestBlock - blocksToScan;

    console.log(`🔍 Scanning blocks ${startBlock} to ${latestBlock} for NFTAuction deployments...`);

    // Load local compiled NFTAuction bytecode
    const NFTAuctionArtifact = await ethers.getContractFactory("NFTAuction");
    const localBytecode = NFTAuctionArtifact.bytecode;

    for (let i = latestBlock; i > startBlock; i--) {
        const block = await provider.getBlock(i);

        for (const txHash of block.transactions) {
            const tx = await provider.getTransaction(txHash);

            // Contract creation tx has no `to` field
            if (!tx.to) {
                const receipt = await provider.getTransactionReceipt(tx.hash);
                const code = await provider.getCode(receipt.contractAddress);

                if (code.startsWith(localBytecode.slice(0, 10))) {
                    console.log("✅ Found NFTAuction contract!");
                    console.log("Block:", i);
                    console.log("Tx Hash:", tx.hash);
                    console.log("Contract Address:", receipt.contractAddress);
                }
            }
        }
    }

    console.log("🔎 Scan complete.");
}

main().catch((err) => {
    console.error(err);
    process.exitCode = 1;
});
