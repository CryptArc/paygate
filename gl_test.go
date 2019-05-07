// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/moov-io/base"
	gl "github.com/moov-io/gl/client"

	"github.com/go-kit/kit/log"
)

type testGLClient struct {
	accounts    []gl.Account
	transaction *gl.Transaction

	err error
}

func (c *testGLClient) Ping() error {
	return c.err
}

func (c *testGLClient) PostTransaction(userId string, lines []transactionLine) (*gl.Transaction, error) {
	if len(lines) == 0 {
		return nil, errors.New("no transactionLine's")
	}
	if c.err != nil {
		return nil, c.err
	}
	return c.transaction, nil
}

func TestGL__verifyAccountExists(t *testing.T) {
	client := &testGLClient{
		accounts: []gl.Account{
			{
				AccountId:     "24125215",
				AccountNumber: "132",
				RoutingNumber: "35151",
				Type:          "Checking",
			},
		},
	}
	dep := &Depository{
		ID:            DepositoryID(nextID()),
		BankName:      "bank name",
		Holder:        "holder",
		HolderType:    Individual,
		Type:          Checking,
		RoutingNumber: "35151",
		AccountNumber: "132",
		Status:        DepositoryUnverified,
	}
	userId := base.ID()
	if err := verifyGLAccountExists(log.NewNopLogger(), client, userId, dep); err != nil {
		t.Fatalf("expected no error, but got %v", err)
	}

	// Change one value
	dep.AccountNumber = "other"
	if err := verifyGLAccountExists(log.NewNopLogger(), client, userId, dep); err == nil {
		t.Fatal("expected errer, but got none")
	}
	dep.AccountNumber = "132"
	dep.RoutingNumber = "other"
	if err := verifyGLAccountExists(log.NewNopLogger(), client, userId, dep); err == nil {
		t.Fatal("expected errer, but got none")
	}
}
