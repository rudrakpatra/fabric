/*
SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing documents
type SmartContract struct {
	contractapi.Contract
}

// Account represents the organization's account balance (Public data)
type Account struct {
	Balance int `json:"balance"`
}

// Document represents a document in the private data collection
type Document struct {
	DocID       string `json:"docID"`
	DocTitle    string `json:"docTitle"`
	DocData     string `json:"docData"`
	DocDataHash string `json:"docDataHash"`
	DocPrice    int    `json:"docPrice"`
}

// AddBalance adds the specified amount to the organization's account balance
func (s *SmartContract) AddBalance(ctx contractapi.TransactionContextInterface) error {
	// Get transient data containing the amount
	transientData, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient data: %v", err)
	}

	// Get amount from transient data
	amountBytes, ok := transientData["amount"]
	if !ok {
		return fmt.Errorf("amount not found in transient data")
	}

	amount := 0
	err = json.Unmarshal(amountBytes, &amount)
	if err != nil {
		return fmt.Errorf("error unmarshaling amount: %v", err)
	}

	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client org ID: %v", err)
	}

	// Get current account balance
	accountBytes, err := ctx.GetStub().GetState(clientOrgID)
	if err != nil {
		return fmt.Errorf("failed to read account: %v", err)
	}

	var account Account
	if accountBytes != nil {
		err = json.Unmarshal(accountBytes, &account)
		if err != nil {
			return err
		}
	}

	// Update balance
	account.Balance += amount

	// Save updated account
	accountBytes, err = json.Marshal(account)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(clientOrgID, accountBytes)
}

// AddDocument adds a new document to the organization's private collection
func (s *SmartContract) AddDocument(ctx contractapi.TransactionContextInterface) error {
	// Get transient data
	transientData, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient data: %v", err)
	}

	// Get document details from transient data
	docBytes, ok := transientData["document"]
	if !ok {
		return fmt.Errorf("document not found in transient data")
	}

	var doc Document
	err = json.Unmarshal(docBytes, &doc)
	if err != nil {
		return fmt.Errorf("error unmarshaling document: %v", err)
	}

	// Compute document hash
	hash := sha256.Sum256([]byte(doc.DocData))
	doc.DocDataHash = hex.EncodeToString(hash[:])

	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client org ID: %v", err)
	}

	// Store in private data collection
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	// Use collection name format: "_implicit_org_<OrgName>"
	collectionName := fmt.Sprintf("_implicit_org_%s", clientOrgID)
	return ctx.GetStub().PutPrivateData(collectionName, doc.DocID, docJSON)
}

// GetBalance retrieves the organization's account balance
func (s *SmartContract) GetBalance(ctx contractapi.TransactionContextInterface) (*Account, error) {
	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return nil, fmt.Errorf("failed to get client org ID: %v", err)
	}

	// Get account balance
	accountBytes, err := ctx.GetStub().GetState(clientOrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to read account: %v", err)
	}

	if accountBytes == nil {
		return &Account{Balance: 0}, nil
	}

	var account Account
	err = json.Unmarshal(accountBytes, &account)
	if err != nil {
		return nil, err
	}

	return &account, nil
}

// UpdateDocument updates an existing document's data and optionally its hash
func (s *SmartContract) UpdateDocument(ctx contractapi.TransactionContextInterface) error {
	// Get transient data
	transientData, err := ctx.GetStub().GetTransient()
	if err != nil {
		return fmt.Errorf("error getting transient data: %v", err)
	}

	// Get update details from transient data
	updateBytes, ok := transientData["update"]
	if !ok {
		return fmt.Errorf("update data not found in transient data")
	}

	type UpdateData struct {
		DocID      string `json:"docID"`
		NewDocData string `json:"newDocData"`
		UpdateHash bool   `json:"updateHash"`
	}

	var updateData UpdateData
	err = json.Unmarshal(updateBytes, &updateData)
	if err != nil {
		return fmt.Errorf("error unmarshaling update data: %v", err)
	}

	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return fmt.Errorf("failed to get client org ID: %v", err)
	}

	collectionName := fmt.Sprintf("_implicit_org_%s", clientOrgID)

	// Get existing document
	docJSON, err := ctx.GetStub().GetPrivateData(collectionName, updateData.DocID)
	if err != nil {
		return fmt.Errorf("failed to get document: %v", err)
	}
	if docJSON == nil {
		return fmt.Errorf("document %s not found", updateData.DocID)
	}

	var doc Document
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return err
	}

	// Update document data
	doc.DocData = updateData.NewDocData
	if updateData.UpdateHash {
		hash := sha256.Sum256([]byte(updateData.NewDocData))
		doc.DocDataHash = hex.EncodeToString(hash[:])
	}

	// Store updated document
	docJSON, err = json.Marshal(doc)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutPrivateData(collectionName, updateData.DocID, docJSON)
}

// GetAllDocuments returns all documents in the organization's private collection
func (s *SmartContract) GetAllDocuments(ctx contractapi.TransactionContextInterface) ([]Document, error) {
	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return nil, fmt.Errorf("failed to get client org ID: %v", err)
	}

	collectionName := fmt.Sprintf("_implicit_org_%s", clientOrgID)

	// Get all documents
	iterator, err := ctx.GetStub().GetPrivateDataByRange(collectionName, "", "")
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	var documents []Document
	for iterator.HasNext() {
		response, err := iterator.Next()
		if err != nil {
			return nil, err
		}

		var doc Document
		if err := json.Unmarshal(response.Value, &doc); err != nil {
			return nil, err
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// GetDocument returns a specific document's details
func (s *SmartContract) GetDocument(ctx contractapi.TransactionContextInterface, docID string) (*Document, error) {
	// Get client org ID
	clientOrgID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return nil, fmt.Errorf("failed to get client org ID: %v", err)
	}

	collectionName := fmt.Sprintf("_implicit_org_%s", clientOrgID)

	// Get document
	docJSON, err := ctx.GetStub().GetPrivateData(collectionName, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %v", err)
	}
	if docJSON == nil {
		return nil, fmt.Errorf("document %s not found", docID)
	}

	var doc Document
	if err := json.Unmarshal(docJSON, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func main() {
	assetChaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		log.Panicf("Error creating asset-transfer-basic chaincode: %v", err)
	}

	if err := assetChaincode.Start(); err != nil {
		log.Panicf("Error starting asset-transfer-basic chaincode: %v", err)
	}
}
