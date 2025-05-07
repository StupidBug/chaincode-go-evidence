package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
	"github.com/hyperledger/fabric-samples/evidence-persistance/chaincode-go-evidence/chaincode"
)

func main() {
	evidenceChaincode, err := contractapi.NewChaincode(&chaincode.EvidenceContract{})
	if err != nil {
		log.Panicf("Error creating evidence chaincode: %v", err)
	}

	if err := evidenceChaincode.Start(); err != nil {
		log.Panicf("Error starting evidence chaincode: %v", err)
	}
}
