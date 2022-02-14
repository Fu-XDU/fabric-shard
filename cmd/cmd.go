package cmd

import (
	"fabric-sc/types"
	"fabric-sc/utils"
	"fmt"
	"log"
	"strings"
)

func formatArgs(args [][]byte) (res string) {
	for _, a := range args {
		res += "\"" + string(a) + "\", "
	}
	res = "[" + strings.TrimRight(res, ", ") + "]"
	return
}

func ExecuteCC(orgIndex int, peer *types.Peer, cc *types.Chaincode, args [][]byte) string {
	f := true
	for f {
		format := "docker exec -e CORE_PEER_MSPCONFIGPATH=%s -e CORE_PEER_LOCALMSPID=%s -e CORE_PEER_ADDRESS=%s cli " +
			"peer chaincode invoke -o %s -C %s -n %s -c '{\"Args\":%s}'"
		cmd := fmt.Sprintf(format, corePeerMSPConfigPath(orgIndex), corePeerLocalMSPID(orgIndex), corePeerAddress(peer.Name), ordererAddress(), cc.ChannelID, cc.Name, formatArgs(args))
		res, err := utils.ExecShell(cmd)
		if err != nil {
			log.Println(err)
			continue
		}
		f = false
		i := strings.LastIndex(res, "chaincodeInvokeOrQuery")
		if i < 0 {
			i = 0
		}
		r := res[i:][strings.Index(res[i:], "->")+3:]
		log.Println(r)
		return r
	}
	return ""
}

func corePeerMSPConfigPath(orgIndex int) string {
	return fmt.Sprintf("/opt/crypto-config/peerOrganizations/org%v.flxdu.cn/users/Admin@org%v.flxdu.cn/msp", orgIndex, orgIndex)
}

func corePeerLocalMSPID(orgIndex int) string {
	return fmt.Sprintf("Org%vMSP", orgIndex)
}

func corePeerAddress(peerName string) string {
	return fmt.Sprintf("%v:7051", peerName)
}

func ordererAddress() string {
	return "orderer.flxdu.cn:7050"
}
