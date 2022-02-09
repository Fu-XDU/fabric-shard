package main

import (
	"fabric-sc/types"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/pkg/errors"
)

// AppContext used to create context for different tests in the orgs package
type AppContext struct {
	// client contexts
	ordererClientContext contextAPI.ClientProvider
	ordererResMgmt       *resmgmt.Client
	ordererEndpoint      string
	org                  []*types.Org
	chaincode            []*types.Chaincode
	channels             []*types.Channel
	blockNum             uint64
}

func NewContext() (ctx *AppContext) {
	ctx = &AppContext{
		ordererClientContext: nil,
		org:                  []*types.Org{},
		chaincode:            []*types.Chaincode{},
		blockNum:             uint64(0),
	}
	return
}

func (ctx *AppContext) NewOrg(name string, adminUser string) {
	ctx.org = append(ctx.org, types.NewOrg(name, adminUser))
}

func (ctx *AppContext) NewOrgs(args [][]string) {
	for _, arg := range args {
		ctx.org = append(ctx.org, types.NewOrg(arg[0], arg[1]))
	}
}

func (ctx *AppContext) LookupPeer(hostname string) (peer *types.Peer, orgIndex int) {
	for i, o := range ctx.org {
		p := o.LookupPeer(hostname)
		if p != nil {
			return p, i
		}
	}
	return nil, -1
}

func (ctx *AppContext) LookupChannel(name string) (channel *types.Channel, index int) {
	for i, ch := range ctx.channels {
		if ch.Name == name {
			return ch, i
		}
	}
	return nil, -1
}

type ChaincodeSetup struct {
	chaincode *types.Chaincode
	target    []*target
}

type target struct {
	org   *types.Org
	peers []*types.Peer
}

func (ctx *AppContext) SetupCC(cc *types.Chaincode) (ccSetup *ChaincodeSetup, err error) {
	ch, i := appCtx.LookupChannel(cc.ChannelID)
	if i == -1 {
		err = errors.Errorf("can not find channel [ %s ] when setup chaincode %v", cc.ChannelID, *cc)
		return
	}

	org := make(map[string]*target)
	for _, peer := range ch.Peers {
		_, orgIndex := appCtx.LookupPeer(peer.Name)
		_, ok := org[appCtx.org[orgIndex].Name]
		if ok {
			org[appCtx.org[orgIndex].Name].peers = append(org[appCtx.org[orgIndex].Name].peers, peer)
		} else {
			org[appCtx.org[orgIndex].Name] = &target{
				org:   appCtx.org[orgIndex],
				peers: []*types.Peer{peer},
			}
		}
	}
	ccSetup = &ChaincodeSetup{}
	ccSetup.target = make([]*target, 0)
	for _, t := range org {
		ccSetup.target = append(ccSetup.target, t)
	}
	ccSetup.chaincode = cc
	return
}
