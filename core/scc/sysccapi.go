/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package scc

import (
	"errors"
	"fmt"

	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/core/common/sysccprovider"
	"github.com/hyperledger/fabric/core/container/inproccontroller"
	"github.com/hyperledger/fabric/core/peer"

	"github.com/spf13/viper"

	pb "github.com/hyperledger/fabric/protos/peer"
)

var sysccLogger = flogging.MustGetLogger("sccapi")

// SystemChaincode defines the metadata needed to initialize system chaincode
// when the fabric comes up. SystemChaincodes are installed by adding an
// entry in importsysccs.go
type SystemChaincode struct {
	//Unique name of the system chaincode
	Name string

	//Path to the system chaincode; currently not used
	Path string

	//InitArgs initialization arguments to startup the system chaincode
	InitArgs [][]byte

	// Chaincode can be invoked to create the actual chaincode instance
	Chaincode func(sccp sysccprovider.SystemChaincodeProvider) shim.Chaincode

	// InvokableExternal keeps track of whether
	// this system chaincode can be invoked
	// through a proposal sent to this peer
	InvokableExternal bool

	// InvokableCC2CC keeps track of whether
	// this system chaincode can be invoked
	// by way of a chaincode-to-chaincode
	// invocation
	InvokableCC2CC bool

	// Enabled a convenient switch to enable/disable system chaincode without
	// having to remove entry from importsysccs.go
	Enabled bool
}

// registerSysCC registers the given system chaincode with the peer
func registerSysCC(syscc *SystemChaincode, sccp sysccprovider.SystemChaincodeProvider) (bool, error) {
	if !syscc.Enabled || !isWhitelisted(syscc) {
		sysccLogger.Info(fmt.Sprintf("system chaincode (%s,%s,%t) disabled", syscc.Name, syscc.Path, syscc.Enabled))
		return false, nil
	}

	err := inproccontroller.Register(syscc.Path, syscc.Chaincode(sccp))
	if err != nil {
		//if the type is registered, the instance may not be... keep going
		if _, ok := err.(inproccontroller.SysCCRegisteredErr); !ok {
			errStr := fmt.Sprintf("could not register (%s,%v): %s", syscc.Path, syscc, err)
			sysccLogger.Error(errStr)
			return false, fmt.Errorf(errStr)
		}
	}

	sysccLogger.Infof("system chaincode %s(%s) registered", syscc.Name, syscc.Path)
	return true, err
}

// deploySysCC deploys the given system chaincode on a chain
func deploySysCC(chainID string, syscc *SystemChaincode) error {
	if !syscc.Enabled || !isWhitelisted(syscc) {
		sysccLogger.Info(fmt.Sprintf("system chaincode (%s,%s) disabled", syscc.Name, syscc.Path))
		return nil
	}

	ccprov := ccprovider.GetChaincodeProvider()

	txid := util.GenerateUUID()

	ctxt := context.Background()
	if chainID != "" {
		lgr := peer.GetLedger(chainID)
		if lgr == nil {
			panic(fmt.Sprintf("syschain %s start up failure - unexpected nil ledger for channel %s", syscc.Name, chainID))
		}

		//init can do GetState (and other Get's) even if Puts cannot be
		//be handled. Need ledger for this
		ctxt2, txsim, err := ccprov.GetContext(lgr, txid)
		if err != nil {
			return err
		}

		ctxt = ctxt2

		defer txsim.Done()
	}

	chaincodeID := &pb.ChaincodeID{Path: syscc.Path, Name: syscc.Name}
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]), ChaincodeId: chaincodeID, Input: &pb.ChaincodeInput{Args: syscc.InitArgs}}

	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := buildSysCC(ctxt, spec)

	if err != nil {
		sysccLogger.Error(fmt.Sprintf("Error deploying chaincode spec: %v\n\n error: %s", spec, err))
		return err
	}

	version := util.GetSysCCVersion()

	cccid := ccprov.GetCCContext(chainID, chaincodeDeploymentSpec.ChaincodeSpec.ChaincodeId.Name, version, txid, true, nil, nil)

	resp, _, err := ccprov.Execute(ctxt, cccid, chaincodeDeploymentSpec)
	if err == nil && resp.Status != shim.OK {
		err = errors.New(resp.Message)
	}

	sysccLogger.Infof("system chaincode %s/%s(%s) deployed", syscc.Name, chainID, syscc.Path)

	return err
}

// DeDeploySysCC stops the system chaincode and deregisters it from inproccontroller
func DeDeploySysCC(chainID string, syscc *SystemChaincode) error {
	chaincodeID := &pb.ChaincodeID{Path: syscc.Path, Name: syscc.Name}
	spec := &pb.ChaincodeSpec{Type: pb.ChaincodeSpec_Type(pb.ChaincodeSpec_Type_value["GOLANG"]), ChaincodeId: chaincodeID, Input: &pb.ChaincodeInput{Args: syscc.InitArgs}}

	ctx := context.Background()
	// First build and get the deployment spec
	chaincodeDeploymentSpec, err := buildSysCC(ctx, spec)

	if err != nil {
		sysccLogger.Error(fmt.Sprintf("Error deploying chaincode spec: %v\n\n error: %s", spec, err))
		return err
	}

	ccprov := ccprovider.GetChaincodeProvider()

	version := util.GetSysCCVersion()

	cccid := ccprov.GetCCContext(chainID, syscc.Name, version, "", true, nil, nil)

	err = ccprov.Stop(ctx, cccid, chaincodeDeploymentSpec)

	return err
}

// buildLocal builds a given chaincode code
func buildSysCC(context context.Context, spec *pb.ChaincodeSpec) (*pb.ChaincodeDeploymentSpec, error) {
	var codePackageBytes []byte
	chaincodeDeploymentSpec := &pb.ChaincodeDeploymentSpec{ExecEnv: pb.ChaincodeDeploymentSpec_SYSTEM, ChaincodeSpec: spec, CodePackage: codePackageBytes}
	return chaincodeDeploymentSpec, nil
}

func isWhitelisted(syscc *SystemChaincode) bool {
	chaincodes := viper.GetStringMapString("chaincode.system")
	val, ok := chaincodes[syscc.Name]
	enabled := val == "enable" || val == "true" || val == "yes"
	return ok && enabled
}
