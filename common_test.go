// Copyright 2018 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"math"
	"math/big"
	"testing"
)

func TestAccountType(t *testing.T) {
	var tpe AccountType
	if !tpe.empty() {
		t.Error("expected empty")
	}
}

func TestAccountType__json(t *testing.T) {
	at := Checking

	// marshal
	bs, err := json.Marshal(&at)
	if err != nil {
		t.Fatal(err.Error())
	}
	if s := string(bs); s != `"Checking"` {
		t.Errorf("got %q", s)
	}

	// unmarshal
	raw := []byte(`"Savings"`)
	if err := json.Unmarshal(raw, &at); err != nil {
		t.Error(err.Error())
	}
	if at != Savings {
		t.Errorf("got %s", at)
	}
}

func TestAmount(t *testing.T) {
	// happy path
	amt, err := NewAmount("USD", "12.00")
	if err != nil {
		t.Error(err)
	}
	if v := amt.String(); v != "USD 12.00" {
		t.Errorf("got %q", v)
	}

	// invalid
	_, err = NewAmount("", ".0")
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestAmount__FromString(t *testing.T) {
	amt := Amount{}
	if err := amt.FromString("fail"); err == nil {
		t.Error("exected error")
	}
	if err := amt.FromString("USD 12.00"); err != nil {
		t.Error(err)
	}
}

func TestAmount__json(t *testing.T) {
	// valid
	raw := []byte(`"USD 12.03"`)
	amt := Amount{}
	if err := json.Unmarshal(raw, &amt); err != nil {
		t.Error(err.Error())
	}
	if amt.symbol != "USD" {
		t.Errorf("got %s", amt.symbol)
	}
	v, _ := amt.number.Float64()
	if n := math.Abs(12.03 - v); n > 0.1 {
		t.Errorf("v=%.2f, n=%.2f", v, n)
	}

	// valid, but no fractional amount
	n := big.NewRat(12, 1) // 12/1 = 12.00
	bs, err := json.Marshal(Amount{n, "USD"})
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != `"USD 12.00"` {
		t.Errorf("got %q", v)
	}

	// round away extra precision
	n = big.NewRat(3, 1000) // 3/1000 = 0.003 (rounds to 0.00)
	bs, err = json.Marshal(Amount{n, "USD"})
	if err != nil {
		t.Error(err)
	}
	if v := string(bs); v != `"USD 0.00"` {
		t.Errorf("got %q", v)
	}

	// invalid
	in := []byte(`"other thing"`)
	if err := json.Unmarshal(in, &amt); err == nil {
		t.Errorf("expected error")
	}
}
