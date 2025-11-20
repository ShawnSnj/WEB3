const { ethers } = require("ethers");

const rawAddresses = [
    "0xA1C9eB073A3fcf5Dd832a7bc5E3dcdc193F5B4C4",
    "0x4CD44c056491aE1E7ee80440C8bBbC80085e2BC0",
    "0x826f6fCee44E324dE5467ffB851F312Ff3A3247C"
];

// 将地址转化为符合 EIP-55 校验和标准的格式
const addresses = rawAddresses.map(addr => ethers.utils.getAddress(addr));
console.log(addresses);
