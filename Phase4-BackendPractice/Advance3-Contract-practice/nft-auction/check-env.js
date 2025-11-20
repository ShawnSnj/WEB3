require("dotenv").config();

console.log("SEPOLIA_RPC_URL =", process.env.SEPOLIA_RPC_URL);
console.log("PRIVATE_KEY =", process.env.PRIVATE_KEY ? "loaded" : "not loaded");
console.log("ETHERSCAN_API_KEY =", process.env.ETHERSCAN_API_KEY ? "loaded" : "not loaded");
