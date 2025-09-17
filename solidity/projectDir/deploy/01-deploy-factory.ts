import { ethers, upgrades } from "hardhat";

const deployFactory = async () => {
  const [deployer] = await ethers.getSigners();
  const Factory = await ethers.getContractFactory("AuctionFactory");

  const factoryProxy = await upgrades.deployProxy(Factory, [process.env.ETH_USD_FEED], { initializer: "initialize" });

  await factoryProxy.deployed();

  console.log("Factory deployed to:", factoryProxy.address);
};

export default deployFactory;

deployFactory.tags = ["Factory"];
