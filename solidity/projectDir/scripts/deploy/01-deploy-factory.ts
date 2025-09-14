import {ethers, upgrades} from "hardhat";

export async function deployFactory() {
    const {deployer} = await ethers.getNamedSigners();
    const Factory = await ethers.getContractFactory("AuctionFactory");


    const factoryProxy = await upgrades.deployProxy(Factory,[process.env.ETH_USD_FEED],{initializer:"initialize"});

    await factoryProxy.deployed();
    
    console.log("Factory deployed to:", factoryProxy.address);   
}

module.exports.tags = ["Factory"];