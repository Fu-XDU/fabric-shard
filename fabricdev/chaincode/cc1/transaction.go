package main

const (
	IN    = "IN"
	OUT   = "OUT"
	INOUT = "INOUT"
)

type Transaction struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Count     string `json:"count"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
}

func newTransaction(from string, to string, count string, txTrpe string) (tx *Transaction) {
	tx = &Transaction{
		From:  from,
		To:    to,
		Count: count,
		Type:  txTrpe,
	}
	return
}

/*
func (tx *Transaction) toLedger(stub shim.ChaincodeStubInterface) {
	timestamp, _ := stub.GetTxTimestamp()
	tx.Timestamp = timestamp.String()
	txJson, _ := json.Marshal(*tx)
	txHash := hex.Encode(util.ComputeSHA256(txJson))
	args := []string{txHash, string(txJson)}
	_, _ = set(stub, args)
}
*/
