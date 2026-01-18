require("@nomicfoundation/hardhat-toolbox");
require("dotenv").config();
const { networks: networkConfigs } = require("./config/networks");

module.exports = {
  solidity: {
    version: "0.8.28",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
      viaIR: true, // Required to fix "stack too deep" errors
    },
  },
  networks: {
    hardhat: {
      forking: {
        url: process.env.MAINNET_RPC_URL || "https://eth.llamarpc.com",
      },
      chainId: 31337,
    },
    mainnet: {
      url: networkConfigs.mainnet.rpcUrl,
      chainId: networkConfigs.mainnet.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
    arbitrum: {
      url: networkConfigs.arbitrum.rpcUrl,
      chainId: networkConfigs.arbitrum.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
    polygon: {
      url: networkConfigs.polygon.rpcUrl,
      chainId: networkConfigs.polygon.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
    optimism: {
      url: networkConfigs.optimism.rpcUrl,
      chainId: networkConfigs.optimism.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
    base: {
      url: networkConfigs.base.rpcUrl,
      chainId: networkConfigs.base.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
    sepolia: {
      url: networkConfigs.sepolia.rpcUrl,
      chainId: networkConfigs.sepolia.chainId,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
      gasPrice: "auto",
    },
  },
  etherscan: {
    apiKey: {
      mainnet: process.env.ETHERSCAN_API_KEY || "",
      arbitrumOne: process.env.ARBISCAN_API_KEY || "",
      polygon: process.env.POLYGONSCAN_API_KEY || "",
      optimisticEthereum: process.env.OPTIMISTIC_ETHERSCAN_API_KEY || "",
      base: process.env.BASESCAN_API_KEY || "",
      sepolia: process.env.ETHERSCAN_API_KEY || "", // Sepolia uses same API key as mainnet
    },
  },
  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
  },
};