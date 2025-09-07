// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

import "@openzeppelin/contracts/access/Ownable.sol";

contract BeggingContract is Ownable{
    mapping(address => uint) public donations;
    event DonationReceived(address indexed donor, uint amount);
    uint256 public constant LEADERBOARD_SIZE = 3;

    address[] public topDonors;

    constructor() Ownable(msg.sender){
    }

    function donate() public payable {
        require(msg.value>0, "Donation amount must be greater than zero.");
        donations[msg.sender] += msg.value;
        emit DonationReceived(msg.sender, msg.value); 

        _updateLeaderboard(msg.sender);
    }

    function withdraw() public onlyOwner{
        require(address(this).balance > 0, "No funds to withdraw.");
        (bool success,) = payable(msg.sender).call{value:address(this).balance}("");
        require(success, "Failed to send Ether");
    }

    function getDonation(address donor) public view returns (uint) {
    return donations[donor];
    }

    function _updateLeaderboard(address _dornor) internal{
        bool found = false;
        for(uint i=0; i<topDonors.length; i++){
            if(topDonors[i] == _dornor){
                found = true;
                break ;
            }
        }
        if(!found){
            if(topDonors.length < LEADERBOARD_SIZE){
                topDonors.push(_dornor);
            }else{
                // 如果排行榜已滿，找到捐贈金額最低的地址
                address lowestDonor = topDonors[0];
                uint lowestAmount = donations[lowestDonor];
                for(uint j=1;j<LEADERBOARD_SIZE;j++){
                    if(donations[topDonors[j]] < lowestAmount){
                        lowestDonor = topDonors[j];
                        lowestAmount = donations[lowestDonor];
                    }
                }
                //如果新捐款者的捐款金額大於排行榜最小金額，替換它
                if(donations[_dornor] > lowestAmount){
                    for(uint k=0;k<LEADERBOARD_SIZE;k++){
                        if(topDonors[k] == lowestDonor){
                            topDonors[k] = _dornor;
                            break;
                        }
                    }
                }

            }
        }
    }
}