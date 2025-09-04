// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

contract MyToken{
    string public name = "MyToken";
    string public symbol = "MTK";
    uint8 public decimals = 18;
    uint256 public totalSupply;

    address public owner;

    mapping(address => uint256) private balances;
    mapping(address => mapping(address => uint256)) private allowances;

    //Events
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(){
        owner = msg.sender;
    }

    //Modifiers
    modifier onlyOwner(){
        require(msg.sender  == owner,"Not contract Owner");
        _;
    }

    function balanceOf(address account) public view returns (uint256){
        return balances[account];
    }

    function transfer(address to, uint256 amount) public returns(bool){
        require(balances[msg.sender]>amount,"Insufficient balance");
        require(to != address(0),"Invalid address");

        balances[msg.sender] -= amount;
        balances[to] += amount;

        emit Transfer(msg.sender,to,amount);
        return true;
    }

    function approve(address spender, uint256 amount) public returns(bool){
        require(spender != address(0), "Invalid spender address");

        allowances[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);

        return true;
    }

    function allowance(address owner_,address spender) public view returns(uint256){
        return allowances[owner_][spender];
    }
    

    function transferFrom(address from, address to, uint256 amount) public returns(bool){
        require(balances[from] > amount, "Insufficient balance");
        require(allowances[from][msg.sender] >= amount, "Insufficient allowance");
        require(to != address(0), "Invalid address");

        balances[from] -= amount;
        balances[to] += amount;

        allowances[from][msg.sender] -= amount;

        emit Transfer(from, to, amount);

        return true;
    }

    function mint(address to, uint256 amount) public onlyOwner{
        require(to != address(0), "Invalid address");

        totalSupply += amount;
        balances[to] += amount;
        emit Transfer(address(0), to, amount);
    }
}
