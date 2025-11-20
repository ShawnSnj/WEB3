const fs = require("fs");
const path = require("path");

async function main() {
    const deploymentPath = path.join(__dirname, "../deployments/sepolia.json");

    if (!fs.existsSync(deploymentPath)) {
        console.log("❌ No deployment found. Run deploy.js first.");
        return;
    }

    const data = JSON.parse(fs.readFileSync(deploymentPath));

    console.log("📌 Deployment Info (Sepolia)");
    console.log("--------------------------------");
    console.log("MockNFT address:     ", data.MockNFT);
    console.log("NFTAuction address:  ", data.NFTAuction);
}

main();
