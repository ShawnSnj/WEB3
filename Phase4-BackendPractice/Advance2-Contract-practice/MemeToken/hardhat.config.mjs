// hardhat.config.mjs

// 1. Correct ESM import for dotenv
import * as dotenv from 'dotenv';
dotenv.config(); 

import "@nomicfoundation/hardhat-toolbox"; 

// 2. Use the correct environment variable name for the private key
const SEPOLIA_RPC_URL = process.env.SEPOLIA_RPC_URL || ""; 
const SEPOLIA_PRIVATE_KEY = process.env.PRIVATE_KEY || ""; // Using PRIVATE_KEY as per your .env naming
const ETHERSCAN_API_KEY = process.env.ETHERSCAN_API_KEY || ""; 

const config = {
    solidity: {
        version: "0.8.28", 
        settings: {
            optimizer: {
                enabled: true,
                runs: 200,
            },
        },
    },
    etherscan: {
        //apiKey: ETHERSCAN_API_KEY, // Hardhat will read the key from the env variable
        // Alternatively, you can use the nested structure shown in the error:
        
        apiKey: {
            sepolia: ETHERSCAN_API_KEY 
        }
        
    },
    networks: {
        localhost: { 
            url: "http://127.0.0.1:8545",
        },
        sepolia: {
            // Hardhat will now see the actual URL from process.env
            url: SEPOLIA_RPC_URL, 
            accounts: SEPOLIA_PRIVATE_KEY ? [SEPOLIA_PRIVATE_KEY] : [],
            chainId: 11155111,
        },
    },
};

export default config;