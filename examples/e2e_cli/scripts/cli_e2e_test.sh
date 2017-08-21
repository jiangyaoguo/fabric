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
installChaincode 0 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_v1 1.0
echo "Install chaincode v1 on org2/peer2..."
installChaincode 2 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_v1 1.0

#Instantiate chaincode on Peer2/Org2
echo "Instantiating chaincode v1 on org2/peer2..."
instantiateChaincode 2 1.0 '{"Args":["init","100","200"]}'

#Invoke on chaincode on Peer0/Org1
echo "send Invoke transaction on org1/peer0 ..."
chaincodeInvoke 0 '{"Args":["invoke","10"]}'

#Query on chaincode on Peer2/Org2
echo "send Invoke transaction on org1/peer0 ..."
chaincodeQuery 2 a 90

#Stop chaincode
echo "stop chaincode on org1/peer0"
stopChaincode 0

## Install new chaincode on Peer0/Org1 and Peer2/Org2
echo "Installing chaincode v2 on org1/peer0..."
installChaincode 0 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_v2 2.0
echo "Install chaincode v2 on org2/peer2..."
installChaincode 2 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_v2 2.0

#Upgrade chaincode to version 2.0 on Peer2/Org2
echo "upgrade chaincode to v2 on org2/peer2 ..."
upgradeChaincode 2 2.0 '{"Args":["init","a","200","c","300"]}'

#Start chaincode
echo "start chaincode on org1/peer0"
startChaincode 0

#Query on chaincode on Peer0/Org1
echo "Querying chaincode on org1/peer0..."
chaincodeQuery 0 a 200
chaincodeQuery 0 b 210
chaincodeQuery 0 c 300

#Invoke on chaincode on Peer0/Org1
echo "Sending invoke transaction on org1/peer0..."
chaincodeInvoke 0 '{"Args":["invoke","a","c","10"]}'

## Install new chaincode on Peer3/Org2
echo "Installing chaincode v2 on org2/peer3..."
installChaincode 3 github.com/hyperledger/fabric/examples/chaincode/go/chaincode_v2 2.0

#Query on chaincode on Peer3/Org2
echo "Querying chaincode on org2/peer3..."
chaincodeQuery 3 a 190
chaincodeQuery 3 b 210
chaincodeQuery 3 c 310

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
