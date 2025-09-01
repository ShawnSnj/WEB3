// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

contract BinarySearch{
function binarySearch(int[] memory arr, int target) public pure returns(int){
    int low = 0;
    int high = int(arr.length) - 1;

    while(low <= high){
        int mid = low + (high -low)/2;
        if(arr[uint(mid)] == target){
            return mid;
        }else if(arr[uint(mid)] < target){
            low = mid +1;
        }else{
            high = mid-1;
        }
    }
    return -1;
}
}
