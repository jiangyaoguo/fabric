/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package gc

import (
	"testing"
	"time"

	"github.com/hyperledger/fabric/core/container/ccintf"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/stretchr/testify/assert"
)

func TestAddVMGC(t *testing.T) {
	vmType := "MockVM"
	vmLiveTime := time.Duration(10 * time.Second)
	gcInterval := time.Duration(1 * time.Second)
	gc := NewGCManager(vmType, vmLiveTime, gcInterval)

	canName1 := "cc1:1.0"
	ccid1 := ccintf.CCID{
		ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "cc1"}},
		PeerID:        "testPeer",
		NetworkID:     "dev",
		Version:       "1.0",
	}
	canName2 := "cc2:1.0"
	ccid2 := ccintf.CCID{
		ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "cc2"}},
		PeerID:        "testPeer",
		NetworkID:     "dev",
		Version:       "1.0",
	}
	now := time.Now()

	gc.AddVM(canName1, ccid1, now)
	gc.AddVM(canName2, ccid2, now.Add(10*time.Second))

	time.Sleep(15 * time.Second)
	gc.Close() // close first in case data race

	expectVMs := map[string]*vmInfo{
		canName2: &vmInfo{ccid: ccid2, activeTime: now.Add(10 * time.Second)},
	}
	assert.Equal(t, len(gc.runningVMs), 1, "Should be left with only one vm")
	assert.Equal(t, expectVMs[canName2].ccid, gc.runningVMs[canName2].ccid, "Should be left with vm %s", canName2)
}

func TestUpdateVMGC(t *testing.T) {
	vmType := "MockVM"
	vmLiveTime := time.Duration(10 * time.Second)
	gcInterval := time.Duration(1 * time.Second)
	gc := NewGCManager(vmType, vmLiveTime, gcInterval)

	canName1 := "cc1:1.0"
	ccid1 := ccintf.CCID{
		ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "cc1"}},
		PeerID:        "testPeer",
		NetworkID:     "dev",
		Version:       "1.0",
	}
	canName2 := "cc2:1.0"
	ccid2 := ccintf.CCID{
		ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "cc2"}},
		PeerID:        "testPeer",
		NetworkID:     "dev",
		Version:       "1.0",
	}
	now := time.Now()

	gc.AddVM(canName1, ccid1, now)
	gc.AddVM(canName2, ccid2, now)

	var updateTime time.Time
	ch := make(chan int)
	go func() {
		time.Sleep(7 * time.Second)
		updateTime = time.Now()
		gc.UpdateVM(canName2, updateTime)
		t.Logf("Update vm %s at %v", canName2, updateTime)
		ch <- 1
	}()

	<-ch
	time.Sleep(5 * time.Second)
	gc.Close()

	expectVMs := map[string]*vmInfo{
		canName2: &vmInfo{ccid: ccid2, activeTime: updateTime},
	}
	assert.Equal(t, len(gc.runningVMs), 1, "Should be left with only one vm")
	assert.Equal(t, expectVMs[canName2].ccid, gc.runningVMs[canName2].ccid, "Should be left with vm %s", canName2)
}
