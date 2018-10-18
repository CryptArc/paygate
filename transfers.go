// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type TransferID string

func (id TransferID) Equal(s string) bool {
	return strings.EqualFold(string(id), s)
}

type Transfer struct {
	ID                     TransferID     `json:"id"`
	Type                   TransferType   `json:"transferType"`
	Amount                 Amount         `json:"amount"`
	Originator             OriginatorID   `json:"originator"`
	OriginatorDepository   DepositoryID   `json:"originatorDepository"`
	Customer               CustomerID     `json:"customer"`
	CustomerDepository     DepositoryID   `json:"customerDepository"`
	Description            string         `json:"description"`
	StandardEntryClassCode string         `json:"standardEntryClassCode"`
	Status                 TransferStatus `json:"status"`
	SameDay                bool           `json:"sameDay"`
	Created                time.Time      `json:"created"`

	WEBDetail WEBDetail `json:"WEBDetail,omitempty"`
}

type transferRequest struct {
	Type                   TransferType `json:"transferType"`
	Amount                 Amount       `json:"amount"`
	Originator             OriginatorID `json:"originator"`
	OriginatorDepository   DepositoryID `json:"originatorDepository"`
	Customer               CustomerID   `json:"customer"`
	CustomerDepository     DepositoryID `json:"customerDepository"`
	Description            string       `json:"description,omitempty"`
	StandardEntryClassCode string       `json:"standardEntryClassCode"`
	SameDay                bool         `json:"sameDay,omitempty"`
	WEBDetail              WEBDetail    `json:"WEBDetail,omitempty"`
}

func (r transferRequest) missingFields() bool {
	return string(r.Type) == "" ||
		string(r.Originator) == "" ||
		string(r.OriginatorDepository) == "" ||
		string(r.Customer) == "" ||
		string(r.CustomerDepository) == "" ||
		r.StandardEntryClassCode == ""
}

type TransferType string

const (
	PushTransfer TransferType = "Push"
	PullTransfer TransferType = "Pull"
)

func (tt *TransferType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "push":
		*tt = PushTransfer
		return nil
	case "pull":
		*tt = PullTransfer
		return nil
	}
	return fmt.Errorf("unknown TransferType %q", s)
}

type TransferStatus string

const (
	TransferCanceled  TransferStatus = "canceled"
	TransferFailed                   = "failed"
	TransferPending                  = "pending"
	TransferProcessed                = "processed"
	TransferReclaimed                = "reclaimed"
)

func (ts TransferStatus) Equal(other TransferStatus) bool {
	return strings.EqualFold(string(ts), string(other))
}

func (ts *TransferStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "canceled":
		*ts = TransferCanceled
		return nil
	case "failed":
		*ts = TransferFailed
		return nil
	case "pending":
		*ts = TransferPending
		return nil
	case "processed":
		*ts = TransferProcessed
		return nil
	case "reclaimed":
		*ts = TransferReclaimed
		return nil
	}
	return fmt.Errorf("unknown TransferStatus %q", s)
}

type WEBDetail struct {
	PaymentType WEBPaymentType `json:"PaymentType,omitempty"`
}

type WEBPaymentType string

const (
	WEBSingle      WEBPaymentType = "Single"
	WEBReoccurring                = "Reoccurring"
)

func addTransfersRoute(r *mux.Router, eventRepo eventRepository, transferRepo transferRepository) {
	r.Methods("GET").Path("/transfers").HandlerFunc(getUserTransfers(transferRepo))
	r.Methods("POST").Path("/transfers").HandlerFunc(createUserTransfers(transferRepo))
	r.Methods("POST").Path("/transfers/batch").HandlerFunc(createUserTransfers(transferRepo))

	r.Methods("DELETE").Path("/transfers/{transferId}").HandlerFunc(deleteUserTransfer(transferRepo))
	r.Methods("GET").Path("/transfers/{transferId}").HandlerFunc(getUserTransfer(transferRepo))
	r.Methods("GET").Path("/transfers/{transferId}/events").HandlerFunc(getUserTransferEvents(eventRepo, transferRepo))
	r.Methods("POST").Path("/transfers/{transferId}/failed").HandlerFunc(validateUserTransfer(transferRepo))
	r.Methods("POST").Path("/transfers/{transferId}/files").HandlerFunc(getUserTransferFiles(transferRepo))
}

func getTransferId(r *http.Request) TransferID {
	vars := mux.Vars(r)
	v, ok := vars["transferId"]
	if ok {
		return TransferID(v)
	}
	return TransferID("")
}

func getUserTransfers(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransfers")
		if err != nil {
			return
		}

		userId := getUserId(r)
		transfers, err := transferRepo.getUserTransfers(userId)
		if err != nil {
			fmt.Println("A")
			internalError(w, err, "getUserTransfers")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			fmt.Println("B")
			internalError(w, err, "getUserTransfers")
			return
		}
	}
}

func getUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransfer")
		if err != nil {
			return
		}

		id, userId := getTransferId(r), getUserId(r)
		transfer, err := transferRepo.getUserTransfer(id, userId)
		if err != nil {
			internalError(w, err, "getUserTransfer")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			internalError(w, err, "getUserTransfer")
			return
		}
	}
}

func createUserTransfers(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserTransfers")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}

		var req transferRequest
		var requests []transferRequest

		if err := json.Unmarshal(bs, &req); err != nil {
			// failed, but try []transferRequest
			if err := json.Unmarshal(bs, &requests); err != nil {
				encodeError(w, err)
				return
			}
		} else {
			if req.missingFields() {
				encodeError(w, errMissingRequiredJson)
				return
			}
			requests = append(requests, req)
		}

		userId := getUserId(r)
		transfers, err := transferRepo.createUserTransfers(userId, requests)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if len(requests) == 1 {
			// don't render surrounding array for single transfer create
			// (it's coming from POST /transfers, not POST /transfers/batch)
			if err := json.NewEncoder(w).Encode(transfers[0]); err != nil {
				internalError(w, err, "createUserTransfers")
				return
			}
		}

		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			internalError(w, err, "createUserTransfers")
			return
		}
	}
}

func deleteUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserTransfer")
		if err != nil {
			return
		}

		id, userId := getTransferId(r), getUserId(r)
		if err := transferRepo.deleteUserTransfer(id, userId); err != nil {
			internalError(w, err, "deleteUserTransfer")
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// POST /transfers/{id}/failed
// 200 - no errors
// 400 - errors, check json
func validateUserTransfer(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserTransfer")
		if err != nil {
			return
		}

		id, _ := getTransferId(r), getUserId(r)
		if id.Equal("200") {
			w.WriteHeader(http.StatusOK)
		} else {
			encodeError(w, errors.New("TODO NACHA ERROR"))
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func getUserTransferFiles(transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserTransfer")
		if err != nil {
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte("files, todo"))
	}
}

func getUserTransferEvents(eventRepo eventRepository, transferRepo transferRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserTransferEvents")
		if err != nil {
			return
		}

		id, userId := getTransferId(r), getUserId(r)

		transfer, err := transferRepo.getUserTransfer(id, userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		events, err := eventRepo.getUserTransferEvents(userId, transfer.ID)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(events); err != nil {
			internalError(w, err, "events")
			return
		}

	}
}

type transferRepository interface {
	getUserTransfers(userId string) ([]*Transfer, error)
	createUserTransfers(userId string, requests []transferRequest) ([]*Transfer, error)

	getUserTransfer(id TransferID, userId string) (*Transfer, error)
	deleteUserTransfer(id TransferID, userId string) error
}

type memTransferRepo struct{}

func (r memTransferRepo) getUserTransfers(userId string) ([]*Transfer, error) {
	amount := Amount{}
	if err := amount.UnmarshalJSON([]byte(`"USD 12.47"`)); err != nil {
		return nil, err
	}

	return []*Transfer{
		&Transfer{
			ID:                     TransferID(nextID()),
			Type:                   PushTransfer,
			Amount:                 amount,
			Originator:             OriginatorID(nextID()),
			OriginatorDepository:   DepositoryID(nextID()),
			Customer:               CustomerID(nextID()),
			CustomerDepository:     DepositoryID(nextID()),
			StandardEntryClassCode: "1",
			Status:                 TransferPending,
			Created:                time.Now(),
		},
	}, nil
}

func (r memTransferRepo) createUserTransfers(userId string, requests []transferRequest) ([]*Transfer, error) {
	return r.getUserTransfers(userId)
}

func (r memTransferRepo) getUserTransfer(id TransferID, userId string) (*Transfer, error) {
	txs, err := r.getUserTransfers(userId)
	if err != nil {
		return nil, err
	}
	return txs[0], nil
}

func (r memTransferRepo) deleteUserTransfer(id TransferID, userId string) error {
	return nil
}
