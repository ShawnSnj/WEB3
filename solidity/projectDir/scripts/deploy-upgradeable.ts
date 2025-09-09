import { ethers, upgrades } from "hardhat";
async function main() {
  
    const AuctionUpgradeable = await ethers.getContractFactory("AuctionUpgradeable");

    const proxy = await upgrades.deployProxy(AuctionUpgradeable, [], { kind: 'uups'});
    await proxy.deployed();

    console.log("AuctionUpgradeable deployed to:", proxy.address);
}
 