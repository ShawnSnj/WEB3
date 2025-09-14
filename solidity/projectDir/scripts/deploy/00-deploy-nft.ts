import { ethers, deployments } from "hardhat";

export default async function deployNFT() {
  const { deployer } = await ethers.getNamedSigners();
  const NFT = await ethers.getContractFactory("MyNFT");
  const nft = await NFT.deploy();
  await nft.deployed();
  console.log("NFT deployed to:", nft.address);
}

module.exports.tags = ["NFT"];