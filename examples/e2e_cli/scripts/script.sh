#!/bin/bash
# Copyright London Stock Exchange Group All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

source ./scripts/steps.sh

echo
echo " ____    _____      _      ____    _____           _____   ____    _____ "
echo "/ ___|  |_   _|    / \    |  _ \  |_   _|         | ____| |___ \  | ____|"
echo "\___ \    | |     / _ \   | |_) |   | |    _____  |  _|     __) | |  _|  "
echo " ___) |   | |    / ___ \  |  _ <    | |   |_____| | |___   / __/  | |___ "
echo "|____/    |_|   /_/   \_\ |_| \_\   |_|           |_____| |_____| |_____|"
echo

## Check for orderering service availablility
echo "Check orderering service availability..."
checkOSNAvailability

## Create channel
echo "Creating channel..."
createChannel

## Join all the peers to the channel
echo "Having all peers join the channel..."
joinChannel

## Set the anchor peers for each org in the channel
echo "Updating anchor peers for org1..."
updateAnchorPeers 0
echo "Updating anchor peers for org2..."
updateAnchorPeers 2

## Install chaincode on Peer0/Org1 and Peer2/Org2
echo "Installing chaincode v1 on org1/peer0..."
installChaincode 0 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 1.0
echo "Install chaincode v1 on org2/peer2..."
installChaincode 2 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 1.0

#Instantiate chaincode on Peer2/Org2
echo "Instantiating chaincode on org2/peer2..."
instantiateChaincode 2 1.0 '{"Args":["init","a","100","b","200"]}'

#Query on chaincode on Peer0/Org1
echo "Querying chaincode on org1/peer0..."
chaincodeQuery 0 a 100

#Invoke on chaincode on Peer0/Org1
echo "Sending invoke transaction on org1/peer0..."
chaincodeInvoke 0 '{"Args":["invoke","a","b","10"]}'

## Install new chaincode on Peer3/Org2
echo "Installing chaincode v2 on org2/peer3..."
installChaincode 3 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_example02 1.0

#Query on chaincode on Peer3/Org2
echo "Querying chaincode on org2/peer3..."
chaincodeQuery 3 a 90

echo
echo "===================== All GOOD, End-2-End execution completed ===================== "
echo

echo
echo " _____   _   _   ____            _____   ____    _____ "
echo "| ____| | \ | | |  _ \          | ____| |___ \  | ____|"
echo "|  _|   |  \| | | | | |  _____  |  _|     __) | |  _|  "
echo "| |___  | |\  | | |_| | |_____| | |___   / __/  | |___ "
echo "|_____| |_| \_| |____/          |_____| |_____| |_____|"
echo

exit 0
