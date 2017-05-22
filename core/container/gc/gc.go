/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gc

import (
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/hyperledger/fabric/common/flogging"
	"github.com/hyperledger/fabric/core/container"
	"github.com/hyperledger/fabric/core/container/ccintf"
)

var gcLogger = flogging.MustGetLogger("GCManager")

type vmInfo struct {
	ccid       ccintf.CCID
	activeTime time.Time
	infoLock   *sync.Mutex
}

func newVmInfo(ccid ccintf.CCID, activeTime time.Time) *vmInfo {
	return &vmInfo{
		ccid:       ccid,
		activeTime: activeTime,
		infoLock:   new(sync.Mutex),
	}
}

// GCManager stops the chaincodes vms that havn't been active for vmLiveTime
type GCManager struct {
	runningVMs map[string]*vmInfo // all the vms that should be monitored
	expiredVMs map[string]*vmInfo // all the vms that should be stopped
	vmLock     *sync.RWMutex
	vmLiveTime time.Duration
	gcInterval time.Duration
	vmType     string
	exitCh     chan struct{}
}

// NewGCManager create a GCManager and start to recycle vm using this GCManager
func NewGCManager(vmType string, vmLiveTime time.Duration, gcInterval time.Duration) *GCManager {
	gc := &GCManager{
		runningVMs: make(map[string]*vmInfo),
		expiredVMs: make(map[string]*vmInfo),
		vmLock:     new(sync.RWMutex),
		vmLiveTime: vmLiveTime,
		gcInterval: gcInterval,
		vmType:     vmType,
		exitCh:     make(chan struct{}, 1),
	}
	go gc.startGC()
	return gc
}

// AddVM add the chaincode vm to GCManager
func (gc *GCManager) AddVM(canName string, ccid ccintf.CCID, activeTime time.Time) {
	gc.vmLock.Lock()
	gc.runningVMs[canName] = newVmInfo(ccid, activeTime)
	gc.vmLock.Unlock()
	gcLogger.Debugf("Add chaincode vm %s to GCManager", canName)
}

// UpdateVM updates the chaincode vm active time.
func (gc *GCManager) UpdateVM(canName string, activeTime time.Time) {
	gc.vmLock.RLock()
	info, exist := gc.runningVMs[canName]
	gc.vmLock.RUnlock()
	if exist {
		info.infoLock.Lock()
		info.activeTime = activeTime
		info.infoLock.Unlock()
	} else {
		// This should not happen, just in case
		gcLogger.Errorf("Try to update unknow chaincode vm %s", canName)
	}

	gcLogger.Debugf("Update chaincode vm %s", canName)
}

// startGC start to stop expired chaincode vm
func (gc *GCManager) startGC() {
	gcLogger.Infof("Start gc for chaincode vm")
	ticker := time.NewTicker(gc.gcInterval)
	for {
		select {
		case now := <-ticker.C:
			gc.vmLock.RLock()
			expiredTime := now.Add(-gc.vmLiveTime)
			for vmName, vmInfo := range gc.runningVMs {
				vmInfo.infoLock.Lock()
				if vmInfo.activeTime.Before(expiredTime) {
					gc.expiredVMs[vmName] = newVmInfo(vmInfo.ccid, vmInfo.activeTime)
				}
				vmInfo.infoLock.Unlock()
			}
			gc.vmLock.RUnlock()

			gc.stopExpiredVMs()
		case _ = <-gc.exitCh:
			ticker.Stop()
			return
		}
	}
}

func (gc *GCManager) stopExpiredVMs() {
	for vmName, vmInfo := range gc.expiredVMs {
		// stop vm
		err := gc.stopVM(vmInfo.ccid)
		if err != nil {
			gcLogger.Errorf("Stop vm %s error: %s", vmName, err)
		} else {
			gc.vmLock.Lock()
			delete(gc.runningVMs, vmName)
			delete(gc.expiredVMs, vmName)
			gc.vmLock.Unlock()
			gcLogger.Infof("Stop vm %s", vmName)
		}
	}
}

func (gc *GCManager) stopVM(ccid ccintf.CCID) error {
	sir := container.StopImageReq{
		CCID:       ccid,
		Timeout:    0,
		Dontkill:   false,
		Dontremove: true,
	}

	ctxt := context.Background()
	_, err := container.VMCProcess(ctxt, gc.vmType, sir)
	return err
}

func (gc *GCManager) Close() {
	select {
	case gc.exitCh <- struct{}{}:
	default:
	}
	gcLogger.Infof("Stop GCManager")
}
