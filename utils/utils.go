package utils

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	contextAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	fabAPI "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	contextImpl "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/msp"
	"github.com/pkg/errors"
	"log"
	"os"
)

// CleanupUserData removes user data.
func CleanupUserData(sdk *fabsdk.FabricSDK) {
	var keyStorePath, credentialStorePath string

	configBackend, err := sdk.Config()
	if err != nil {
		// if an error is returned from Config, it means configBackend was nil, in this case simply hard code
		// the keyStorePath and credentialStorePath to the default values
		// This case is mostly happening due to configless test that is not passing a ConfigProvider to the SDK
		// which makes configBackend = nil.
		// Since configless test uses the same config values as the default ones (config_test.yaml), it's safe to
		// hard code these paths here
		keyStorePath = "/tmp/msp/keystore"
		credentialStorePath = "/tmp/state-store"
	} else {
		cryptoSuiteConfig := cryptosuite.ConfigFromBackend(configBackend)
		identityConfig, err := msp.ConfigFromBackend(configBackend)
		if err != nil {
			log.Fatal(err)
		}

		keyStorePath = cryptoSuiteConfig.KeyStorePath()
		credentialStorePath = identityConfig.CredentialStorePath()
	}

	CleanupTestPath(keyStorePath)
	CleanupTestPath(credentialStorePath)
}

// CleanupTestPath removes the contents of a state store.
func CleanupTestPath(storePath string) {
	err := os.RemoveAll(storePath)
	if err != nil {
		log.Fatalf("Cleaning up directory '%s' failed: %v", storePath, err)
	}
}

// DiscoverLocalPeers queries the local peers for the given MSP context and returns all of the peers. If
// the number of peers does not match the expected number then an error is returned.
func DiscoverLocalPeers(ctxProvider contextAPI.ClientProvider, expectedPeers int) ([]fabAPI.Peer, error) {
	ctx, err := contextImpl.NewLocal(ctxProvider)
	if err != nil {
		return nil, errors.Wrap(err, "error creating local context")
	}

	discoveredPeers, err := retry.NewInvoker(retry.New(retry.TestRetryOpts)).Invoke(
		func() (interface{}, error) {
			peers, serviceErr := ctx.LocalDiscoveryService().GetPeers()
			if serviceErr != nil {
				return nil, errors.Wrapf(serviceErr, "error getting peers for MSP [%s]", ctx.Identifier().MSPID)
			}
			if len(peers) < expectedPeers {
				return nil, status.New(status.TestStatus, status.GenericTransient.ToInt32(), fmt.Sprintf("Expecting %d peers but got %d", expectedPeers, len(peers)), nil)
			}
			return peers, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return discoveredPeers.([]fabAPI.Peer), nil
}

// IsJoinedChannel returns true if the given peer has joined the given channel
func IsJoinedChannel(channelID string, resMgmtClient *resmgmt.Client, peer *fabAPI.Peer) (bool, error) {
	resp, err := resMgmtClient.QueryChannels(resmgmt.WithTargets(*peer))
	if err != nil {
		return false, err
	}
	for _, chInfo := range resp.Channels {
		if chInfo.ChannelId == channelID {
			return true, nil
		}
	}
	return false, nil
}