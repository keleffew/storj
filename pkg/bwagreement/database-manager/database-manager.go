// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dbmanager

import (
	"context"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/internal/migrate"
	dbx "storj.io/storj/pkg/bwagreement/database-manager/dbx"
	"storj.io/storj/pkg/pb"
)

// Error is a standard error class for this package.
var (
	Error = errs.Class("bwagreement db error")
	mon   = monkit.Package()
)

//DBManager is an implementation of the database access interface
type DBManager struct {
	DB *dbx.DB
}

var db *DBManager
var once sync.Once

// NewDBManager creates a new instance of a DatabaseManager
// except we're using a singleton pattern here, because SQLite
// will throw lots of "database is locked" errors if we don't
// let a single db driver instance handle concurrency
func NewDBManager(driver, source string) (*DBManager, error) {
	var err error
	once.Do(func() {
		sqlDB, err := dbx.Open(driver, source)
		if err != nil {
			return
		}
		err = migrate.Create("bwagreement", sqlDB)
		if err != nil {
			return
		}
		db = &DBManager{DB: sqlDB}
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return db, nil
}

// Create a db entry for the provided storagenode
func (dbm *DBManager) Create(ctx context.Context, createBwAgreement *pb.RenterBandwidthAllocation) (bwagreement *dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)

	signature := createBwAgreement.GetSignature()
	data := createBwAgreement.GetData()

	bwagreement, err = dbm.DB.Create_Bwagreement(
		ctx,
		dbx.Bwagreement_Signature(signature),
		dbx.Bwagreement_Data(data),
	)
	if err != nil {
		return nil, Error.Wrap(status.Errorf(codes.Internal, err.Error()))
	}
	return bwagreement, nil
}

// GetBandwidthAllocations all bandwidth agreements and sorts by satellite
func (dbm *DBManager) GetBandwidthAllocations(ctx context.Context) (rows []*dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	rows, err = dbm.DB.All_Bwagreement(ctx)
	return rows, Error.Wrap(err)
}

// GetBandwidthAllocationsSince all bandwidth agreements created since a time
func (dbm *DBManager) GetBandwidthAllocationsSince(ctx context.Context, since time.Time) (rows []*dbx.Bwagreement, err error) {
	defer mon.Task()(&ctx)(&err)
	rows, err = dbm.DB.All_Bwagreement_By_CreatedAt_Greater(ctx, dbx.Bwagreement_CreatedAt(since))
	return rows, err
}
