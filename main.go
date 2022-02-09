package main

import (
	"bytes"
	"fabric-sc/types"
	"fabric-sc/utils"
	"fmt"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	lcpackager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/lifecycle"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/pkg/errors"
	"log"
	"os"
	"sync"
	"time"
)

const (
	ordererEndpoint = "orderer.flxdu.cn"
	configPath      = "./config.yaml"
	goPath          = "/Users/fuming/go"

	period = 2 * time.Second
)

var (
	// SDK
	sdk    *fabsdk.FabricSDK
	appCtx *AppContext

	channelBeCreated = map[string]struct{}{}

	orgs = [][]string{{"Org1", "Admin"}, {"Org2", "Admin"}, {"Org3", "Admin"}, {"Org4", "Admin"}}

	chaincodes = []*types.Chaincode{
		types.NewChaincode("cc2", "1.0", "hello1", goPath, "cc2", types.LIFECYCLE),
		types.NewChaincode("cc1", "1.0", "hello2", goPath, "cc1", types.LIFECYCLE),
		types.NewChaincode("cc1", "1.0", "hello3", goPath, "cc1", types.LIFECYCLE),
		types.NewChaincode("cc1", "1.0", "hello4", goPath, "cc1", types.LIFECYCLE),
	}
)

func main() {
	err := setupSDK()
	if err != nil {
		log.Fatal(fmt.Sprintf("unable to setupNetwork SDK [%s]", err))
	}
	/*
		err = setupNetwork()
		if err != nil {
			log.Fatal(fmt.Sprintf("unable to setup fabric network [%s]", err))
		}
	*/
	defer teardown()
	utils.CleanupUserData(sdk)
	defer utils.CleanupUserData(sdk)
	err = ccTest()
	if err != nil {
		log.Fatal(fmt.Sprintf("ccTest error [%s]", err))
	}
	os.Exit(0)
}

func ccTest() (err error) {
	_, err = executeCC(appCtx.org[1], chaincodes[1], "setBalance", [][]byte{[]byte("peer0.org2"), []byte("0xffff")})
	if err != nil {
		return
	}
	_, err = executeCC(appCtx.org[1], chaincodes[1], "setBalance", [][]byte{[]byte("peer1.org2"), []byte("0xffff")})
	if err != nil {
		return
	}
	var pend sync.WaitGroup
	for i := 0; i < 50; i++ {
		pend.Add(2)
		go func() {
			_, err = executeCC(appCtx.org[1], chaincodes[1], "transfer", [][]byte{[]byte("peer0.org2"), []byte("peer1.org2"), []byte("0xf")})
			if err != nil {
				log.Println(err)
			}
			pend.Done()
			return
		}()
		go func() {
			_, err = executeCC(appCtx.org[1], chaincodes[1], "transfer", [][]byte{[]byte("peer1.org2"), []byte("peer0.org2"), []byte("0xe")})
			if err != nil {
				log.Println(err)
			}
			pend.Done()
			return
		}()
	}
	pend.Wait()
	log.Println("Done!")
	resp, err := queryCC(appCtx.org[1], chaincodes[1], "getBalance", [][]byte{[]byte("peer0.org2")})
	if err != nil {
		return
	}
	log.Println(string(resp.Payload))
	resp, err = queryCC(appCtx.org[1], chaincodes[1], "getBalance", [][]byte{[]byte("peer1.org2")})
	if err != nil {
		return
	}
	log.Println(string(resp.Payload))
	return
}

func executeCC(org *types.Org, cc *types.Chaincode, fcn string, args [][]byte) (resp channel.Response, err error) {
	client, err := org.ChannelClient(sdk, cc.ChannelID)
	if err != nil {
		return
	}
	resp, err = client.Execute(channel.Request{ChaincodeID: cc.Name, Fcn: fcn, Args: args}, channel.WithRetry(retry.DefaultChannelOpts))
	if err != nil {
		return
	}
	if resp.TxValidationCode != pb.TxValidationCode_VALID {
		err = errors.Errorf("chaincode execute tx validate failed, code: [%s]", resp.TxValidationCode.String())
		return
	}
	log.Printf("Execute chaincode [%s] done, channel: [%s], caller: [%s], func: [%s], args: %s, TxID: [%s]", cc.Name, cc.ChannelID, org.Name, fcn, args, resp.TransactionID)
	return
}

func queryCC(org *types.Org, cc *types.Chaincode, fcn string, args [][]byte) (resp channel.Response, err error) {
	client, err := org.ChannelClient(sdk, cc.ChannelID)
	if err != nil {
		return
	}
	resp, err = client.Query(channel.Request{ChaincodeID: cc.Name, Fcn: fcn, Args: args}, channel.WithRetry(retry.DefaultChannelOpts))
	if err != nil {
		return
	}
	if resp.TxValidationCode != pb.TxValidationCode_VALID {
		err = errors.Errorf("chaincode query tx validate failed, code: [%s]", resp.TxValidationCode.String())
		return
	}
	log.Printf("Query chaincode [%s] done, channel: [%s], caller: [%s], func: [%s], args: %s, TxID: [%s]", cc.Name, cc.ChannelID, org.Name, fcn, args, resp.TransactionID)
	return
}

func setupSDK() (err error) {
	// Create SDK setupNetwork
	sdk, err = fabsdk.New(config.FromFile(configPath))
	if err != nil {
		return errors.Wrap(err, "Failed to create new SDK")
	}

	appCtx = NewContext()
	appCtx.NewOrgs(orgs)

	appCtx.ordererClientContext = sdk.Context(fabsdk.WithUser("Admin"), fabsdk.WithOrg("OrdererOrg"))
	appCtx.ordererResMgmt, err = resmgmt.New(appCtx.ordererClientContext)
	appCtx.ordererEndpoint = ordererEndpoint

	if err != nil {
		return err
	}
	for _, o := range appCtx.org {
		err = o.NewClient(sdk)
		if err != nil {
			return err
		}
		err = o.LoadPeers(sdk)
		if err != nil {
			return err
		}
		o.NewAdminClientCtx(sdk)
		_, err = utils.DiscoverLocalPeers(o.AdminClientContext, len(o.Peers))
		if err != nil {
			return err
		}
		err = o.NewResmgmt()
		if err != nil {
			return err
		}
	}

	appCtx.channels, err = loadConfig()
	if err != nil {
		return errors.Wrap(err, "Failed to load config")
	}
	return
}

func setupNetwork() (err error) {
	var joined bool
	for _, o := range appCtx.org {
		for _, p := range o.Peers {
			for _, c := range p.Channel {
				joined, err = utils.IsJoinedChannel(c.Name, o.ResMgmt, p.Peer)
				if err != nil {
					return
				}
				if !joined {
					err = createAndJoinChannel(appCtx, c, o, p)
					if err != nil {
						return
					}
				}
			}
		}
	}
	err = setAnchorPeers(appCtx)
	if err != nil {
		return
	}
	for _, cc := range chaincodes {
		err = depolyCC(cc)
		if err != nil {
			return
		}
	}
	return
}

func depolyCC(cc *types.Chaincode) (err error) {
	ccSetup, err := appCtx.SetupCC(cc)
	if cc.Mode == types.LSCC {
		var ccPkg *resource.CCPackage
		ccPkg, err = packager.NewCCPackage(cc.CCPath, cc.GoPath)
		if err != nil {
			log.Fatal(err)
		}
		for _, t := range ccSetup.target {
			err = createCC(t.org, t.peers, cc, ccPkg, cc.Name, cc.Version)
		}
	} else {
		cc.CCPath = cc.GoPath + "/src/" + cc.CCPath
		label, ccPkg := packageCC(cc.Name, cc.Version, cc.CCPath)
		packageID := lcpackager.ComputePackageID(label, ccPkg)
		sequence := int64(1)
		upgrade := false

		for _, t := range ccSetup.target {
			log.Printf("安装链码[ %s ], 通道[ %s ], 组织[ %s ], 节点[ %s ]", cc.Name, cc.ChannelID, t.org.Name, types.PeersName(t.peers))
			fabPeers := types.Peers2FabPeers(t.peers)
			// Install cc
			installCC(label, ccPkg, t.org, fabPeers)
			log.Println("链码安装成功")
			time.Sleep(period)
			// Get installed cc package
			getInstalledCCPackage(packageID, ccPkg, t.org, fabPeers)
			log.Println("getInstalledCCPackage成功")
			time.Sleep(period)
			// Query installed cc
			queryInstalled(packageID, t.org, fabPeers)
			log.Println("queryInstalled成功")
			time.Sleep(period)
		}
		var allpeers []*types.Peer
		time.Sleep(period)
		for _, t := range ccSetup.target {
			log.Printf("确认链码[ %s ], 通道[ %s ], 组织[ %s ], 节点[ %s ]", cc.Name, cc.ChannelID, t.org.Name, types.PeersName(t.peers))
			fabPeers := types.Peers2FabPeers(t.peers)
			// Approve cc
			approveCC(packageID, cc, sequence, t.org, fabPeers)
			log.Println("approveCC成功")
			time.Sleep(period)
			// Query approve cc
			queryApprovedCC(cc, sequence, t.org, fabPeers)
			log.Println("queryApprovedCC成功")
			time.Sleep(period)
			// Check commit readiness
			checkCCCommitReadiness(cc, sequence, t.org, fabPeers)
			log.Println("checkCCCommitReadiness成功")
			time.Sleep(period)
			allpeers = append(allpeers, t.peers...)
		}
		commitOrg := ccSetup.target[0].org
		log.Printf("提交链码[ %s ], 通道[ %s ], 组织[ %s ], 节点[ %s ]", cc.Name, cc.ChannelID, commitOrg.Name, types.PeersName(allpeers))
		fabPeers := types.Peers2FabPeers(allpeers)
		// Commit cc
		commitCC(cc, sequence, commitOrg, fabPeers)
		log.Println("commitCC成功")
		time.Sleep(period)
		// Query committed cc
		queryCommittedCC(cc, sequence, commitOrg, fabPeers)
		log.Println("queryCommittedCC成功")
		time.Sleep(period)
		// Init cc
		initCC(cc, upgrade, sdk, commitOrg, fabPeers)
		log.Println("initCC成功")
		time.Sleep(period)
	}
	return
}

func createCC(org *types.Org, peers []*types.Peer, cc *types.Chaincode, ccPkg *resource.CCPackage, ccName, ccVersion string) (err error) {
	installCCReq := resmgmt.InstallCCRequest{Name: ccName, Path: cc.CCPath, Version: ccVersion, Package: ccPkg}
	fabPeers := types.Peers2FabPeers(peers)
	// Install example cc to Org peers
	response, err := org.ResMgmt.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithTargets(fabPeers...))
	if err != nil {
		err = errors.Wrapf(err, "InstallCC for [ %s ] failed", org.Name)
		return
	}
	for _, r := range response {
		log.Println(r.Target, r.Info)
	}
	// Ensure the CC is installed on all peers in both orgs
	installed := queryInstalledCC(org.Name, org.ResMgmt, ccName, ccVersion, fabPeers)
	if !installed {
		err = errors.Errorf("Expecting chaincode [%s:%s] to be installed on %v peers in %s", cc.Name, cc.Version, len(fabPeers), org.Name)
		return
	}
	return
}

func queryInstalledCC(orgID string, resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fab.Peer) bool {
	installed, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			ok := isCCInstalled(orgID, resMgmt, ccName, ccVersion, peers)
			if !ok {
				return &ok, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Chaincode [%s:%s] is not installed on all peers in Org1", ccName, ccVersion), nil)
			}
			return &ok, nil
		},
	)
	if err != nil {
		log.Fatal("Got error checking if chaincode was installed")
	}
	return *(installed).(*bool)
}

func isCCInstalled(orgID string, resMgmt *resmgmt.Client, ccName, ccVersion string, peers []fab.Peer) bool {
	log.Printf("Querying [%s] peers to see if chaincode [%s:%s] was installed", orgID, ccName, ccVersion)
	installedOnAllPeers := true
	for _, peer := range peers {
		log.Printf("Querying [%s] ...", peer.URL())
		resp, err := resMgmt.QueryInstalledChaincodes(resmgmt.WithTargets(peer))
		if err != nil {
			log.Fatalf("QueryInstalledChaincodes for peer [%s] failed", peer.URL())
		}
		found := false
		for _, ccInfo := range resp.Chaincodes {
			log.Printf("... found chaincode [%s:%s]", ccInfo.Name, ccInfo.Version)
			if ccInfo.Name == ccName && ccInfo.Version == ccVersion {
				found = true
				break
			}
		}
		if !found {
			log.Printf("... chaincode [%s:%s] is not installed on peer [%s]", ccName, ccVersion, peer.URL())
			installedOnAllPeers = false
		}
	}
	return installedOnAllPeers
}

func initCC(cc *types.Chaincode, upgrade bool, sdk *fabsdk.FabricSDK, org *types.Org, peers []fab.Peer) {
	// Prepare channel client context using client context
	clientChannelContext := sdk.ChannelContext(cc.ChannelID, fabsdk.WithUser(org.AdminUser), fabsdk.WithOrg(org.Name))
	// Channel client is used to query and execute transactions (Org1 is default org)
	client, err := channel.New(clientChannelContext)
	if err != nil {
		log.Fatalf("Failed to create new channel client: %s", err)
	}

	var args [][]byte
	if upgrade {
		args = [][]byte{[]byte("hello"), []byte("world")}
	} else {
		// TODO:只能有一个，后面要改
		args = [][]byte{[]byte("world")}
	}

	// init
	_, err = client.Execute(channel.Request{ChaincodeID: cc.Name, Fcn: "init", Args: args, IsInit: true},
		channel.WithRetry(retry.DefaultChannelOpts), channel.WithTargets(peers...)) //, channel.WithTargets(peers...)
	if err != nil {
		log.Fatalf("Failed to init: %s", err)
	}
}

func queryCommittedCC(cc *types.Chaincode, sequence int64, org *types.Org, peers []fab.Peer) {
	req := resmgmt.LifecycleQueryCommittedCCRequest{
		Name: cc.Name,
	}

	for _, p := range peers {
		resp, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
			func() (interface{}, error) {
				resp1, err := org.ResMgmt.LifecycleQueryCommittedCC(cc.ChannelID, req, resmgmt.WithTargets(p))
				if err != nil {
					return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("LifecycleQueryCommittedCC returned error: %v", err), nil)
				}
				flag := false
				for _, r := range resp1 {
					if r.Name == cc.Name && r.Sequence == sequence {
						flag = true
						break
					}
				}
				if !flag {
					return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("LifecycleQueryCommittedCC returned : %v", resp1), nil)
				}
				return resp1, err
			},
		)

		if err != nil {
			log.Fatal(err)
		}

		if resp == nil {
			log.Fatal("resp == nil")
		}
	}
}

func approveCC(packageID string, cc *types.Chaincode, sequence int64, org *types.Org, peers []fab.Peer) {
	//ccPolicy := policydsl.SignedByNOutOfGivenRole(1, mb.MSPRole_MEMBER, []string{org.Name + "MSP"})
	approveCCReq := resmgmt.LifecycleApproveCCRequest{
		Name:              cc.Name,
		Version:           cc.Version,
		PackageID:         packageID,
		Sequence:          sequence,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		//SignaturePolicy:   ccPolicy,
		InitRequired: true,
	}

	txnID, err := org.ResMgmt.LifecycleApproveCC(cc.ChannelID, approveCCReq, resmgmt.WithTargets(peers...), resmgmt.WithOrdererEndpoint(ordererEndpoint), resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Fatal("确认链码失败, err:", err)
	}
	if txnID == "" {
		log.Fatal("txnID empty")
	}
}

func commitCC(cc *types.Chaincode, sequence int64, org *types.Org, peers []fab.Peer) {
	//ccPolicy := policydsl.SignedByNOutOfGivenRole(1, mb.MSPRole_MEMBER, []string{org.Name + "MSP"})
	req := resmgmt.LifecycleCommitCCRequest{
		Name:              cc.Name,
		Version:           cc.Version,
		Sequence:          sequence,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		//SignaturePolicy:   ccPolicy,
		InitRequired: true,
	}
	txnID, err := org.ResMgmt.LifecycleCommitCC(cc.ChannelID, req, resmgmt.WithTargets(peers...), resmgmt.WithOrdererEndpoint(ordererEndpoint), resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		log.Fatal(err)
	}
	if txnID == "" {
		log.Fatal("txnID empty")
	}
}

func checkCCCommitReadiness(cc *types.Chaincode, sequence int64, org *types.Org, peers []fab.Peer) {
	//ccPolicy := policydsl.SignedByNOutOfGivenRole(1, mb.MSPRole_MEMBER, []string{org.Name + "MSP"})
	req := resmgmt.LifecycleCheckCCCommitReadinessRequest{
		Name:              cc.Name,
		Version:           cc.Version,
		EndorsementPlugin: "escc",
		ValidationPlugin:  "vscc",
		//SignaturePolicy:   ccPolicy,
		Sequence:     sequence,
		InitRequired: true,
	}

	for _, p := range peers {
		resp, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
			func() (interface{}, error) {
				resp1, err := org.ResMgmt.LifecycleCheckCCCommitReadiness(cc.ChannelID, req, resmgmt.WithTargets(p))
				//fmt.Printf("LifecycleCheckCCCommitReadiness cc = %v, = %v, peer = %v \n", cc.Name, resp1,p.URL())
				if err != nil {
					return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("LifecycleCheckCCCommitReadiness returned error: %v", err), nil)
				}
				flag := true
				for _, r := range resp1.Approvals {
					flag = flag || r
				}
				if !flag {
					return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("LifecycleCheckCCCommitReadiness returned : %v", resp1), nil)
				}
				return resp1, err
			},
		)
		if err != nil {
			log.Fatal(err)
		}
		if resp == nil {
			log.Fatal("resp == nil")
		}
	}
}

func queryApprovedCC(cc *types.Chaincode, sequence int64, org *types.Org, peers []fab.Peer) {
	// Query approve cc
	queryApprovedCCReq := resmgmt.LifecycleQueryApprovedCCRequest{
		Name:     cc.Name,
		Sequence: sequence,
	}
	for _, p := range peers {
		resp, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
			func() (interface{}, error) {
				resp1, err := org.ResMgmt.LifecycleQueryApprovedCC(cc.ChannelID, queryApprovedCCReq, resmgmt.WithTargets(p))
				if err != nil {
					return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("LifecycleQueryApprovedCC returned error: %v", err), nil)
				}
				return resp1, err
			},
		)
		if err != nil {
			log.Fatal(err)
		}
		if resp == nil {
			log.Fatal("resp == nil")
		}
	}
}

func queryInstalled(packageID string, org *types.Org, peers []fab.Peer) {
	resp1, err := org.ResMgmt.LifecycleQueryInstalledCC(resmgmt.WithTargets([]fab.Peer{peers[0]}...))
	if err != nil {
		log.Fatal(err)
	}
	packageID1 := ""
	for _, t := range resp1 {
		if t.PackageID == packageID {
			packageID1 = t.PackageID
		}
	}
	if packageID != packageID1 {
		log.Fatal("packageID != packageID1")
	}
}

func getInstalledCCPackage(packageID string, ccPkg []byte, org *types.Org, peers []fab.Peer) {
	resp1, err := org.ResMgmt.LifecycleGetInstalledCCPackage(packageID, resmgmt.WithTargets([]fab.Peer{peers[0]}...))
	if err != nil {
		log.Fatal(err)
	}
	if !bytes.Equal(ccPkg, resp1) {
		log.Fatal("!bytes.Equal(ccPkg, resp1)")
	}
}

func installCC(label string, ccPkg []byte, org *types.Org, peers []fab.Peer) {
	installCCReq := resmgmt.LifecycleInstallCCRequest{
		Label:   label,
		Package: ccPkg,
	}

	packageID := lcpackager.ComputePackageID(installCCReq.Label, installCCReq.Package)

	if !checkInstalled(packageID, peers[0], org.ResMgmt) {
		resp, err := org.ResMgmt.LifecycleInstallCC(installCCReq, resmgmt.WithTargets(peers...), resmgmt.WithRetry(retry.DefaultResMgmtOpts))
		if err != nil {
			log.Fatal(err)
		}
		if packageID != resp[0].PackageID {
			log.Fatal("packageID != resp[0].PackageID")
		}
	}
}

func checkInstalled(packageID string, peer fab.Peer, client *resmgmt.Client) bool {
	flag := false
	resp1, err := client.LifecycleQueryInstalledCC(resmgmt.WithTargets(peer))
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range resp1 {
		if t.PackageID == packageID {
			flag = true
		}
	}
	return flag
}

func packageCC(ccName, ccVersion, ccPath string) (string, []byte) {
	label := ccName + "_" + ccVersion

	desc := &lcpackager.Descriptor{
		Path:  ccPath,
		Type:  pb.ChaincodeSpec_GOLANG,
		Label: label,
	}
	ccPkg, err := lcpackager.NewCCPackage(desc)
	if err != nil {
		log.Fatal(err)
	}
	return desc.Label, ccPkg
}

func loadConfig() (channels []*types.Channel, err error) {
	conf, err := sdk.Config()
	if err != nil {
		return
	}
	data, ok := conf.Lookup("channels")
	if !ok {
		return
	}

	for k, v := range data.(map[string]interface{}) {
		ch := types.NewChannel(k)
		if _, ok = v.(map[string]interface{})["configpath"]; !ok {
			log.Fatalf("channel %s configpath required", ch.Name)
		}
		ch.ConfigPath = v.(map[string]interface{})["configpath"].(string)

		if _, ok = v.(map[string]interface{})["peers"]; ok {
			peers := v.(map[string]interface{})["peers"].(map[string]interface{})
			for name, c := range peers {
				ch.PeersHostname = append(ch.PeersHostname, name)
				if fabPeer, _ := appCtx.LookupPeer(name); fabPeer != nil {
					ch.Peers = append(ch.Peers, fabPeer)
					fabPeer.Channel = append(fabPeer.Channel, ch)
				}

				peerConf := c.(map[string]interface{})
				if _, ok = peerConf["anchorpeer"]; ok {
					if _, ok = peerConf["txpath"]; ok {
						if peerConf["anchorpeer"].(bool) {
							ch.AnchorPeers = append(ch.AnchorPeers, types.NewAnchorPeer(name, peerConf["txpath"].(string)))
						}
					} else {
						log.Printf("Peer %s is anchor peer, txpath required", name)
					}
				}
			}
		} else {
			log.Printf("channel %s peers is nil", ch.Name)
		}
		channels = append(channels, ch)
	}
	return
}

func createAndJoinChannel(ctx *AppContext, channel *types.Channel, org *types.Org, peer *types.Peer) error {
	// Get signing identity that is used to sign create channel request
	if _, ok := channelBeCreated[channel.Name]; !ok {
		adminUserIdentity, err := org.MspClient.GetSigningIdentity(org.AdminUser)
		if err != nil {
			return errors.Wrapf(err, "failed to get %sAdminUser", org.Name)
		}
		signingIdentity := []msp.SigningIdentity{adminUserIdentity}
		channel.BlockNum, err = createChannel(signingIdentity, ctx.ordererClientContext, true, channel.BlockNum, org, channel)
		if err != nil {
			return err
		}
		log.Printf("创建通道:[ %s ]", channel.Name)
		channelBeCreated[channel.Name] = struct{}{}
	}
	// Join channel
	err := org.ResMgmt.JoinChannel(channel.Name, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithTargets([]fab.Peer{*peer.Peer}...), resmgmt.WithOrdererEndpoint(ctx.ordererEndpoint))
	if err != nil {
		return err
	}
	log.Printf("组织%s,节点%s加入通道:[ %s ]", org.Name, peer.Name, channel.Name)
	return nil
}

func setAnchorPeers(ctx *AppContext) error {
	for _, c := range ctx.channels {
		for _, wantAnchorPeers := range c.AnchorPeers {
			isAnchor := false
			_, orgIndex := ctx.LookupPeer(wantAnchorPeers.Name)
			peerOrg := ctx.org[orgIndex]
			channelCfg, err := peerOrg.ResMgmt.QueryConfigFromOrderer(c.Name, resmgmt.WithOrdererEndpoint(ordererEndpoint))
			if err != nil {
				return err
			}
			anchorPeers := channelCfg.AnchorPeers()
			for _, p := range anchorPeers {
				if wantAnchorPeers.Name == p.Host {
					isAnchor = true
					break
				}
			}

			// Log channel anchor peers
			var haveAnchorPeers []string
			for _, p := range anchorPeers {
				haveAnchorPeers = append(haveAnchorPeers, p.Host)
			}
			log.Printf("Channel [ %s ] anchorPeers : %v", c.Name, haveAnchorPeers)

			// set anchor
			if !isAnchor {
				ch := types.NewChannel(c.Name)
				ch.ConfigPath = wantAnchorPeers.TxPath
				adminUserIdentity, err := peerOrg.MspClient.GetSigningIdentity(peerOrg.AdminUser)
				if err != nil {
					return errors.Wrapf(err, "failed to get %sAdminUser", peerOrg.Name)
				}
				signingIdentity := []msp.SigningIdentity{adminUserIdentity}
				ch.BlockNum, err = createChannel(signingIdentity, peerOrg.AdminClientContext, false, ch.BlockNum, peerOrg, ch)
				if err != nil {
					return err
				}
				log.Printf("Set peer %s as anchor peer at channel %s...", wantAnchorPeers.Name, c.Name)
			}

			channelCfg, err = peerOrg.ResMgmt.QueryConfigFromOrderer(c.Name, resmgmt.WithOrdererEndpoint(ordererEndpoint))
			if err != nil {
				return err
			}
			anchorPeers = channelCfg.AnchorPeers()
			// Log channel anchor peers
			var haveAnchorPeers2 []string
			for _, p := range anchorPeers {
				haveAnchorPeers2 = append(haveAnchorPeers2, p.Host)
			}
			log.Printf("Channel [ %s ] anchorPeers : %v", c.Name, haveAnchorPeers2)
		}
	}
	return nil
}

func createChannel(signingIdentities []msp.SigningIdentity, clientContext contextAPI.ClientProvider, genesis bool, lastConfigBlock uint64, org *types.Org, ch *types.Channel) (blockNum uint64, err error) {
	// do the same get ch client and create channel for each anchor peer as well (first for Org1MSP)
	chMgmtClient, err := resmgmt.New(clientContext)
	if err != nil {
		err = errors.Wrap(err, "Failed to get a new channel management client")
		return
	}

	configQueryClient, err := resmgmt.New(org.AdminClientContext)
	if err != nil {
		err = errors.Wrap(err, "Failed to get a new channel management client")
		return
	}

	// create a channel
	req := resmgmt.SaveChannelRequest{ChannelID: ch.Name,
		ChannelConfigPath: ch.ConfigPath,
		SigningIdentities: signingIdentities,
	}
	_, err = chMgmtClient.SaveChannel(req, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint(ordererEndpoint))
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("error should be nil for SaveChannel of %s", ch.Name))
		return
	}

	blockNum, err = WaitForOrdererConfigUpdate(configQueryClient, ch.Name, genesis, lastConfigBlock)
	if err != nil {
		err = errors.Wrap(err, "error should be nil for SaveChannel for anchor peer 1")
		return
	}
	return
}

// WaitForOrdererConfigUpdate waits until the config block update has been committed.
// In Fabric 1.0 there is a bug that panics the orderer if more than one config update is added to the same block.
// This function may be invoked after each config update as a workaround.
func WaitForOrdererConfigUpdate(client *resmgmt.Client, channelID string, genesis bool, lastConfigBlock uint64) (uint64, error) {
	blockNum, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			chConfig, err := client.QueryConfigFromOrderer(channelID, resmgmt.WithOrdererEndpoint(ordererEndpoint))
			if err != nil {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), err.Error(), nil)
			}

			currentBlock := chConfig.BlockNumber()
			if currentBlock <= lastConfigBlock && !genesis {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Block number was not incremented [%d, %d]", currentBlock, lastConfigBlock), nil)
			}

			block, err := client.QueryConfigBlockFromOrderer(channelID, resmgmt.WithOrdererEndpoint(ordererEndpoint))
			if err != nil {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), err.Error(), nil)
			}
			if block.Header.Number != currentBlock {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Invalid block number [%d, %d]", block.Header.Number, currentBlock), nil)
			}

			return &currentBlock, nil
		},
	)
	return *blockNum.(*uint64), err
}

func teardown() {
	if sdk != nil {
		sdk.Close()
	}
}
