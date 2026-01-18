/**
 * Network Configuration for Production Deployment
 * 
 * Contains addresses for Aave, Uniswap, and tokens across different EVM networks
 */

const networks = {
    mainnet: {
        name: "Ethereum Mainnet",
        chainId: 1,
        rpcUrl: process.env.MAINNET_RPC_URL || "https://eth.llamarpc.com",
        explorer: "https://etherscan.io",
        
        // Aave V3
        aavePoolAddressesProvider: "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e",
        aavePool: "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2",
        
        // Uniswap
        uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
        uniswapV2Router: "0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D",
        useV3: true,
        defaultPoolFee: 3000, // 0.3%
        
        // Tokens
        tokens: {
            WETH: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
            USDC: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
            USDT: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
            DAI: "0x6B175474E89094C44Da98b954EedeAC495271d0F",
            WBTC: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599"
        }
    },
    
    arbitrum: {
        name: "Arbitrum One",
        chainId: 42161,
        rpcUrl: process.env.ARBITRUM_RPC_URL || "https://arb1.arbitrum.io/rpc",
        explorer: "https://arbiscan.io",
        
        // Aave V3
        aavePoolAddressesProvider: "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
        aavePool: "0x794a61358D6845594F94dc1DB02A252b5b4814aD",
        
        // Uniswap
        uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
        uniswapV2Router: "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24", // SushiSwap on Arbitrum
        useV3: true,
        defaultPoolFee: 3000,
        
        // Tokens
        tokens: {
            WETH: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1",
            USDC: "0xaf88d065e77c8cC2239327C5EDb3A432268e5831", // Native USDC
            USDCe: "0xFF970A61A04b1cA14834A43f5dE4533eBDDB5CC8", // Bridged USDC
            USDT: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9",
            DAI: "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1",
            ARB: "0x912CE59144191C1204E64559FE8253a0e49E6548"
        }
    },
    
    polygon: {
        name: "Polygon",
        chainId: 137,
        rpcUrl: process.env.POLYGON_RPC_URL || "https://polygon-rpc.com",
        explorer: "https://polygonscan.com",
        
        // Aave V3
        aavePoolAddressesProvider: "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
        aavePool: "0x794a61358D6845594F94dc1DB02A252b5b4814aD",
        
        // Uniswap
        uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
        uniswapV2Router: "0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506", // QuickSwap
        useV3: true,
        defaultPoolFee: 3000,
        
        // Tokens
        tokens: {
            WETH: "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619",
            USDC: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
            USDT: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
            DAI: "0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063",
            WMATIC: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270"
        }
    },
    
    optimism: {
        name: "Optimism",
        chainId: 10,
        rpcUrl: process.env.OPTIMISM_RPC_URL || "https://mainnet.optimism.io",
        explorer: "https://optimistic.etherscan.io",
        
        // Aave V3
        aavePoolAddressesProvider: "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
        aavePool: "0x794a61358D6845594F94dc1DB02A252b5b4814aD",
        
        // Uniswap
        uniswapV3Router: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
        uniswapV2Router: "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24",
        useV3: true,
        defaultPoolFee: 3000,
        
        // Tokens
        tokens: {
            WETH: "0x4200000000000000000000000000000000000006",
            USDC: "0x7F5c764cBc14f9669B88837ca1490cCa17c31607",
            USDT: "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58",
            DAI: "0xDA10009cBd5D07dd0CeCc66161FC93D7c9000da1"
        }
    },
    
    base: {
        name: "Base",
        chainId: 8453,
        rpcUrl: process.env.BASE_RPC_URL || "https://mainnet.base.org",
        explorer: "https://basescan.org",
        
        // Aave V3
        aavePoolAddressesProvider: "0xe20fCBdBfFC4Dd138cE8b2E6FBb6CB49777ad64D",
        aavePool: "0xA238Dd80C259a72e81d7e4664a9801593F98d1c5",
        
        // Uniswap
        uniswapV3Router: "0x2626664c2603336E57B271c5C0b26F421741e481",
        uniswapV2Router: "0x4752ba5dbc23f44d87826276bf6fd6b1c372ad24",
        useV3: true,
        defaultPoolFee: 3000,
        
        // Tokens
        tokens: {
            WETH: "0x4200000000000000000000000000000000000006",
            USDC: "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913"
        }
    },
    
    sepolia: {
        name: "Sepolia Testnet",
        chainId: 11155111,
        rpcUrl: process.env.SEPOLIA_RPC_URL || "https://rpc.sepolia.org",
        explorer: "https://sepolia.etherscan.io",
        
        // Aave V3 Sepolia (testnet addresses)
        // Note: Addresses verified from deployed Pool contract
        aavePoolAddressesProvider: "0x012bAC54348C0E635dCAc9D5FB99f06F24136C9A", // Auto-derived from Pool
        aavePool: "0x6Ae43d3271ff6888e7Fc43Fd7321a503ff738951",
        
        // Uniswap V2 Sepolia (testnet)
        uniswapV3Router: "0x3bFA4769FB09eefC5a80d6E87c3B9C650f7Ae48E", // Sepolia V3 Router (if available)
        uniswapV2Router: "0xC532a74256D3Db42D0Bf7a0400fEFDbad7694008", // Sepolia V2 Router
        useV3: false, // Use V2 on Sepolia as V3 may not be fully deployed
        defaultPoolFee: 3000,
        
        // Testnet tokens (Sepolia)
        tokens: {
            WETH: "0xfFf9976782d46CC05630D1f6eBAb18b2324d6B14", // Sepolia WETH
            USDC: "0x94a9D9AC8a22534E3FaCa9F4e7F2E2cf85d5E4C8", // Sepolia USDC (test token)
            DAI: "0x3e622317f8C93f7328350cF0B56d9eD4C620C5d6"  // Sepolia DAI (test token)
        }
    }
};

/**
 * Get network configuration
 */
function getNetworkConfig(networkName) {
    const config = networks[networkName];
    if (!config) {
        throw new Error(`Unknown network: ${networkName}. Available: ${Object.keys(networks).join(", ")}`);
    }
    return config;
}

/**
 * Get all supported networks
 */
function getSupportedNetworks() {
    return Object.keys(networks);
}

module.exports = {
    networks,
    getNetworkConfig,
    getSupportedNetworks
};
