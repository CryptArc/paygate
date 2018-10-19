// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-kit/kit/log"
	kitprom "github.com/go-kit/kit/metrics/prometheus"
	stdprom "github.com/prometheus/client_golang/prometheus"
)

var (
	// migrations holds all our SQL migrations to be done (in order)
	migrations = []string{
		// Customers
		`create table if not exists customers(customer_id priary key, user_id, email, default_depository, status, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,

		// Depositories
		`create table if not exists depositories(depository_id primary key, user_id, bank_name, holder, holder_type, type, routing_number, account_number, status, metadata, parent, created_at datetime, last_updated_at datetime, deleted_at datetime);`,

		// Events
		`create table if not exists events(event_id primary key, user_id, topic, message, type, created_at datetime);`,

		// Gateways
		`create table if not exists gateways(gateway_id primary key, user_id, origin, origin_name, destination, destination_name, created_at datetime, deleted_at datetime);`,

		// Originators
		`create table if not exists originators(originator_id primary key, user_id, default_depository, identification, metadata, created_at datetime, last_updated_at datetime, deleted_at datetime);`,

		// Transfers
		`create table if not exists transfers(transfer_id, user_id, type, amount, originator_id, originator_depository, customer, customer_depository, description, standard_entry_class_code, status, same_day, created_at datetime, last_updated_at datetime, deleted_at datetime);`,
	}

	// Metrics
	connections = kitprom.NewGaugeFrom(stdprom.GaugeOpts{
		Name: "sqlite_connections",
		Help: "How many sqlite connections and what status they're in.",
	}, []string{"state"})
)

// TODO(adam): context for shutdown hook
func collectDatabaseStatistics(db *sql.DB) {
	worker := promMetricCollector{}
	go worker.run(db)
}

type promMetricCollector struct{}

func (p *promMetricCollector) run(db *sql.DB) {
	if db == nil {
		return
	}
	t := time.NewTicker(1 * time.Second)
	for range t.C {
		stats := db.Stats()
		connections.With("state", "idle").Set(float64(stats.Idle))
		connections.With("state", "inuse").Set(float64(stats.InUse))
		connections.With("state", "open").Set(float64(stats.OpenConnections))
	}
}

// migrate runs our database migrations (defined at the top of this file)
// over a sqlite database it creates first.
// To configure where on disk the sqlite db is set SQLITE_DB_PATH.
//
// You use db like any other database/sql driver.
//
// https://github.com/mattn/go-sqlite3/blob/master/_example/simple/simple.go
// https://astaxie.gitbooks.io/build-web-application-with-golang/en/05.3.html
func migrate(db *sql.DB, logger log.Logger) error {
	logger.Log("sqlite", "starting sqlite migrations...") // TODO(adam): more detail?
	for i := range migrations {
		row := migrations[i]
		res, err := db.Exec(row)
		if err != nil {
			return fmt.Errorf("migration #%d [%s...] had problem: %v", i, row[:40], err)
		}
		n, err := res.RowsAffected()
		if err == nil {
			logger.Log("sqlite", fmt.Sprintf("migration #%d [%s...] changed %d rows", i, row[:40], n))
		}
	}
	logger.Log("sqlite", "finished migrations")
	return nil
}
