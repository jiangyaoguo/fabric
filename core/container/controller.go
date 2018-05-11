/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package container

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/container/api"
	"github.com/hyperledger/fabric/core/container/ccintf"
	"github.com/hyperledger/fabric/core/container/dockercontroller"
	"github.com/hyperledger/fabric/core/container/inproccontroller"
	"github.com/hyperledger/fabric/core/container/mockcontroller"
	pb "github.com/hyperledger/fabric/protos/peer"
)

type VMProvider interface {
	NewVM() api.VM
}

type refCountedLock struct {
	refCount int
	lock     *sync.RWMutex
}

//VMController - manages VMs
//   . abstract construction of different types of VMs (we only care about Docker for now)
//   . manage lifecycle of VM (start with build, start, stop ...
//     eventually probably need fine grained management)
type VMController struct {
	sync.RWMutex
	// Handlers for each chaincode
	containerLocks map[string]*refCountedLock
	vmProviders    map[string]VMProvider
}

var vmLogger = flogging.MustGetLogger("container")

// NewVMController creates a new instance of VMController
func NewVMController() *VMController {
	return &VMController{
		containerLocks: make(map[string]*refCountedLock),
		vmProviders: map[string]VMProvider{
			DOCKER: dockercontroller.NewProvider(),
			SYSTEM: inproccontroller.NewProvider(),
			MOCKVM: mockcontroller.NewProvider(),
		},
	}
}

//constants for supported containers
const (
	DOCKER = "Docker"
	SYSTEM = "System"
	MOCKVM = "MockVM"
)

func (vmc *VMController) newVM(typ string) api.VM {
	v, ok := vmc.vmProviders[typ]
	if !ok {
		vmLogger.Panicf("Programming error: unsupported VM type: %s", typ)
	}
	return v.NewVM()
}

func (vmc *VMController) lockContainer(id string) {
	//get the container lock under global lock
	vmc.Lock()
	var refLck *refCountedLock
	var ok bool
	if refLck, ok = vmc.containerLocks[id]; !ok {
		refLck = &refCountedLock{refCount: 1, lock: &sync.RWMutex{}}
		vmc.containerLocks[id] = refLck
	} else {
		refLck.refCount++
		vmLogger.Debugf("refcount %d (%s)", refLck.refCount, id)
	}
	vmc.Unlock()
	vmLogger.Debugf("waiting for container(%s) lock", id)
	refLck.lock.Lock()
	vmLogger.Debugf("got container (%s) lock", id)
}

func (vmc *VMController) unlockContainer(id string) {
	vmc.Lock()
	if refLck, ok := vmc.containerLocks[id]; ok {
		if refLck.refCount <= 0 {
			panic("refcnt <= 0")
		}
		refLck.lock.Unlock()
		if refLck.refCount--; refLck.refCount == 0 {
			vmLogger.Debugf("container lock deleted(%s)", id)
			delete(vmc.containerLocks, id)
		}
	} else {
		vmLogger.Debugf("no lock to unlock(%s)!!", id)
	}
	vmc.Unlock()
}

//VMCReqIntf - all requests should implement this interface.
//The context should be passed and tested at each layer till we stop
//note that we'd stop on the first method on the stack that does not
//take context
type VMCReqIntf interface {
	do(ctxt context.Context, v api.VM) VMCResp
	getCCID() ccintf.CCID
}

//VMCResp - response from requests. resp field is a anon interface.
//It can hold any response. err should be tested first
type VMCResp struct {
	Err  error
	Resp interface{}
}

//StartContainerReq - properties for starting a container.
type StartContainerReq struct {
	ccintf.CCID
	Builder       api.BuildSpecFactory
	Args          []string
	Env           []string
	FilesToUpload map[string][]byte
	PrelaunchFunc api.PrelaunchFunc
}

func (si StartContainerReq) do(ctxt context.Context, v api.VM) VMCResp {
	var resp VMCResp

	if err := v.Start(ctxt, si.CCID, si.Args, si.Env, si.FilesToUpload, si.Builder, si.PrelaunchFunc); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

func (si StartContainerReq) getCCID() ccintf.CCID {
	return si.CCID
}

//StopContainerReq - properties for stopping a container.
type StopContainerReq struct {
	ccintf.CCID
	Timeout uint
	//by default we will kill the container after stopping
	Dontkill bool
	//by default we will remove the container after killing
	Dontremove bool
}

func (si StopContainerReq) do(ctxt context.Context, v api.VM) VMCResp {
	var resp VMCResp

	if err := v.Stop(ctxt, si.CCID, si.Timeout, si.Dontkill, si.Dontremove); err != nil {
		resp = VMCResp{Err: err}
	} else {
		resp = VMCResp{}
	}

	return resp
}

func (si StopContainerReq) getCCID() ccintf.CCID {
	return si.CCID
}

//Process should be used as follows
//   . construct a context
//   . construct req of the right type (e.g., CreateImageReq)
//   . call it in a go routine
//   . process response in the go routing
//context can be cancelled. VMCProcess will try to cancel calling functions if it can
//For instance docker clients api's such as BuildImage are not cancelable.
//In all cases VMCProcess will wait for the called go routine to return
func (vmc *VMController) Process(ctxt context.Context, vmtype string, req VMCReqIntf) (VMCResp, error) {
	v := vmc.newVM(vmtype)
	if v == nil {
		return VMCResp{}, fmt.Errorf("Unknown VM type %s", vmtype)
	}

	c := make(chan struct{})
	var resp VMCResp
	go func() {
		defer close(c)

		id, err := v.GetVMName(req.getCCID(), nil)
		if err != nil {
			resp = VMCResp{Err: err}
			return
		}
		vmc.lockContainer(id)
		resp = req.do(ctxt, v)
		vmc.unlockContainer(id)
	}()

	select {
	case <-c:
		return resp, nil
	case <-ctxt.Done():
		//TODO cancel req.do ... (needed) ?
		<-c
		return VMCResp{}, ctxt.Err()
	}
}

// GetChaincodePackageBytes creates bytes for docker container generation using the supplied chaincode specification
func GetChaincodePackageBytes(spec *pb.ChaincodeSpec) ([]byte, error) {
	if spec == nil || spec.ChaincodeId == nil {
		return nil, fmt.Errorf("invalid chaincode spec")
	}

	return platforms.GetDeploymentPayload(spec)
}
