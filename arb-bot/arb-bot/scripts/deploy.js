const { ethers } = require("hardhat");

async function main() {
  const [deployer] = await ethers.getSigners();
  console.log("Deployer:", deployer.address);

  const SimpleDEX = await ethers.getContractFactory("SimpleDEX");

  // Two pools with different prices.
  // dex1: reserveX=1,000,000; reserveY=2,000,000  => price(Y/X)=2
  // dex2: reserveX=1,000,000; reserveY=1,000,000  => price(Y/X)=1
  const dex1 = await SimpleDEX.deploy(1_000_000, 2_000_000);
  await dex1.waitForDeployment();

  const dex2 = await SimpleDEX.deploy(1_000_000, 1_000_000);
  await dex2.waitForDeployment();

  console.log("SimpleDEX #1:", dex1.target);
  console.log("SimpleDEX #2:", dex2.target);

  const FlashArb = await ethers.getContractFactory("FlashArb");
  const arb = await FlashArb.deploy();
  await arb.waitForDeployment();
  console.log("FlashArb:", arb.target);

  console.log("\nFill `config/config.json` with:");
  console.log(
    JSON.stringify(
      {
        dex1_address: dex1.target,
        dex2_address: dex2.target,
        arb_contract: arb.target,
      },
      null,
      2
    )
  );
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

