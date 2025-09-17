import type { HardhatUserConfig } from "hardhat/config";
import "@nomiclabs/hardhat-waffle";
import "@nomiclabs/hardhat-ethers";
import "@openzeppelin/hardhat-upgrades";
import "hardhat-deploy";
import * as dotenv from "dotenv";

dotenv.config();

// 手動補上 namedAccounts 的型別
const config: HardhatUserConfig & {
  namedAccounts: {
    [name: string]: {
      [network: string]: number | string;
    };
  };
} = {
  solidity: "0.8.28",

  networks: {
    hardhat: {},
    sepolia: {
      url: process.env.SEPOLIA_RPC || "",
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },

  namedAccounts: {
    deployer: {
      default: 0, // 第一個帳戶
    },
  },

  paths: {
    sources: "./contracts",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
    deployments: "./deployments",
  },
};

export default config;
