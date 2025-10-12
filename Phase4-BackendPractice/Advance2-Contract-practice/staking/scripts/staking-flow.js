const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("MetaNodeStaking Full Flow", function () {
  let token, staking;
  let owner, user;
  const initialStake = ethers.parseUnits("100", 18);
  const unstakeAmount = ethers.parseUnits("50", 18);

  const metaNodeTokenAddress = "0x4Fb5922D38e025BC7b30Fb09D1C25D5866c9e652"; // Already deployed token
  const stakingAddress = "0x6941EA063e5d0657AF04950cBd8F3BF0d3Ed1c34";     // Already deployed staking contract

  before(async function () {
    [owner, user] = await ethers.getSigners();

    token = await ethers.getContractAt("MetaNodeToken", metaNodeTokenAddress);
    staking = await ethers.getContractAt("MetaNodeStaking", stakingAddress);

    // Transfer some tokens to user if they don’t have
    const userBalance = await token.balanceOf(user.address);
    if (userBalance.lt(initialStake)) {
      const tx = await token.transfer(user.address, initialStake);
      await tx.wait();
    }

    // Add a pool if not yet done (poolId = 0)
    const pool = await staking.pools(0).catch(() => null);
    if (!pool || pool.stTokenAddress === ethers.ZeroAddress) {
      const tx = await staking.connect(owner).addOrUpdatePool(
        metaNodeTokenAddress,
        10,           // poolWeight
        ethers.parseUnits("10", 18), // minDepositAmount
        10            // unstakeLockedBlocks
      );
      await tx.wait();
    }
  });

  it("✅ Stake tokens", async function () {
    await token.connect(user).approve(stakingAddress, initialStake);
    const tx = await staking.connect(user).stake(0, initialStake);
    await tx.wait();

    const userInfo = await staking.users(user.address, 0);
    expect(userInfo.stAmount).to.equal(initialStake);
    console.log("✅ Staked:", ethers.formatUnits(userInfo.stAmount, 18), "tokens");
  });

  it("⛓️ Simulate blocks and unstake", async function () {
    // Dummy stake by owner to trigger reward update (optional)
    const dummyAmount = ethers.parseUnits("1", 18);
    await token.connect(owner).approve(stakingAddress, dummyAmount);
    await staking.connect(owner).stake(0, dummyAmount);

    // Unstake
    const tx = await staking.connect(user).unstake(0, unstakeAmount);
    await tx.wait();

    const userInfo = await staking.users(user.address, 0);
    expect(userInfo.stAmount).to.equal(initialStake - unstakeAmount);
    console.log("⛓️ Unstaked:", ethers.formatUnits(unstakeAmount, 18));
  });

  it("🎁 Claim rewards", async function () {
    const pending = await staking.users(user.address, 0).then(u => u.pendingMetaNode);
    console.log("🔎 Pending reward before claim:", ethers.formatUnits(pending, 18));

    const claimTx = await staking.connect(user).claimReward(0);
    await claimTx.wait();

    const pendingAfter = await staking.users(user.address, 0).then(u => u.pendingMetaNode);
    console.log("✅ Claimed. Pending after:", ethers.formatUnits(pendingAfter, 18));

    const balance = await token.balanceOf(user.address);
    console.log("📦 User token balance after claim:", ethers.formatUnits(balance, 18));
  });
});
