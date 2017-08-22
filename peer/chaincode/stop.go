/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode

import (
	"fmt"

	"github.com/spf13/cobra"
)

var chaincodeStopCmd *cobra.Command

// invokeCmd returns the cobra command for Chaincode Invoke
func stopCmd(cf *ChaincodeCmdFactory) *cobra.Command {
	chaincodeStopCmd = &cobra.Command{
		Use:       "stop",
		Short:     fmt.Sprintf("Stop the specified chaicnode."),
		Long:      fmt.Sprintf("Stop the specified chaicnode. It will disable a running chaincode."),
		ValidArgs: []string{"1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return chaincodeStop(cmd, args, cf)
		},
	}

	flagList := []string{
		"name",
		"channelID",
	}
	attachFlags(chaincodeStopCmd, flagList)

	return chaincodeStopCmd
}

// chaincodeStop disable a chaincode.
func chaincodeStop(cmd *cobra.Command, args []string, cf *ChaincodeCmdFactory) error {
	var err error
	if cf == nil {
		cf, err = InitCmdFactory(true, true)
		if err != nil {
			return err
		}
	}
	defer cf.BroadcastClient.Close()

	env, err := action(cmd, cf, "stop")
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
