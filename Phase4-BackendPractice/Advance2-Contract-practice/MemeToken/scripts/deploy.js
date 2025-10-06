// This is your corrected deploy script
import hre from "hardhat"; 

async function main() {
    // Use `hre` to access ethers from the Hardhat Runtime Environment
    const [deployer] = await hre.ethers.getSigners();

    console.log("Deploying contracts with the account:", deployer.address);

    const supplyInEther = "1000000"; // 1 million tokens
    const initialSupply = hre.ethers.parseEther(supplyInEther); // Convert to wei


    // Get the MemeToken contract
    const MemeToken = await hre.ethers.getContractFactory("MemeToken");

    // Deploy the contract with an initial supply of 1 million tokens (example)
    const memeToken = await MemeToken.deploy(initialSupply);
    await memeToken.waitForDeployment()

    console.log("MemeToken deployed to:",await memeToken.getAddress());
}

// Run the deployment
main()
    .then(() => process.exit(0))
    .catch((error) => {
        console.error(error);
        process.exit(1);
    });
