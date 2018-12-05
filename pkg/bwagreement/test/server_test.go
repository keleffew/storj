// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	dbManager "storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/pb"
)

var (
	ctx = context.Background()
)

func TestTwoSqliteFileDbManagers(t *testing.T) {
	dbx1, err := dbManager.NewDBManager("sqlite3", "file:accounting.db?cache=shared")
	assert.NoError(t, err)
	dbx2, err := dbManager.NewDBManager("sqlite3", "file:accounting.db?cache=shared")
	assert.Error(t, err)
	assert.NoError(t, dbx1.DB.Close())
	assert.NoError(t, dbx2.DB.Close())
}

func TestTwoSqliteMemDbManagers(t *testing.T) {
	dbx1, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory")
	assert.NoError(t, err)
	dbx2, err := dbManager.NewDBManager("sqlite3", "file::memory:?mode=memory")
	assert.NoError(t, err)
	assert.NoError(t, dbx1.DB.Close())
	assert.NoError(t, dbx2.DB.Close())
}
func TestBandwidthAgreements(t *testing.T) {
	TS := newTestServer(t)
	defer TS.stop()

	pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, TS.K)
	assert.NoError(t, err)

	rba, err := GenerateRenterBandwidthAllocation(pba, TS.K)
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	_, err = TS.C.BandwidthAgreements(ctx, rba)
	assert.NoError(t, err)
}
