import 'dotenv/config';
import axios from "axios";

const address = "0xeE567Fe1712Faf6149d80dA1E6934E354124CfE3";
const apiKey = process.env.ETHERSCAN_API_KEY;

const url = `https://api.etherscan.io/v2/api?chainid=11155111&module=account&action=txlist&address=${address}&sort=asc&apikey=${apiKey}`;

try {
  const { data } = await axios.get(url);

  console.log("Raw Etherscan V2 response:\n", JSON.stringify(data, null, 2));

  // Use optional chaining to handle flexible response structures
  const txs = data?.data?.result || data?.result || [];

  if (!Array.isArray(txs)) {
    console.error("❌ Unexpected response structure:", data);
    process.exit(1);
  }

  // Filter out contract creation transactions
  const contracts = txs
    .filter(tx => !tx.to || tx.to === "")
    .map(tx => ({
      hash: tx.hash,
      contract: tx.contractAddress,
      block: tx.blockNumber
    }));

  if (contracts.length === 0) {
    console.log("No contract creation transactions found for this address.");
  } else {
    console.log("\n📜 Contracts created by", address);
    console.table(contracts);
  }

} catch (err) {
  console.error("🚨 Request failed:", err.response?.data || err.message);
}
