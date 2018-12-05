// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"go.uber.org/zap"
	"storj.io/storj/pkg/accounting"
	dbManager "storj.io/storj/pkg/bwagreement/database-manager"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
)

// Config contains configurable values for tally
type Config struct {
	Interval    time.Duration `help:"how frequently tally should run" default:"30s"`
	DatabaseURL string        `help:"the database connection string to use" default:"sqlite3://$CONFDIR/accounting.db?cache=shared"`
}

// Initialize a tally struct
func (c Config) initialize(ctx context.Context) (Tally, error) {
	pointerdb := pointerdb.LoadFromContext(ctx)
	overlay := overlay.LoadServerFromContext(ctx)
	kademlia := kademlia.LoadFromContext(ctx)
	db, err := accounting.NewDb(c.DatabaseURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	driver, source, err := utils.SplitURL(c.DatabaseURL)
	if err != nil {
		return nil, err
	}
	zap.L().Warn("Pre tally NewDBManager : " + c.DatabaseURL)
	dbx, err := dbManager.NewDBManager(driver, source)
	if err != nil {
		return nil, err
	}
	zap.L().Warn("Post tally NewDBManager")
	return newTally(zap.L(), db, dbx, pointerdb, overlay, kademlia, 0, c.Interval), nil
}

// Run runs the tally with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	tally, err := c.initialize(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := tally.Run(ctx); err != nil {
			defer cancel()
			zap.L().Error("Error running tally", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
