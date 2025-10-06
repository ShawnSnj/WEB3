// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

// 导入 Uniswap V2 路由接口
interface IUniswapV2Router02 {
    function factory() external pure returns (address);
    function WETH() external pure returns (address);

    function addLiquidityETH(
        address token,
        uint amountTokenDesired,
        uint amountTokenMin,
        uint amountETHMin,
        address to,
        uint deadline
    ) external payable returns (uint amountToken, uint amountETH, uint liquidity);

    function swapExactTokensForETHSupportingFeeOnTransferTokens(
        uint amountIn,
        uint amountOutMin,
        address[] calldata path,
        address to,
        uint deadline
    ) external;
}

// 导入 Uniswap V2 工厂接口
interface IUniswapV2Factory {
    function createPair(address tokenA, address tokenB) external returns (address pair);
}

contract MemeTaxToken is ERC20, Ownable {
    // --- 核心配置 ---

    // 交易税率配置 (以万分之几表示，例如 500 = 5%)
    uint256 public taxBuyFee = 500;    // 买入（从LP池获得代币）税
    uint256 public taxSellFee = 1000;   // 卖出（向LP池出售代币）税

    // 税费分配比例 (总比例必须 <= 10000)
    uint256 public taxMarketing = 200; // 营销/国库分配
    uint256 public taxLiquidity = 300; // 自动添加到流动性池的比例
    uint256 public taxBurn = 0;        // 销毁比例

    // 交易额度限制配置
    uint256 public maxTxAmount;      // 单笔交易最大代币数量 (例如 1% of total supply)
    uint256 public maxWalletAmount;  // 单个钱包最大持仓数量 (例如 2% of total supply)

    // --- 流动性池相关配置 ---

    // Uniswap V2 路由地址 (Sepolia/主网地址不同，部署时需指定)
    IUniswapV2Router02 public uniswapV2Router;
    // 交易对合约地址 (MemeToken/WETH Pair)
    address public uniswapV2Pair;

    // 营销/国库钱包地址
    address payable public marketingWallet;

    // 内部状态变量
    bool private swapping; // 防止重入，标记当前是否处于交易/自动加流动性过程中

    // 排除税收的地址（如 LP 池、交易所、自身合约）
    mapping(address => bool) public isFeeExempt;
    // 排除交易额度限制的地址
    mapping(address => bool) public isTxLimitExempt;

    // --- 事件 ---
    event TaxDeducted(address indexed from, address indexed to, uint256 amount, uint256 taxAmount);
    event AutoLiquify(uint256 amountETH, uint256 amountTokens);
    event TaxWalletUpdated(address indexed newWallet);

    // --- 构造函数 ---
    constructor(
        uint256 initialSupply, // 初始总供应量 (含小数位数)
        address routerAddress, // Uniswap V2 路由地址
        address payable walletAddress // 营销钱包地址
    ) ERC20("Meme Tax Token", "MTT") Ownable() {
        
        // 1. 设置营销钱包
        marketingWallet = walletAddress;

        // 2. 设置初始限制 (假设初始供应量为 10^24，即 100万亿个代币)
        uint256 totalSupply_ = initialSupply * 10**decimals();
        maxTxAmount = totalSupply_ / 100; // 1% of total supply
        maxWalletAmount = totalSupply_ / 50; // 2% of total supply

        // 3. 排除自身、Owner、Router和营销钱包的税费和限制
        isFeeExempt[owner()] = true;
        isFeeExempt[address(this)] = true;
        isFeeExempt[marketingWallet] = true;

        isTxLimitExempt[owner()] = true;
        isTxLimitExempt[address(this)] = true;
        isTxLimitExempt[marketingWallet] = true;

        // 4. 初始化路由和流动性池
        uniswapV2Router = IUniswapV2Router02(routerAddress);
        isFeeExempt[routerAddress] = true; 
        isTxLimitExempt[routerAddress] = true;

        // 5. 铸造初始代币并转移给合约拥有者
        _mint(msg.sender, totalSupply_);
    }

    // --- 核心 ERC-20 覆盖函数 ---

    // 覆盖 _transfer 函数以实现税费、流动性集成和交易限制
    function _transfer(
        address from,
        address to,
        uint256 amount
    ) internal override {
        // 0. 执行交易前的检查
        require(from != address(0) && to != address(0), "ERC20: transfer from the zero address");
        
        // 1. 交易限制检查 (不适用于被排除的地址)
        if (!isTxLimitExempt[from] && !isTxLimitExempt[to]) {
            // 1.1 交易额度限制
            require(amount <= maxTxAmount, "MemeTax: Exceeds Max Tx Amount");

            // 1.2 钱包持仓限制 (检查接收方，不适用于销毁地址或LP池自身)
            if (to != address(uniswapV2Router) && to != uniswapV2Pair) {
                require(balanceOf(to) + amount <= maxWalletAmount, "MemeTax: Exceeds Max Wallet Amount");
            }
        }

        // 2. 税费计算与扣除
        if (isFeeExempt[from] || isFeeExempt[to]) {
            // 如果任一地址被豁免，则不征税，执行标准转账
            super._transfer(from, to, amount);
            return;
        }

        // 2.1 确定当前交易是否为买入/卖出，并确定税率
        uint256 currentTax = 0;
        bool isSell = (from == uniswapV2Pair); // 默认为买入
        bool isBuy = (to == uniswapV2Pair);     // 默认为卖出
        
        if (isSell) {
             currentTax = taxSellFee; // 卖出税
        } else if (isBuy) {
             currentTax = taxBuyFee;  // 买入税
        } else {
             currentTax = taxBuyFee; // 普通转账使用买入税
        }

        // 2.2 计算税额
        uint256 taxAmount = (amount * currentTax) / 10000;
        uint256 amountToTransfer = amount - taxAmount;

        // 2.3 扣除税费并转移到合约自身
        if (taxAmount > 0) {
            super._transfer(from, address(this), taxAmount);
            emit TaxDeducted(from, to, amount, taxAmount);
        }

        // 2.4 转移剩余代币给接收方
        super._transfer(from, to, amountToTransfer);


        // 3. 自动添加流动性 (仅在卖出时触发，且不在交换过程中)
        if (uniswapV2Pair != address(0) && isSell && !swapping) {
            // 达到阈值后，将合约持有的税费代币进行兑换和加流动性
            uint256 contractTokenBalance = balanceOf(address(this));
            
            // 只有当合约持有的代币达到可用于流动性税的最低要求时才进行操作
            if (contractTokenBalance >= maxTxAmount) { 
                swapping = true;
                
                // 3.1 计算用于流动性/营销的代币数量
                uint256 tokenForLiquidity = (contractTokenBalance * taxLiquidity) / 10000;
                uint256 tokenForMarketing = (contractTokenBalance * taxMarketing) / 10000;
                uint256 tokenToSwap = tokenForLiquidity / 2; // 只兑换流动性代币的一半用于ETH配对
                
                // 3.2 兑换代币为 ETH (实现自动流动性添加的核心步骤)
                _swapTokensForETH(tokenToSwap);
                
                // 3.3 自动添加流动性
                uint256 ethReceived = address(this).balance;
                if (ethReceived > 0) {
                    _addLiquidity(tokenForLiquidity - tokenToSwap, ethReceived);
                }

                // 3.4 转移营销税到钱包
                if (tokenForMarketing > 0) {
                    super._transfer(address(this), marketingWallet, tokenForMarketing);
                }
                
                swapping = false;
            }
        }
    }

    // --- 内部辅助函数 ---

    // 内部函数：将代币兑换为 ETH
    function _swapTokensForETH(uint256 tokenAmount) private {
        // 允许 Router 消费本合约的代币
        _approve(address(this), address(uniswapV2Router), tokenAmount);

        address[] memory path = new address[](2);
        path[0] = address(this);
        path[1] = uniswapV2Router.WETH();

        // 执行兑换
        uniswapV2Router.swapExactTokensForETHSupportingFeeOnTransferTokens(
            tokenAmount,
            0, // 接受的最小 ETH 数量设置为 0，以避免在内部交易中失败
            path,
            address(this),
            block.timestamp
        );
    }

    // 内部函数：添加流动性
    function _addLiquidity(uint256 tokenAmount, uint256 ethAmount) private {
        // 允许 Router 消费本合约的代币
        _approve(address(this), address(uniswapV2Router), tokenAmount);

        // 添加流动性
        uniswapV2Router.addLiquidityETH{value: ethAmount}(
            address(this),
            tokenAmount,
            0, // 最小 Token 数量
            0, // 最小 ETH 数量
            marketingWallet, // 将 LP 代币发送给营销钱包
            block.timestamp
        );

        emit AutoLiquify(ethAmount, tokenAmount);
    }
    
    // --- 外部管理函数 (仅限 Owner) ---

    // Owner: 设置/启用流动性池交易对
    function setSwapAndLiquifyEnabled(bool _enabled) public onlyOwner {
        // 设置交易对地址
        if (uniswapV2Pair == address(0)) {
            uniswapV2Pair = IUniswapV2Factory(uniswapV2Router.factory())
                .createPair(address(this), uniswapV2Router.WETH());
            
            // 立即豁免 LP 池地址的税费和限制
            isFeeExempt[uniswapV2Pair] = true;
            isTxLimitExempt[uniswapV2Pair] = true;
        }
        // 额外的逻辑可以放在这里，例如启动自动加流动性
    }
    
    // Owner: 设置交易税
    function setTaxes(uint256 _buyFee, uint256 _sellFee) public onlyOwner {
        require(_buyFee <= 2000 && _sellFee <= 2000, "Tax must be <= 20%"); // 限制最大税率
        taxBuyFee = _buyFee;
        taxSellFee = _sellFee;
    }
    
    // Owner: 设置税费分配比例
    function setTaxSplit(uint256 _marketing, uint256 _liquidity, uint256 _burn) public onlyOwner {
        require(_marketing + _liquidity + _burn <= 10000, "Split exceeds 100%");
        taxMarketing = _marketing;
        taxLiquidity = _liquidity;
        taxBurn = _burn;
    }

    // Owner: 设置交易限制
    function setLimits(uint256 _maxTx, uint256 _maxWallet) public onlyOwner {
        require(_maxTx >= totalSupply() / 1000, "Max Tx must be at least 0.1% supply");
        require(_maxWallet >= totalSupply() / 500, "Max Wallet must be at least 0.2% supply");
        maxTxAmount = _maxTx;
        maxWalletAmount = _maxWallet;
    }

    // Owner: 豁免地址的税费
    function setFeeExempt(address holder, bool exempt) public onlyOwner {
        isFeeExempt[holder] = exempt;
    }

    // Owner: 豁免地址的交易限制
    function setTxLimitExempt(address holder, bool exempt) public onlyOwner {
        isTxLimitExempt[holder] = exempt;
    }
    
    // 允许合约接收 ETH 以便进行自动流动性添加
    receive() external payable {}
}