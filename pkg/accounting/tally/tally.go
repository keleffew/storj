// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"context"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
	"storj.io/storj/pkg/accounting"
	dbx "storj.io/storj/pkg/accounting/dbx"
	dbManager "storj.io/storj/pkg/bwagreement/database-manager"
	bwDbx "storj.io/storj/pkg/bwagreement/database-manager/dbx"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// Tally is the service for accounting for data stored on each storage node
type Tally interface {
	Run(ctx context.Context) error
}

type tally struct {
	pointerdb  *pointerdb.Server
	overlay    pb.OverlayServer
	kademlia   *kademlia.Kademlia
	limit      int
	logger     *zap.Logger
	ticker     *time.Ticker
	nodes      map[string]int64
	nodeClient node.Client
	DB         *dbx.DB
}

func newTally(ctx context.Context, pointerdb *pointerdb.Server, overlay pb.OverlayServer, kademlia *kademlia.Kademlia, limit int, logger *zap.Logger, interval time.Duration) *tally {
	rt, err := kademlia.GetRoutingTable(ctx)
	if err != nil {
		// return Error.Wrap(err)
	}
	self := rt.Local()
	identity := &provider.FullIdentity{} //do i need anything in here?
	client, err := node.NewNodeClient(identity, self, kademlia)
	if err != nil {
		// return Error.Wrap(err)
	}
	return &tally{
		pointerdb:  pointerdb,
		overlay:    overlay,
		kademlia:   kademlia,
		limit:      limit,
		logger:     logger,
		ticker:     time.NewTicker(interval),
		nodes:      make(map[string]int64),
		nodeClient: client,
		//TODO:
		//accountingDBServer
	}
}

// Run the tally loop
func (t *tally) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	for {
		err = t.identifyActiveNodes(ctx)
		if err != nil {
			zap.L().Error("Tally failed", zap.Error(err))
		}

		select {
		case <-t.ticker.C: // wait for the next interval to happen
		case <-ctx.Done(): // or the tally is canceled via context
			_ = t.db.Close()
			return ctx.Err()
		}
	}
}

// identifyActiveNodes iterates through pointerdb and identifies nodes that have storage on them
func (t *tally) identifyActiveNodes(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	t.logger.Debug("entering pointerdb iterate")
	err = t.pointerdb.Iterate(ctx, &pb.IterateRequest{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			lim := t.limit
			if lim <= 0 || lim > storage.LookupLimit {
				lim = storage.LookupLimit
			}
			for ; lim > 0 && it.Next(&item); lim-- {
				pointer := &pb.Pointer{}
				err = proto.Unmarshal(item.Value, pointer)
				if err != nil {
					return Error.Wrap(err)
				}
				pieces := pointer.Remote.RemotePieces
				var nodeIDs storj.NodeIDList
				for _, p := range pieces {
					nodeIDs = append(nodeIDs, p.NodeId)
				}
				online, err := t.onlineNodes(ctx, nodeIDs)
				if err != nil {
					return Error.Wrap(err)
				}
				err = t.tallyAtRestStorage(ctx, pointer, online)
				if err != nil {
					return Error.Wrap(err)
				}
			}
			return nil
		},
	)
	return t.updateRawTable()
}

func (t *tally) onlineNodes(ctx context.Context, nodeIDs storj.NodeIDList) (online []*pb.Node, err error) {
	responses, err := t.overlay.BulkLookup(ctx, pb.NodeIDsToLookupRequests(nodeIDs))
	if err != nil {
		return []*pb.Node{}, err
	}
	nodes := pb.LookupResponsesToNodes(responses)
	for _, n := range nodes {
		if n != nil {
			online = append(online, n)
		}
	}
	return online, nil
}

func (t *tally) tallyAtRestStorage(ctx context.Context, pointer *pb.Pointer, nodes []*pb.Node) error {
	segmentSize := pointer.GetSegmentSize()
	minReq := pointer.Remote.Redundancy.GetMinReq()
	if minReq <= 0 {
		return Error.New("minReq must be an int greater than 0")
	}
	pieceSize := segmentSize / int64(minReq)
	for _, n := range nodes {
		ok, err := t.nodeClient.Ping(ctx, *n)
		if ok {
			t.nodes[n.Id] = pieceSize
		}
		if !ok || err != nil {
			zap.L().Error("ping failed for node: " + n.Id)
			continue
		}
	}
	return nil
}

func (t *tally) updateRawTable() error {
	//TODO
	//takes in-memory map of ok nodes
	//adds all and sets all other nodes to 0
	return nil
}

// Query bandwidth allocation database, selecting all new contracts since the last collection run time.
// Grouping by storage node ID and adding total of bandwidth to granular data table.
func (t *tally) Query(ctx context.Context) error {
	lastBwTally, err := t.db.Find_Timestamps_Value_By_Name(ctx, accounting.LastBandwidthTally)
	if err != nil {
		return err
	}
	var bwAgreements []*bwDbx.Bwagreement
	if lastBwTally == nil {
		t.logger.Info("Tally found no existing bandwith tracking data")
		bwAgreements, err = t.dbm.GetBandwidthAllocations(ctx)
	} else {
		bwAgreements, err = t.dbm.GetBandwidthAllocationsSince(ctx, lastBwTally.Value)
	}
	if len(bwAgreements) == 0 {
		t.logger.Info("Tally found no new bandwidth allocations")
		return nil
	}

	// sum totals by node id ... todo: add nodeid as SQL column so DB can do this?
	bwTotals := make(map[string]int64)
	var latestBwa time.Time
	for _, baRow := range bwAgreements {
		rbad := &pb.RenterBandwidthAllocation_Data{}
		if err := proto.Unmarshal(baRow.Data, rbad); err != nil {
			t.logger.DPanic("Could not deserialize renter bwa in tally query")
			continue
		}
		if baRow.CreatedAt.After(latestBwa) {
			latestBwa = baRow.CreatedAt
		}
		bwTotals[rbad.StorageNodeId.String()] += rbad.GetTotal() // todo: check for overflow?
	}

	//todo:  consider if we actually need EndTime in granular
	if lastBwTally == nil {
		t.logger.Info("No previous bandwidth timestamp found in tally query")
		lastBwTally = &dbx.Value_Row{Value: latestBwa} //todo: something better here?
	}

	//insert all records in a transaction so if we fail, we don't have partial info stored
	//todo:  replace with a WithTx() method per DBX docs?
	tx, err := t.db.Open(ctx)
	if err != nil {
		t.logger.DPanic("Failed to create DB txn in tally query")
		return err
	}
	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			t.logger.Warn("DB txn was rolled back in tally query")
			err = tx.Rollback()
		}
	}()

	//todo:  switch to bulk update SQL?
	for k, v := range bwTotals {
		nID := dbx.Granular_NodeId(k)
		start := dbx.Granular_StartTime(lastBwTally.Value)
		end := dbx.Granular_EndTime(latestBwa)
		total := dbx.Granular_DataTotal(v)
		_, err = tx.Create_Granular(ctx, nID, start, end, total)
		if err != nil {
			t.logger.DPanic("Create granular SQL failed in tally query")
			return err //todo: retry strategy?
		}
	}

	//todo:  move this into txn when we have masterdb?
	update := dbx.Timestamps_Update_Fields{Value: dbx.Timestamps_Value(latestBwa)}
	_, err = tx.Update_Timestamps_By_Name(ctx, accounting.LastBandwidthTally, update)
	if err != nil {
		t.logger.DPanic("Failed to update bandwith timestamp in tally query")
	}
	return err
}
