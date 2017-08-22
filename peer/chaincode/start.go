/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	protcommon "github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
)

var chaincodeStartCmd *cobra.Command

// invokeCmd returns the cobra command for Chaincode Invoke
func startCmd(cf *ChaincodeCmdFactory) *cobra.Command {
	chaincodeStartCmd = &cobra.Command{
		Use:       "start",
		Short:     fmt.Sprintf("Start the specified chaicnode."),
		Long:      fmt.Sprintf("Start the specified chaicnode. It will enable stopped chaincode."),
		ValidArgs: []string{"1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return chaincodeStart(cmd, args, cf)
		},
	}

	flagList := []string{
		"name",
		"channelID",
	}
	attachFlags(chaincodeStartCmd, flagList)

	return chaincodeStartCmd
}

// chaincodeStart start the stopped chaincode.
func chaincodeStart(cmd *cobra.Command, args []string, cf *ChaincodeCmdFactory) error {
	var err error
	if cf == nil {
		cf, err = InitCmdFactory(true, true)
		if err != nil {
			return err
		}
	}
	defer cf.BroadcastClient.Close()

	env, err := action(cmd, cf, "start")
	if err != nil {
		return err
	}

	if env != nil {
		logger.Debug("Send signed envelope to orderer")
		err = cf.BroadcastClient.Send(env)
		return err
	}

	return nil
}

func action(cmd *cobra.Command, cf *ChaincodeCmdFactory, action string) (*protcommon.Envelope, error) {
	creator, err := cf.Signer.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Error serializing identity for %s: %s", cf.Signer.GetIdentifier(), err)
	}

	prop, _, err := utils.CreateActionProposal(chainID, chaincodeName, action, creator)
	if err != nil {
		return nil, fmt.Errorf("Error creating proposal %s: %s", chainFuncName, err)
	}
	logger.Debugf("Get %s proposal for chaincode <%s/%s>", action, chaincodeName, chainID)

	var signedProp *pb.SignedProposal
	signedProp, err = utils.GetSignedProposal(prop, cf.Signer)
	if err != nil {
		return nil, fmt.Errorf("Error creating signed proposal  %s: %s", chainFuncName, err)
	}

	proposalResponse, err := cf.EndorserClient.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		return nil, fmt.Errorf("Error endorsing %s: %s", chainFuncName, err)
	}
	logger.Debugf("endorse chaincode %s proposal, get response <%v>", action, proposalResponse.Response)

	if proposalResponse != nil {
		// assemble a signed transaction (it's an Envelope message)
		env, err := utils.CreateSignedTx(prop, cf.Signer, proposalResponse)
		if err != nil {
			return nil, fmt.Errorf("Could not assemble transaction, err %s", err)
		}
		logger.Debug("Get Signed envelope")
		return env, nil
	}

	return nil, nil
}
