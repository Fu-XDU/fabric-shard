package types

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/pkg/errors"
)

type Org struct {
	Name               string
	AdminUser          string
	AdminClientContext contextAPI.ClientProvider
	ResMgmt            *resmgmt.Client
	MspClient          *mspclient.Client
	Peers              []*Peer
	Cache              map[string]*channel.Client
}

func NewOrg(name string, adminUser string) (o *Org) {
	o = &Org{
		Name:               name,
		AdminUser:          adminUser,
		AdminClientContext: nil,
		ResMgmt:            nil,
		MspClient:          nil,
		Peers:              []*Peer{},
		Cache:              map[string]*channel.Client{},
	}
	return
}

func (o *Org) NewClient(sdk *fabsdk.FabricSDK) (err error) {
	o.MspClient, err = mspclient.New(sdk.Context(), mspclient.WithOrg(o.Name))
	if err != nil {
		err = errors.Wrap(err, "failed to create org1MspClient")
	}
	return
}

func (o *Org) LoadPeers(sdk *fabsdk.FabricSDK) (err error) {
	o.Peers, err = loadOrgPeers(sdk.Context(fabsdk.WithUser(o.AdminUser), fabsdk.WithOrg(o.Name)), o.Name)
	return
}

func (o *Org) NewAdminClientCtx(sdk *fabsdk.FabricSDK) {
	o.AdminClientContext = sdk.Context(fabsdk.WithUser(o.AdminUser), fabsdk.WithOrg(o.Name))
}

// NewResmgmt New org resource management client
func (o *Org) NewResmgmt() (err error) {
	o.ResMgmt, err = resmgmt.New(o.AdminClientContext)
	return
}

func loadOrgPeers(ctxProvider contextAPI.ClientProvider, org string) (peers []*Peer, err error) {
	ctx, err := ctxProvider()
	if err != nil {
		err = errors.New(fmt.Sprintf("context creation failed: %s", err))
	}

	orgPeers, ok := ctx.EndpointConfig().PeersConfig(org)
	if !ok {
		err = errors.Errorf("ctx.EndpointConfig().PeersConfig(%s) failed！", org)
	}

	for _, peerConfig := range orgPeers {
		var peer fab.Peer
		peer, err = ctx.InfraProvider().CreatePeerFromConfig(&fab.NetworkPeer{PeerConfig: peerConfig})
		if err != nil {
			return
		}
		if _, ok = peerConfig.GRPCOptions["hostnameoverride"]; !ok {
			return nil, errors.Errorf("Need hostnameoverride")
		}
		hostname := peerConfig.GRPCOptions["hostnameoverride"].(string)
		p := NewPeer(hostname)
		p.Peer = &peer
		peers = append(peers, p)
	}
	return
}

func (o *Org) LookupPeer(hostname string) (peer *Peer) {
	for _, p := range o.Peers {
		if p.Name == hostname {
			return p
		}
	}
	return
}

func (o *Org) ChannelClient(sdk *fabsdk.FabricSDK, channelID string) (client *channel.Client, err error) {
	if c, ok := o.Cache[channelID]; ok {
		return c, nil
	}
	// Prepare channel client context using client context
	clientChannelContext := sdk.ChannelContext(channelID, fabsdk.WithUser(o.AdminUser), fabsdk.WithOrg(o.Name))
	// Channel client is used to query and execute transactions
	client, err = channel.New(clientChannelContext)
	o.Cache[channelID] = client
	return
}
