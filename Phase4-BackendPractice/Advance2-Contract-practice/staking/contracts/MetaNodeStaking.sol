// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// 引入ERC20接口
interface IERC20 {
    function transfer(address recipient, uint256 amount) external returns (bool);
    function transferFrom(address sender, address recipient, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}

// MetaNode质押合约
contract MetaNodeStaking {
    
    // 质押池数据结构
    struct Pool {
        address stTokenAddress; // 质押代币的地址
        uint256 poolWeight; // 质押池的权重
        uint256 lastRewardBlock; // 最后一次奖励计算的区块号
        uint256 accMetaNodePerST; // 每个质押代币累计的 MetaNode 奖励
        uint256 stTokenAmount; // 池中质押代币的总量
        uint256 minDepositAmount; // 最小质押金额
        uint256 unstakeLockedBlocks; // 解锁解除质押所需的区块数
    }

    // 用户数据结构
    struct User {
        uint256 stAmount; // 用户质押的代币数量
        uint256 finishedMetaNode; // 已领取的 MetaNode 奖励数量
        uint256 pendingMetaNode; // 待领取的 MetaNode 奖励数量
        Request[] requests; // 解除质押请求
    }

    // 解除质押请求
    struct Request {
        uint256 amount; // 解除质押的数量
        uint256 unlockBlock; // 解锁区块数
    }

    // MetaNode奖励代币
    IERC20 public metaNodeToken;

    // 质押池映射
    mapping(uint256 => Pool) public pools;
    uint256 public poolCount;

    // 用户质押数据映射
    mapping(address => mapping(uint256 => User)) public users;

    // 合约管理员地址
    address public admin;

    // 合约升级角色
    address public upgradeRole;

    modifier onlyAdmin() {
        require(msg.sender == admin, "Only admin can execute");
        _;
    }

    modifier onlyUpgradeRole() {
        require(msg.sender == upgradeRole, "Only upgrade role can execute");
        _;
    }

    modifier updateReward(uint256 _pid, address _user) {
        if (block.number > pools[_pid].lastRewardBlock) {
            // 计算奖励
            Pool storage pool = pools[_pid];
            uint256 stAmount = pool.stTokenAmount;
            if (stAmount > 0) {
                uint256 reward = (block.number - pool.lastRewardBlock) * pool.poolWeight;
                pool.accMetaNodePerST += reward / stAmount;
            }
            pool.lastRewardBlock = block.number;
        }
        _;
    }

    constructor(address _metaNodeToken) {
        metaNodeToken = IERC20(_metaNodeToken);
        admin = msg.sender;
        upgradeRole = msg.sender;
    }

    // 质押功能
    function stake(uint256 _pid, uint256 _amount) external updateReward(_pid, msg.sender) {
        require(_amount >= pools[_pid].minDepositAmount, "Amount is below the minimum deposit requirement");

        Pool storage pool = pools[_pid];
        User storage user = users[msg.sender][_pid];

        // 扣除用户质押代币
        IERC20(pool.stTokenAddress).transferFrom(msg.sender, address(this), _amount);

        // 更新用户质押数量
        user.stAmount += _amount;
        pool.stTokenAmount += _amount;

        // 更新奖励
        user.pendingMetaNode = user.stAmount * pool.accMetaNodePerST - user.finishedMetaNode;
    }

    // 解除质押功能
    function unstake(uint256 _pid, uint256 _amount) external updateReward(_pid, msg.sender) {
        User storage user = users[msg.sender][_pid];
        require(user.stAmount >= _amount, "Not enough staked tokens");

        // 新增解除质押请求
        uint256 unlockBlock = block.number + pools[_pid].unstakeLockedBlocks;
        user.requests.push(Request({
            amount: _amount,
            unlockBlock: unlockBlock
        }));

        // 更新质押数量
        user.stAmount -= _amount;
        pools[_pid].stTokenAmount -= _amount;
    }

    // 领取奖励
    function claimReward(uint256 _pid) external updateReward(_pid, msg.sender) {
        User storage user = users[msg.sender][_pid];
        uint256 reward = user.pendingMetaNode;
        require(reward > 0, "No reward to claim");

        // 更新已领取奖励
        user.finishedMetaNode += reward;
        user.pendingMetaNode = 0;

        // 转账奖励
        metaNodeToken.transfer(msg.sender, reward);
    }

    // 添加或更新质押池
    function addOrUpdatePool(
        address _stTokenAddress,
        uint256 _poolWeight,
        uint256 _minDepositAmount,
        uint256 _unstakeLockedBlocks
    ) external onlyAdmin {
        require(_stTokenAddress != address(0), "Invalid token address");
        require(_poolWeight > 0, "Invalid pool weight");

        // 更新已有池或新增池
        pools[poolCount] = Pool({
            stTokenAddress: _stTokenAddress,
            poolWeight: _poolWeight,
            lastRewardBlock: block.number,
            accMetaNodePerST: 0,
            stTokenAmount: 0,
            minDepositAmount: _minDepositAmount,
            unstakeLockedBlocks: _unstakeLockedBlocks
        });

        poolCount++;
    }

    // 升级合约
    function upgradeContract(address newContract) external onlyUpgradeRole {
        // 更新合约地址
        metaNodeToken = IERC20(newContract);
    }

    // 暂停/恢复操作
    function pauseStaking() external onlyAdmin {
        // 实现暂停逻辑
    }

    function resumeStaking() external onlyAdmin {
        // 实现恢复逻辑
    }
}
