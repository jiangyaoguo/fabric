/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package common

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/hyperledger/fabric/core/comm"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
)

// PeerClient represents a client for communicating with a peer
type PeerClient struct {
	commonClient
}

// NewPeerClientFromEnv creates an instance of a PeerClient from the global
// Viper instance
func NewPeerClientFromEnv() (*PeerClient, error) {
	address, override, clientConfig, err := configFromEnv("peer")
	if err != nil {
		return nil, errors.WithMessage(err, "failed to load config for PeerClient")
	}
	return newPeerClientForClientConfig(address, override, clientConfig)
}

// NewPeerClientForAddress creates an instance of a PeerClient using the
// provided peer address and, if TLS is enabled, the TLS root cert file
func NewPeerClientForAddress(address, tlsRootCertFile string) (*PeerClient, error) {
	if address == "" {
		return nil, errors.New("peer address must be set")
	}

	_, override, clientConfig, err := configFromEnv("peer")
	if clientConfig.SecOpts.UseTLS {
		if tlsRootCertFile == "" {
			return nil, errors.New("tls root cert file must be set")
		}
		caPEM, res := ioutil.ReadFile(tlsRootCertFile)
		if res != nil {
			err = errors.WithMessage(res, fmt.Sprintf("unable to load TLS root cert file from %s", tlsRootCertFile))
			return nil, err
		}
		clientConfig.SecOpts.ServerRootCAs = [][]byte{caPEM}
	}
	return newPeerClientForClientConfig(address, override, clientConfig)
}

func newPeerClientForClientConfig(address, override string, clientConfig comm.ClientConfig) (*PeerClient, error) {
	// set timeout
	clientConfig.Timeout = time.Second * 3
	gClient, err := comm.NewGRPCClient(clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create PeerClient from config")
	}
	pClient := &PeerClient{
		commonClient: commonClient{
			GRPCClient: gClient,
			address:    address,
			sn:         override}}
	return pClient, nil
}

// Endorser returns a client for the Endorser service
func (pc *PeerClient) Endorser() (pb.EndorserClient, error) {
	conn, err := pc.commonClient.NewConnection(pc.address, pc.sn)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("endorser client failed to connect to %s", pc.address))
	}
	return pb.NewEndorserClient(conn), nil
}

// Admin returns a client for the Admin service
func (pc *PeerClient) Admin() (pb.AdminClient, error) {
	conn, err := pc.commonClient.NewConnection(pc.address, pc.sn)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("admin client failed to connect to %s", pc.address))
	}
	return pb.NewAdminClient(conn), nil
}

// GetEndorserClient returns a new endorser client. If the both the address and
// tlsRootCertFile are not provided, the target values for the client are taken
// from the configuration settings for "peer.address" and
// "peer.tls.rootcert.file"
func GetEndorserClient(address string, tlsRootCertFile string) (pb.EndorserClient, error) {
	var peerClient *PeerClient
	var err error
	if address != "" {
		peerClient, err = NewPeerClientForAddress(address, tlsRootCertFile)
	} else {
		peerClient, err = NewPeerClientFromEnv()
	}
	if err != nil {
		return nil, err
	}
	return peerClient.Endorser()
}

// GetAdminClient returns a new admin client.  The target address for
// the client is taken from the configuration setting "peer.address"
func GetAdminClient() (pb.AdminClient, error) {
	peerClient, err := NewPeerClientFromEnv()
	if err != nil {
		return nil, err
	}
	return peerClient.Admin()
}
