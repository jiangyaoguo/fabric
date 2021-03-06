/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package chaincode_test

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/chaincode"
	"github.com/hyperledger/fabric/core/chaincode/mock"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	pb "github.com/hyperledger/fabric/protos/peer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var _ = Describe("Launcher", func() {
	var (
		fakeRuntime         *mock.Runtime
		fakeRegistry        *mock.LaunchRegistry
		fakeExecutor        *mock.Executor
		fakePackageProvider *mock.PackageProvider
		fakePackage         *mock.CCPackage
		ready               chan (struct{})

		cccid          *ccprovider.CCContext
		signedProp     *pb.SignedProposal
		proposal       *pb.Proposal
		chaincodeID    *pb.ChaincodeID
		deploymentSpec *pb.ChaincodeDeploymentSpec

		launcher *chaincode.Launcher
	)

	BeforeEach(func() {
		signedProp = &pb.SignedProposal{ProposalBytes: []byte("some-proposal-bytes")}
		proposal = &pb.Proposal{Payload: []byte("some-payload-bytes")}
		cccid = ccprovider.NewCCContext("chain-id", "context-name", "context-version", "tx-id", false, signedProp, proposal)
		chaincodeID = &pb.ChaincodeID{Name: "chaincode-name", Version: "chaincode-version"}
		deploymentSpec = &pb.ChaincodeDeploymentSpec{
			CodePackage:   []byte("code-package"),
			ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: chaincodeID},
		}
		deploymentSpecPayload, err := proto.Marshal(deploymentSpec)
		Expect(err).NotTo(HaveOccurred())

		ready = make(chan struct{})
		fakeRegistry = &mock.LaunchRegistry{}
		fakeRegistry.LaunchingReturns(ready, nil)

		fakeRuntime = &mock.Runtime{}
		fakeRuntime.StartStub = func(context.Context, *ccprovider.CCContext, *pb.ChaincodeDeploymentSpec) error {
			close(ready)
			return nil
		}

		fakePackage = &mock.CCPackage{}
		fakePackage.GetDepSpecReturns(deploymentSpec)
		fakePackageProvider = &mock.PackageProvider{}
		fakePackageProvider.GetChaincodeReturns(fakePackage, nil)

		cdsResponse := &pb.Response{
			Status:  shim.OK,
			Payload: deploymentSpecPayload,
		}
		fakeExecutor = &mock.Executor{}
		fakeExecutor.ExecuteChaincodeReturns(cdsResponse, nil, nil)
		lifecycle := &chaincode.Lifecycle{
			Executor: fakeExecutor,
		}

		launcher = &chaincode.Launcher{
			Runtime:         fakeRuntime,
			PackageProvider: fakePackageProvider,
			Registry:        fakeRegistry,
			Lifecycle:       lifecycle,
			StartupTimeout:  5 * time.Second,
		}
	})

	Context("when launch is provided with an invocation spec", func() {
		var invocationSpec *pb.ChaincodeInvocationSpec

		BeforeEach(func() {
			invocationSpec = &pb.ChaincodeInvocationSpec{
				ChaincodeSpec: &pb.ChaincodeSpec{ChaincodeId: chaincodeID},
			}
		})

		It("gets the deployment spec", func() {
			err := launcher.Launch(context.Background(), cccid, invocationSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeExecutor.ExecuteChaincodeCallCount()).To(Equal(1))
			ctx, cccid, args := fakeExecutor.ExecuteChaincodeArgsForCall(0)
			Expect(ctx).To(Equal(context.Background()))
			Expect(cccid).To(Equal(ccprovider.NewCCContext("chain-id", "lscc", "latest", "tx-id", true, signedProp, proposal)))
			Expect(args).To(Equal(util.ToChaincodeArgs("getdepspec", "chain-id", "chaincode-name")))
		})

		It("uses the deployment spec when starting the runtime", func() {
			err := launcher.Launch(context.Background(), cccid, invocationSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRuntime.StartCallCount()).To(Equal(1))
			ctx, ccCtx, cds := fakeRuntime.StartArgsForCall(0)
			Expect(ctx).To(Equal(context.Background()))
			Expect(ccCtx).To(Equal(cccid))
			Expect(cds).To(Equal(deploymentSpec))
		})

		Context("when getting the deployment spec fails", func() {
			BeforeEach(func() {
				fakeExecutor.ExecuteChaincodeReturns(nil, nil, errors.New("king-kong"))
			})

			It("returns a wrapped error", func() {
				err := launcher.Launch(context.Background(), cccid, invocationSpec)
				Expect(err).To(MatchError(MatchRegexp("failed to get deployment spec for context-name:context-version:.*king-kong")))
			})
		})

		Context("when the returned deployment spec has a nil chaincode package", func() {
			BeforeEach(func() {
				deploymentSpec.CodePackage = nil
				deploymentSpecPayload, err := proto.Marshal(deploymentSpec)
				Expect(err).NotTo(HaveOccurred())

				cdsResponse := &pb.Response{
					Status:  shim.OK,
					Payload: deploymentSpecPayload,
				}
				fakeExecutor.ExecuteChaincodeReturns(cdsResponse, nil, nil)
			})

			It("gets the package from the package provider", func() {
				err := launcher.Launch(context.Background(), cccid, invocationSpec)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePackageProvider.GetChaincodeCallCount()).To(Equal(1))
				name, version := fakePackageProvider.GetChaincodeArgsForCall(0)
				Expect(name).To(Equal("chaincode-name"))
				Expect(version).To(Equal("chaincode-version"))
			})

			Context("when getting the package fails", func() {
				BeforeEach(func() {
					fakePackageProvider.GetChaincodeReturns(nil, errors.New("tangerine"))
				})

				It("returns a wrapped error", func() {
					err := launcher.Launch(context.Background(), cccid, deploymentSpec)
					Expect(err).To(MatchError("failed to get chaincode package: tangerine"))
				})
			})
		})

		Context("when launching a system chaincode", func() {
			BeforeEach(func() {
				cccid = ccprovider.NewCCContext("chain-id", "lscc", "latest", "tx-id", true, signedProp, proposal)
			})

			It("returns an error", func() {
				err := launcher.Launch(context.Background(), cccid, invocationSpec)
				Expect(err).To(MatchError("a syscc should be running (it cannot be launched) lscc:latest"))
			})
		})
	})

	Context("when launch is provided with a deployment spec", func() {
		BeforeEach(func() {
			fakePackage.GetDepSpecReturns(deploymentSpec)
			fakePackageProvider.GetChaincodeReturns(fakePackage, nil)
		})

		It("does not get the deployment spec from lifecycle", func() {
			err := launcher.Launch(context.Background(), cccid, deploymentSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeExecutor.ExecuteChaincodeCallCount()).To(Equal(0))
		})

		Context("when the deployment spec is missing a chaincode package", func() {
			BeforeEach(func() {
				deploymentSpec.CodePackage = nil
				deploymentSpecPayload, err := proto.Marshal(deploymentSpec)
				Expect(err).NotTo(HaveOccurred())

				cdsResponse := &pb.Response{
					Status:  shim.OK,
					Payload: deploymentSpecPayload,
				}
				fakeExecutor.ExecuteChaincodeReturns(cdsResponse, nil, nil)
			})

			It("gets the package from the package provider", func() {
				err := launcher.Launch(context.Background(), cccid, deploymentSpec)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakePackageProvider.GetChaincodeCallCount()).To(Equal(1))
				name, version := fakePackageProvider.GetChaincodeArgsForCall(0)
				Expect(name).To(Equal("chaincode-name"))
				Expect(version).To(Equal("chaincode-version"))
			})

			Context("when getting the package fails", func() {
				BeforeEach(func() {
					fakePackageProvider.GetChaincodeReturns(nil, errors.New("tangerine"))
				})

				It("returns a wrapped error", func() {
					err := launcher.Launch(context.Background(), cccid, deploymentSpec)
					Expect(err).To(MatchError("failed to get chaincode package: tangerine"))
				})
			})
		})
	})

	It("registers the chaincode as launching", func() {
		err := launcher.Launch(context.Background(), cccid, deploymentSpec)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.LaunchingCallCount()).To(Equal(1))
		cname := fakeRegistry.LaunchingArgsForCall(0)
		Expect(cname).To(Equal("context-name:context-version"))
	})

	It("starts the runtime for the chaincode", func() {
		err := launcher.Launch(context.Background(), cccid, deploymentSpec)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRuntime.StartCallCount()).To(Equal(1))
		ctx, ccCtx, cds := fakeRuntime.StartArgsForCall(0)
		Expect(ctx).To(Equal(context.Background()))
		Expect(ccCtx).To(Equal(cccid))
		Expect(cds).To(Equal(deploymentSpec))
	})

	It("waits for ready to close", func() {
		fakeRuntime.StartReturns(nil)

		errCh := make(chan error, 1)
		go func() { errCh <- launcher.Launch(context.Background(), cccid, deploymentSpec) }()

		Consistently(errCh).ShouldNot(Receive())
		close(ready)
		Eventually(errCh).Should(Receive(BeNil()))
	})

	It("does not deregister the chaincode", func() {
		err := launcher.Launch(context.Background(), cccid, deploymentSpec)
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeRegistry.DeregisterCallCount()).To(Equal(0))
	})

	Context("when launch registration fails", func() {
		BeforeEach(func() {
			fakeRegistry.LaunchingReturns(nil, errors.New("gargoyle"))
		})

		It("returns an error", func() {
			err := launcher.Launch(context.Background(), cccid, deploymentSpec)
			Expect(err).To(MatchError("failed to register context-name:context-version as launching: gargoyle"))
		})
	})

	Context("when starting the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("banana"))
		})

		It("returns a wrapped error", func() {
			err := launcher.Launch(context.Background(), cccid, deploymentSpec)
			Expect(err).To(MatchError("error starting container: banana"))
		})

		It("stops the runtime", func() {
			launcher.Launch(context.Background(), cccid, deploymentSpec)

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ctx, ccContext, cds := fakeRuntime.StopArgsForCall(0)
			Expect(ctx).To(Equal(context.Background()))
			Expect(ccContext).To(Equal(cccid))
			Expect(cds).To(Equal(deploymentSpec))
		})

		It("deregisters the chaincode", func() {
			launcher.Launch(context.Background(), cccid, deploymentSpec)

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("context-name:context-version"))
		})
	})

	Context("when the runtime startup times out", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(nil)
			launcher.StartupTimeout = 250 * time.Millisecond
		})

		It("returns a meaningful error", func() {
			err := launcher.Launch(context.Background(), cccid, deploymentSpec)
			Expect(err).To(MatchError("timeout expired while starting chaincode context-name:context-version for transaction tx-id"))
		})

		It("stops the runtime", func() {
			launcher.Launch(context.Background(), cccid, deploymentSpec)

			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
			ctx, ccContext, cds := fakeRuntime.StopArgsForCall(0)
			Expect(ctx).To(Equal(context.Background()))
			Expect(ccContext).To(Equal(cccid))
			Expect(cds).To(Equal(deploymentSpec))
		})

		It("deregisters the chaincode", func() {
			launcher.Launch(context.Background(), cccid, deploymentSpec)

			Expect(fakeRegistry.DeregisterCallCount()).To(Equal(1))
			cname := fakeRegistry.DeregisterArgsForCall(0)
			Expect(cname).To(Equal("context-name:context-version"))
		})
	})

	Context("when stopping the runtime fails", func() {
		BeforeEach(func() {
			fakeRuntime.StartReturns(errors.New("whirled-peas"))
			fakeRuntime.StopReturns(errors.New("applesauce"))
		})

		It("preserves the initial error", func() {
			err := launcher.Launch(context.Background(), cccid, deploymentSpec)
			Expect(err).To(MatchError("error starting container: whirled-peas"))
			Expect(fakeRuntime.StopCallCount()).To(Equal(1))
		})
	})
})
