package types

type Channel struct {
	Name          string
	PeersHostname []string
	Peers         []*Peer
	ConfigPath    string
	AnchorPeers   []*anchorPeer
	BlockNum      uint64
}

func NewChannel(name string) (ch *Channel) {
	ch = &Channel{
		Name:          name,
		PeersHostname: []string{},
		AnchorPeers:   []*anchorPeer{},
		BlockNum:      uint64(0),
	}
	return
}
