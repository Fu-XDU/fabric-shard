package main

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/util"
	"strconv"
)

// SmartContract is the data structure which represents this contract and on which
// various contract lifecycle functions are attached
type SmartContract struct {
}

type Res struct {
	Result    string `json:"result"`
	TxID      string `json:"tx_id"`
	Timestamp string `json:"timestamp"`
}

// Init is called during chaincode instantiation to initialize any
// data. Note that chaincode upgrade also calls this function to reset
// or to migrate data.
func (s *SmartContract) Init(stub shim.ChaincodeStubInterface) peer.Response {
	// Get the args from the transaction proposal
	args := stub.GetStringArgs()
	if len(args) != 2 {
		return shim.Error("Incorrect arguments. Expecting a key and a value")
	}

	// Set up any variables or assets here by calling stub.PutState()

	// We store the key and the value on the ledger
	err := stub.PutState(args[0], []byte(args[1]))
	if err != nil {
		return shim.Error(fmt.Sprintf("Failed to create asset: %s", args[0]))
	}
	return shim.Success(nil)
}

// Invoke is called per transaction on the chaincode. Each transaction is
// either a 'get' or a 'set' on the asset created by Init function. The Set
// method may create a new asset by specifying a new key-value pair.
func (s *SmartContract) Invoke(stub shim.ChaincodeStubInterface) peer.Response {
	// Extract the function and args from the transaction proposal
	fn, args := stub.GetFunctionAndParameters()

	var result string
	var err error
	if fn == "handleCrossTransReq" {
		result, err = handleCrossTransReq(stub, args)
	} else if fn == "validate" {
		result, err = validate(stub, args)
	}
	if err != nil {
		return shim.Error(err.Error())
	}
	txid := stub.GetTxID()
	timestamp, err := stub.GetTxTimestamp()
	res := Res{
		Result:    result,
		TxID:      txid,
		Timestamp: strconv.FormatInt(timestamp.Seconds, 10),
	}
	resJson, _ := json.Marshal(res)
	// Return the result as success payload
	return shim.Success(resJson)
}

//
// args: toChannel, from, to, count, payload
func handleCrossTransReq(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 5 {
		return "", fmt.Errorf("Incorrect arguments. Expecting from, to, count, payload")
	}
	// Do something
	return "ok", nil
}

// args: txid, from, to, count
func validate(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("Incorrect arguments. Expecting txid, from, to, count")
	}
	toArgs := append([]string{"handleCrossTransReq"}, args[1:]...)
	res := stub.InvokeChaincode("cc2", util.ToChaincodeArgs(toArgs...), args[0])
	a := res.GetPayload()
	return string(a), nil
}

func set(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key and a value")
	}

	err := stub.PutState(args[0], []byte(args[1]))
	if err != nil {
		return "", fmt.Errorf("Failed to set asset: %s", args[0])
	}
	return args[1], nil
}

// main function starts up the chaincode in the container during instantiate
func main() {
	if err := shim.Start(new(SmartContract)); err != nil {
		fmt.Printf("Error starting SmartContract chaincode: %s", err)
	}
}
