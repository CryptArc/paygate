// Copyright 2019 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package filetransfer

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/moov-io/base/admin"
	"github.com/moov-io/paygate/internal/database"

	"github.com/go-kit/kit/log"
)

type testSqliteRepository struct {
	*sqliteRepository

	testDB *database.TestSQLiteDB
}

func (r *testSqliteRepository) Close() error {
	r.sqliteRepository.Close()
	return r.testDB.Close()
}

func createTestsqliteRepository(t *testing.T) *testSqliteRepository {
	t.Helper()

	db := database.CreateTestSqliteDB(t)
	repo := &sqliteRepository{db: db.DB}
	return &testSqliteRepository{repo, db}
}

func TestsqliteRepository__getCounts(t *testing.T) {
	repo := createTestsqliteRepository(t)
	defer repo.Close()

	writeCutoffTime(t, repo)
	writeFTPConfig(t, repo)
	writeFileTransferConfig(t, repo.db)

	cutoffs, ftps, filexfers := repo.GetCounts()
	if cutoffs != 1 {
		t.Errorf("got %d", cutoffs)
	}
	if ftps != 1 {
		t.Errorf("got %d", ftps)
	}
	if filexfers != 1 {
		t.Errorf("got %d", filexfers)
	}

	// If we read at least one row from each config table we need to make sure NewRepository
	// returns sqliteRepository (rather than localFileTransferRepository)
	r := NewRepository(repo.db, "")
	if _, ok := r.(*sqliteRepository); !ok {
		t.Errorf("got %T", r)
	}
}

func writeCutoffTime(t *testing.T, repo *testSqliteRepository) {
	t.Helper()

	query := `insert into cutoff_times (routing_number, cutoff, location) values ('123456789', 1700, 'America/New_York');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestsqliteRepository__GetCutoffTimes(t *testing.T) {
	repo := createTestsqliteRepository(t)
	defer repo.Close()

	writeCutoffTime(t, repo)

	cutoffTimes, err := repo.GetCutoffTimes()
	if err != nil {
		t.Fatal(err)
	}
	if len(cutoffTimes) != 1 {
		t.Errorf("len(cutoffTimes)=%d", len(cutoffTimes))
	}
	if cutoffTimes[0].RoutingNumber != "123456789" {
		t.Errorf("cutoffTimes[0].RoutingNumber=%s", cutoffTimes[0].RoutingNumber)
	}
	if cutoffTimes[0].Cutoff != 1700 {
		t.Errorf("cutoffTimes[0].Cutoff=%d", cutoffTimes[0].Cutoff)
	}
	if v := cutoffTimes[0].Loc.String(); v != "America/New_York" {
		t.Errorf("cutoffTimes[0].Loc=%v", v)
	}
}

func writeFTPConfig(t *testing.T, repo *testSqliteRepository) {
	t.Helper()

	query := `insert into ftp_configs (routing_number, hostname, username, password) values ('123456789', 'ftp.moov.io', 'moov', 'secret');`
	stmt, err := repo.db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestsqliteRepository__GetFTPConfigs(t *testing.T) {
	repo := createTestsqliteRepository(t)
	defer repo.Close()

	writeFTPConfig(t, repo)

	// now read
	configs, err := repo.GetFTPConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].Hostname != "ftp.moov.io" {
		t.Errorf("got %q", configs[0].Hostname)
	}
	if configs[0].Username != "moov" {
		t.Errorf("got %q", configs[0].Username)
	}
	if configs[0].Password != "secret" {
		t.Errorf("got %q", configs[0].Password)
	}
}

func writeFileTransferConfig(t *testing.T, db *sql.DB) {
	t.Helper()

	query := `insert into file_transfer_configs (routing_number, inbound_path, outbound_path, return_path) values ('123456789', 'inbound/', 'outbound/', 'return/');`
	stmt, err := db.Prepare(query)
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err := stmt.Exec(); err != nil {
		t.Fatal(err)
	}
}

func TestsqliteRepository__GetConfigs(t *testing.T) {
	repo := createTestsqliteRepository(t)
	defer repo.Close()

	writeFileTransferConfig(t, repo.db)

	// now read
	configs, err := repo.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].InboundPath != "inbound/" {
		t.Errorf("got %q", configs[0].InboundPath)
	}
	if configs[0].OutboundPath != "outbound/" {
		t.Errorf("got %q", configs[0].OutboundPath)
	}
	if configs[0].ReturnPath != "return/" {
		t.Errorf("got %q", configs[0].ReturnPath)
	}
}

func TestMySQLFileTransferRepository(t *testing.T) {
	testdb := database.CreateTestMySQLDB(t)

	repo := NewRepository(testdb.DB, "mysql")
	if _, ok := repo.(*sqliteRepository); !ok {
		t.Fatalf("got %T", repo)
	}
	writeFileTransferConfig(t, testdb.DB)

	configs, err := repo.GetConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Errorf("len(configs)=%d", len(configs))
	}
	if configs[0].RoutingNumber != "123456789" {
		t.Errorf("got %q", configs[0].RoutingNumber)
	}
	if configs[0].InboundPath != "inbound/" {
		t.Errorf("got %q", configs[0].InboundPath)
	}
	if configs[0].OutboundPath != "outbound/" {
		t.Errorf("got %q", configs[0].OutboundPath)
	}
	if configs[0].ReturnPath != "return/" {
		t.Errorf("got %q", configs[0].ReturnPath)
	}

	testdb.Close()
}

func TestFileTransferConfigs__maskPassword(t *testing.T) {
	if v := maskPassword(""); v != "**" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("12"); v != "**" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("123"); v != "1*3" {
		t.Errorf("got %q", v)
	}
	if v := maskPassword("password"); v != "p******d" {
		t.Errorf("got %q", v)
	}

	out := maskPasswords([]*FTPConfig{{Password: "password"}})
	if len(out) != 1 {
		t.Errorf("got %d ftpConfigs: %v", len(out), out)
	}
	if out[0].Password != "p******d" {
		t.Errorf("got %q", out[0].Password)
	}
}

func TestFileTransferConfigsHTTP__GetConfigs(t *testing.T) {
	svc := admin.NewServer(":0")
	go svc.Listen()
	defer svc.Shutdown()

	repo := &localFileTransferRepository{}

	AddFileTransferConfigRoutes(log.NewNopLogger(), svc, repo)

	req, err := http.NewRequest("GET", "http://localhost"+svc.BindAddr()+"/configs/uploads", nil) // need moov-io/base update
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-user-id", "userId")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("bogus HTTP status: %d", resp.StatusCode)
	}
	var response adminConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.CutoffTimes) == 0 || len(response.FTPConfigs) == 0 || len(response.FileTransferConfigs) == 0 {
		t.Errorf("response.CutoffTimes=%d response.FTPConfigs=%d response.FileTransferConfigs=%d", len(response.CutoffTimes), len(response.FTPConfigs), len(response.FileTransferConfigs))
	}
}

// svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", upsertCutoffTimeConfig(logger, repo))
// svc.AddHandler("/configs/uploads/cutoff-times/{routingNumber}", deleteCutoffTimeConfig(logger, repo))

// svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", upsertFileTransferConfig(logger, repo))
// svc.AddHandler("/configs/uploads/file-transfers/{routingNumber}", deleteFileTransferConfig(logger, repo))

// svc.AddHandler("/configs/uploads/ftp/{routingNumber}", upsertFTPConfig(logger, repo))
// svc.AddHandler("/configs/uploads/ftp/{routingNumber}", deleteFTPConfig(logger, repo))
