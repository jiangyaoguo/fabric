/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package mockcontroller

import (
	"io"
	"sync"
	"time"

	"github.com/hyperledger/fabric/common/flogging"
	container "github.com/hyperledger/fabric/core/container/api"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"golang.org/x/net/context"
)

var (
	mockLogger = flogging.MustGetLogger("mockcontroller")
)

// Controller implements container.VMProvider
type Provider struct{}

// NewProvider creates a new instance of Provider
func NewProvider() *Provider {
	return &Provider{}
}

// NewVM creates a new MockVM instance
func (p *Provider) NewVM() container.VM {
	return &MockVM{}
}

type MockVM struct {
	runningVMs map[string]time.Time
	vmLock     sync.Mutex
}

func (mock *MockVM) Deploy(ctxt context.Context, ccid ccintf.CCID, args []string, env []string, reader io.Reader) error {
	return nil
}

func (mock *MockVM) Start(ctxt context.Context, ccid ccintf.CCID, args []string, env []string, filesToUpload map[string][]byte, builder container.BuildSpecFactory, prelaunchFunc container.PrelaunchFunc) error {
	mock.vmLock.Lock()
	defer mock.vmLock.Unlock()
	mock.runningVMs[ccid.GetName()] = time.Now()
	return nil
}

func (mock *MockVM) Stop(ctxt context.Context, ccid ccintf.CCID, timeout uint, dontkill bool, dontremove bool) error {
	mock.vmLock.Lock()
	defer mock.vmLock.Unlock()
	delete(mock.runningVMs, ccid.GetName())
	return nil
}

func (mock *MockVM) Destroy(ctxt context.Context, ccid ccintf.CCID, force bool, noprune bool) error {
	mock.vmLock.Lock()
	defer mock.vmLock.Unlock()
	delete(mock.runningVMs, ccid.GetName())
	return nil
}

func (mock *MockVM) GetVMName(ccid ccintf.CCID, formatFunc func(string) (string, error)) (string, error) {
	return ccid.GetName(), nil
}
