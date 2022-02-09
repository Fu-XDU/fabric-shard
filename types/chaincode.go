package types

const (
	LIFECYCLE = "lifecycle"
	LSCC      = "lscc"
)

type Chaincode struct {
	Name      string
	Version   string
	ChannelID string
	GoPath    string
	CCPath    string
	Mode      string
}

func NewChaincode(name string, version string, channelID string, goPath string, ccPath string, mode string) (cc *Chaincode) {
	cc = &Chaincode{
		Name:      name,
		Version:   version,
		ChannelID: channelID,
		GoPath:    goPath,
		CCPath:    ccPath,
		Mode:      mode,
	}
	return
}
