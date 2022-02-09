package main

import (
	"encoding/json"
	"fmt"
	"github.com/cc1/hex"
	"github.com/hyperledger/fabric-chaincode-go/shim"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/util"
	"math/big"
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
	if fn == "setBalance" {
		result, err = setBalance(stub, args)
	} else if fn == "transfer" {
		result, err = transfer(stub, args)
	} else if fn == "getBalance" {
		result, err = GetBalance(stub, args)
	} else if fn == "crossTransReq" {
		result, err = crossTransReq(stub, args)
	} else if fn == "pay" {
		result, err = pay(stub, args)
	} else if fn == "add" {
		result, err = add(stub, args)
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

func setBalance(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	args[1] = hex.RemovePrefix(args[1])
	args = append(args, "+")
	result, err := update(stub, args)
	if err == nil {
		result = args[1]
	}
	return result, err
}

func GetBalance(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	result, err := get(stub, args)
	return result, err
}

// args: key
func getBalance(stub shim.ChaincodeStubInterface, key string) (*big.Int, error) {
	balanceStr, err := get(stub, []string{key})
	if err != nil {
		return nil, err
	}
	balance, ok := new(big.Int).SetString(balanceStr, 16)
	if !ok {
		return nil, fmt.Errorf("Can't parse string to big int")
	}
	return balance, nil
}

// args:from to count
func transfer(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("Incorrect arguments. Expecting from, to, count")
	}
	args[2] = hex.RemovePrefix(args[2])
	count, err := hex.Str2Big(args[2])
	if err != nil {
		return "", err
	}
	// Must sub first
	_, err = sub(stub, args[0], count)
	if err != nil {
		return "", err
	}
	_, err = addBalance(stub, args[1], count)
	//tx := newTransaction(args[0], args[1], args[2], INOUT)
	//tx.toLedger(stub)
	return "Success", err
}

// args:from to count
func pay(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 3 {
		return "", fmt.Errorf("Incorrect arguments. Expecting from, to, count")
	}
	args[2] = hex.RemovePrefix(args[2])
	count, err := hex.Str2Big(args[2])
	if err != nil {
		return "", err
	}

	_, err = sub(stub, args[0], count)
	if err != nil {
		return "", err
	}
	return "Success", err
}

// args:from to count outTxHash
func add(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 4 {
		return "", fmt.Errorf("Incorrect arguments. Expecting from, to, count, outTxHash")
	}
	args[2] = hex.RemovePrefix(args[2])
	_, err := update(stub, []string{args[1], args[2], "+"})
	if err != nil {
		return "", err
	}
	return "Success", err
}

func addBalance(stub shim.ChaincodeStubInterface, key string, count *big.Int) (string, error) {
	countHex := count.Text(16)
	_, err := update(stub, []string{key, countHex, "+"})
	if err != nil {
		return "", err
	}
	return countHex, err
}

func sub(stub shim.ChaincodeStubInterface, key string, count *big.Int) (string, error) {
	/*
	balance, err := getBalance(stub, key)
	if err != nil {
		return "", err
	}
	//  -1 if balance < count
	//   0 if balance == count
	//  +1 if balance > count
	enough := balance.Cmp(count)
	if enough == -1 {
		return "", fmt.Errorf("Insufficient balance, need: %v, hold: %v", hex.WithPrefix(count.Text(16)), hex.WithPrefix(balance.Text(16)))
	}
	 */
	countHex := count.Text(16)
	_, err := update(stub, []string{key, countHex, "-"})
	if err != nil {
		return "", err
	}
	return countHex, nil
}

/**
 * Updates the ledger to include a new delta for a particular variable. If this is the first time
 * this variable is being added to the ledger, then its initial value is assumed to be 0. The arguments
 * to give in the args array are as follows:
 *	- args[0] -> name of the variable
 *	- args[1] -> new delta (float)
 *	- args[2] -> operation (currently supported are addition "+" and subtraction "-")
 *
 * @param APIstub The chaincode shim
 * @param args The arguments array for the update invocation
 *
 * @return A response structure indicating success or failure with a message
 */
// set
func update(APIstub shim.ChaincodeStubInterface, args []string) (string, error) {
	// Check we have a valid number of args
	// name val op
	wantArgsCount := 3
	if len(args) != wantArgsCount {
		return "", fmt.Errorf("Incorrect number of arguments, expecting %v", wantArgsCount)
	}

	// Extract the args
	name := args[0]
	op := args[2]
	// TODO:检查args[1]是否为16进制

	// Make sure a valid operator is provided
	if op != "+" && op != "-" {
		return "", fmt.Errorf("Operator %s is unrecognized", op)
	}

	// Retrieve info needed for the update procedure
	txid := APIstub.GetTxID()
	compositeIndexName := "varName~op~value~txID"

	// Create the composite key that will allow us to query for all deltas on a particular variable
	compositeKey, compositeErr := APIstub.CreateCompositeKey(compositeIndexName, []string{name, op, args[1], txid})
	if compositeErr != nil {
		return "", fmt.Errorf("Could not create a composite key for %s: %s", name, compositeErr.Error())
	}

	// Save the composite key index
	compositePutErr := APIstub.PutState(compositeKey, []byte{0x00})
	if compositePutErr != nil {
		return "", fmt.Errorf("Could not put operation for %s in the ledger: %s", name, compositePutErr.Error())
	}

	return args[1], nil
}

/**
 * Retrieves the aggregate value of a variable in the ledger. Gets all delta rows for the variable
 * and computes the final value from all deltas. The args array for the invocation must contain the
 * following argument:
 *	- args[0] -> The name of the variable to get the value of
 *
 * @param APIstub The chaincode shim
 * @param args The arguments array for the get invocation
 *
 * @return A response structure indicating success or failure with a message
 */
func get(APIstub shim.ChaincodeStubInterface, args []string) (string, error) {
	// Check we have a valid number of args
	wantArgsCount := 1
	if len(args) != wantArgsCount {
		return "", fmt.Errorf("Incorrect number of arguments, expecting %v", wantArgsCount)
	}

	name := args[0]
	// Get all deltas for the variable
	deltaResultsIterator, deltaErr := APIstub.GetStateByPartialCompositeKey("varName~op~value~txID", []string{name})
	if deltaErr != nil {
		return "", fmt.Errorf("Could not retrieve value for %s: %s", name, deltaErr.Error())
	}
	defer deltaResultsIterator.Close()

	// Check the variable existed
	if !deltaResultsIterator.HasNext() {
		return "", fmt.Errorf("No variable by the name %s exists", name)
	}

	// Iterate through result set and compute final value
	finalVal := new(big.Int).SetInt64(0)
	for i := 0; deltaResultsIterator.HasNext(); i++ {
		// Get the next row
		responseRange, nextErr := deltaResultsIterator.Next()
		if nextErr != nil {
			return "", fmt.Errorf(nextErr.Error())
		}

		// Split the composite key into its component parts
		_, keyParts, splitKeyErr := APIstub.SplitCompositeKey(responseRange.Key)
		if splitKeyErr != nil {
			return "", fmt.Errorf(splitKeyErr.Error())
		}

		// Retrieve the delta value and operation
		operation := keyParts[1]
		valueStr := keyParts[2]

		// Convert the value string and perform the operation
		value, ok := new(big.Int).SetString(valueStr, 16)
		if !ok {
			return "", fmt.Errorf("new(big.Int).SetString(valueStr, 16) !ok")
		}
		switch operation {
		case "+":
			finalVal.Add(finalVal, value)
		case "-":
			finalVal.Sub(finalVal, value)
		default:
			return "", fmt.Errorf("Unrecognized operation %s", operation)
		}
	}

	return finalVal.Text(16), nil
}

// setStandard stores the asset (both key and value) on the ledger. If the key exists,
// it will override the value with the new one
func setStandard(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key and a value")
	}

	err := stub.PutState(args[0], []byte(args[1]))
	if err != nil {
		return "", fmt.Errorf("Failed to set asset: %s", args[0])
	}
	return args[1], nil
}

// getStandard returns the value of the specified asset key
func getStandard(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("Incorrect arguments. Expecting a key")
	}

	value, err := stub.GetState(args[0])
	if err != nil {
		return "", fmt.Errorf("Failed to get asset: %s with error: %s", args[0], err)
	}
	if value == nil {
		return "", fmt.Errorf("Asset not found: %s", args[0])
	}
	return string(value), nil
}

//
// args: rootChannel, toChannel, from, to, count, payload
func crossTransReq(stub shim.ChaincodeStubInterface, args []string) (string, error) {
	if len(args) != 6 {
		return "", fmt.Errorf("Incorrect arguments. Expecting rootChannel, toChannel, from, to, count, payload")
	}
	toArgs := append([]string{"handleCrossTransReq"}, args[1:]...)
	res := stub.InvokeChaincode("cc2", util.ToChaincodeArgs(toArgs...), args[0])
	a := res.GetPayload()
	return string(a), nil
}

// main function starts up the chaincode in the container during instantiate
func main() {
	if err := shim.Start(new(SmartContract)); err != nil {
		fmt.Printf("Error starting SmartContract chaincode: %s", err)
	}
}
