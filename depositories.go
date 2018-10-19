// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
)

type DepositoryID string

func (id DepositoryID) empty() bool {
	return string(id) == ""
}

type Depository struct {
	ID            DepositoryID     `json:"id"`
	BankName      string           `json:"bankName"`
	Holder        string           `json:"holder"`
	HolderType    HolderType       `json:"holderType"`
	Type          AccountType      `json:"type"`
	RoutingNumber string           `json:"routingNumber"`
	AccountNumber string           `json:"accountNumber"`
	Status        DepositoryStatus `json:"status"`
	Metadata      string           `json:"metadata"`
	Parent        *DepositoryID    `json:"parent"`
	Created       time.Time        `json:"created"`
	Updated       time.Time        `json:"updated"`
}

type depositoryRequest struct {
	BankName      string        `json:"bankName,omitempty"`
	Holder        string        `json:"holder,omitempty"`
	HolderType    HolderType    `json:"holderType,omitempty"`
	Type          AccountType   `json:"type,omitempty"`
	RoutingNumber string        `json:"routingNumber,omitempty"`
	AccountNumber string        `json:"accountNumber,omitempty"`
	Metadata      string        `json:"metadata,omitempty"`
	Parent        *DepositoryID `json:"parent,omitempty"`
}

func (r depositoryRequest) missingFields() bool {
	empty := func(s string) bool { return s == "" }
	return (empty(r.BankName) ||
		empty(r.Holder) ||
		r.HolderType.empty() ||
		r.Type.empty() ||
		empty(r.RoutingNumber) ||
		empty(r.AccountNumber))
}

type HolderType string

const (
	Individual HolderType = "Individual"
	Business   HolderType = "Business"
)

func (t *HolderType) empty() bool {
	return string(*t) == ""
}

func (t *HolderType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "individual":
		*t = Individual
		return nil
	case "business":
		*t = Business
		return nil
	}
	return fmt.Errorf("unknown HolderType %q", s)
}

type DepositoryStatus string

const (
	DepositoryUnverified DepositoryStatus = "Unverified"
	DepositoryVerified   DepositoryStatus = "Verified"
)

func (ds DepositoryStatus) empty() bool {
	return string(ds) == ""
}

func (ds *DepositoryStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "unverified":
		*ds = DepositoryUnverified
		return nil
	case "verified":
		*ds = DepositoryVerified
		return nil
	}
	return fmt.Errorf("unknown DepositoryStatus %q", s)
}

func addDepositoryRoutes(r *mux.Router, depositoryRepo depositoryRepository) {
	r.Methods("GET").Path("/depositories").HandlerFunc(getUserDepositories(depositoryRepo))
	r.Methods("POST").Path("/depositories").HandlerFunc(createUserDepository(depositoryRepo))

	r.Methods("GET").Path("/depositories/{depositoryId}").HandlerFunc(getUserDepository(depositoryRepo))
	r.Methods("PATCH").Path("/depositories/{depositoryId}").HandlerFunc(updateUserDepository(depositoryRepo))
	r.Methods("DELETE").Path("/depositories/{depositoryId}").HandlerFunc(deleteUserDepository(depositoryRepo))

	r.Methods("POST").Path("/depositories/{depositoryId}/micro-deposits").HandlerFunc(initiateMicroDeposits(depositoryRepo))
}

// GET /depositories
// response: [ depository ]
func getUserDepositories(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepositories")
		if err != nil {
			return
		}

		userId := getUserId(r)
		deposits, err := depositoryRepo.getUserDepositories(userId)
		if err != nil {
			internalError(w, err, "getUserDepositories")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(deposits); err != nil {
			internalError(w, err, "getUserDepositories")
			return
		}
	}
}

// POST /depositories
// request: model w/o ID
// response: 201 w/ depository json
func createUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "createUserDepository")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}

		var req depositoryRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}
		if req.missingFields() {
			encodeError(w, errMissingRequiredJson)
			return
		}

		userId, now := getUserId(r), time.Now()
		depository := &Depository{
			ID:            DepositoryID(nextID()),
			BankName:      req.BankName,
			Holder:        req.Holder,
			HolderType:    req.HolderType,
			Type:          req.Type,
			RoutingNumber: req.RoutingNumber,
			AccountNumber: req.AccountNumber,
			Status:        DepositoryUnverified,
			Metadata:      req.Metadata,
			Parent:        req.Parent,
			Created:       now,
			Updated:       now,
		}

		if err := depositoryRepo.upsertUserDepository(userId, depository); err != nil {
			internalError(w, err, "createUserDepository")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err, "createUserDepository")
			return
		}
	}
}

func getUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "getUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		depository, err := depositoryRepo.getUserDepository(id, userId)
		if err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err, "getUserDepository")
			return
		}
	}
}

func updateUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "updateUserDepository")
		if err != nil {
			return
		}

		bs, err := read(r.Body)
		if err != nil {
			encodeError(w, err)
			return
		}
		var req depositoryRequest
		if err := json.Unmarshal(bs, &req); err != nil {
			encodeError(w, err)
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		depository, err := depositoryRepo.getUserDepository(id, userId)
		if err != nil {
			internalError(w, err, "depositories")
			return
		}
		if depository == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Update model
		if req.BankName != "" {
			depository.BankName = req.BankName
		}
		if req.Holder != "" {
			depository.Holder = req.Holder
		}
		if req.HolderType != "" {
			depository.HolderType = req.HolderType
		}
		if req.Type != "" {
			depository.Type = req.Type
		}
		if req.RoutingNumber != "" {
			depository.RoutingNumber = req.RoutingNumber
		}
		if req.AccountNumber != "" {
			depository.AccountNumber = req.AccountNumber
		}
		if req.Metadata != "" {
			depository.Metadata = req.Metadata
		}
		if !req.Parent.empty() {
			depository.Parent = req.Parent
		}
		depository.Updated = time.Now()

		if err := depositoryRepo.upsertUserDepository(userId, depository); err != nil {
			internalError(w, err, "updateUserDepository")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(depository); err != nil {
			internalError(w, err, "updateUserDepository")
			return
		}
	}
}

func deleteUserDepository(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserDepository")
		if err != nil {
			return
		}

		id, userId := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if err := depositoryRepo.deleteUserDepository(id, userId); err != nil {
			encodeError(w, err)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}
}

// POST /depositories/{id}/micro-deposits
// 200 - Micro deposits verified
// 201 - Micro deposits initiated
// 400 - Invalid Amounts
// 404 - A depository with the specified ID was not found.
// 409 - Too many attempts. Bank already verified.
func initiateMicroDeposits(depositoryRepo depositoryRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w, err := wrapResponseWriter(w, r, "deleteUserDepository")
		if err != nil {
			return
		}

		id, _ := getDepositoryId(r), getUserId(r)
		if id == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// TODO(adam): something
		// if err := depositoryRepo.initiateMicroDeposits(id, userId); err != nil {
		// 	// TODO(adam)
		// }

		switch id {
		case "200":
			w.WriteHeader(http.StatusOK)
		case "201":
			w.WriteHeader(http.StatusCreated)
		case "400":
			w.WriteHeader(http.StatusBadRequest)
		case "404":
			w.WriteHeader(http.StatusNotFound)
		case "409":
			w.WriteHeader(http.StatusConflict)
		}
	}
}

// getDepositoryId extracts the DepositoryID from the incoming request.
func getDepositoryId(r *http.Request) DepositoryID {
	v := mux.Vars(r)
	id, ok := v["depositoryId"]
	if !ok {
		return DepositoryID("")
	}
	return DepositoryID(id)
}

type depositoryRepository interface {
	getUserDepositories(userId string) ([]*Depository, error)
	getUserDepository(id DepositoryID, userId string) (*Depository, error)

	upsertUserDepository(userId string, dep *Depository) error
	deleteUserDepository(id DepositoryID, userId string) error

	initiateMicroDeposits(id DepositoryID, userId string) error
}

type sqliteDepositoryRepo struct {
	db  *sql.DB
	log log.Logger
}

func (r *sqliteDepositoryRepo) close() error {
	return r.db.Close()
}

func (r *sqliteDepositoryRepo) getUserDepositories(userId string) ([]*Depository, error) {
	query := `select depository_id from depositories where user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	rows, err := stmt.Query(userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var depositoryIds []string
	for rows.Next() {
		var row string
		rows.Scan(&row)
		if row != "" {
			depositoryIds = append(depositoryIds, row)
		}
	}

	var depositories []*Depository
	for i := range depositoryIds {
		dep, err := r.getUserDepository(DepositoryID(depositoryIds[i]), userId)
		if err == nil && dep != nil && dep.BankName != "" {
			depositories = append(depositories, dep)
		}
	}
	return depositories, nil
}

// (depository_id primary key, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at, deleted_at)

func (r *sqliteDepositoryRepo) getUserDepository(id DepositoryID, userId string) (*Depository, error) {
	query := `select depository_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at
from depositories
where depository_id = ? and user_id = ? and deleted_at is null
limit 1`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return nil, err
	}
	row := stmt.QueryRow(id, userId)

	dep := &Depository{}
	err = row.Scan(&dep.ID, &dep.BankName, &dep.Holder, &dep.HolderType, &dep.Type, &dep.RoutingNumber, &dep.AccountNumber, &dep.Status, &dep.Metadata, &dep.Parent, &dep.Created, &dep.Updated)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}
	if dep.ID == "" || dep.BankName == "" {
		return nil, nil // no records found
	}

	// TODO(adam): dep.validateStatus() ? (and other fields)

	return dep, nil
}

func (r *sqliteDepositoryRepo) upsertUserDepository(userId string, dep *Depository) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	now := time.Now()
	if dep.Created.IsZero() {
		dep.Created = now
		dep.Updated = now
	}

	query := `insert or ignore into depositories (depository_id, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at, last_updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	res, err := stmt.Exec(dep.ID, userId, dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber, dep.AccountNumber, dep.Status, dep.Metadata, dep.Parent, dep.Created, dep.Updated)
	if err != nil {
		return fmt.Errorf("problem upserting depository=%q, userId=%q: %v", dep.ID, userId, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		query = `update depositories
set bank_name = ?, holder = ?, holder_type = ?, type = ?, routing_number = ?,
account_number = ?, status = ?, metadata = ?, parent = ?, last_updated_at = ?
where depository_id = ? and user_id = ? and deleted_at is null`
		stmt, err := tx.Prepare(query)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			dep.BankName, dep.Holder, dep.HolderType, dep.Type, dep.RoutingNumber,
			dep.AccountNumber, dep.Status, dep.Metadata, dep.Parent, time.Now(),
			dep.ID, userId)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *sqliteDepositoryRepo) deleteUserDepository(id DepositoryID, userId string) error {
	query := `update depositories set deleted_at = ? where depository_id = ? and user_id = ? and deleted_at is null`
	stmt, err := r.db.Prepare(query)
	if err != nil {
		return err
	}
	if _, err := stmt.Exec(time.Now(), id, userId); err != nil {
		return fmt.Errorf("error deleting depository_id=%q, user_id=%q: %v", id, userId, err)
	}
	return nil
}

func (r *sqliteDepositoryRepo) initiateMicroDeposits(id DepositoryID, userId string) error {
	// TODO: implement, record anything sent -- table = depository_micro_deposits(depository_id, amount, created_at)
	return nil
}
