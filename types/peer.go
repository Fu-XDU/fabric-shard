package types

import "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"

type Peer struct {
	Name    string
	Peer    *fab.Peer
	Channel []*Channel
}

type anchorPeer struct {
	Name   string
	TxPath string
}

func NewPeer(name string) (p *Peer) {
	p = &Peer{
		Name:    name,
		Peer:    nil,
		Channel: []*Channel{},
	}
	return
}

func NewAnchorPeer(name string, txPath string) (ap *anchorPeer) {
	ap = &anchorPeer{
		Name:   name,
		TxPath: txPath,
	}
	return
}

func Peers2FabPeers(peers []*Peer) (fabPeers []fab.Peer) {
	for _, p := range peers {
		fabPeers = append(fabPeers, *p.Peer)
	}
	return
}

func PeersName(peers []*Peer) string {
	name := ""
	for _, p := range peers {
		name += p.Name + " "
	}
	return name
}
