package runtime

import (
	"github.com/vorsar/cep/internal/clock"
	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
	"github.com/vorsar/cep/internal/plan"
	"github.com/vorsar/cep/internal/table"
)

type itemKind uint8

const (
	iEvent  itemKind = iota + 1
	iReject          // a payload that failed its schema or the format's limits (owner Q1 of S3.2b-2)
	iTick
	iWrite
	iDrain
	iStats
	iAdd      // design s3-3-1 §2.3: register an app, inactive
	iRemove   // unregister a drained or inactive app
	iQuiet    // the ticks stop for the app (S3.1-1's D3)
	iBatchEnd // a source's batch-close signal
	iActivate // design s3-3-1 §2.4 as fixed: the router decides the activation in intake order (review B1)
)

type item struct {
	kind    itemKind
	app     AppID
	ev      event.Event
	info    event.Decoded      // iEvent from SendPayload: its decode report
	payload []byte             // iEvent from SendPayload with a recording: the payload as sent, cleaned; the item's own
	stream  int                // iReject
	fault   *event.DecodeError // iReject
	now     int64
	table   int
	op      TableOp
	rows    [][]event.Value
	reply   chan reply // buffered 1: the router never blocks on it
	add     *App       // iAdd
	batch   string     // iBatchEnd
}

type reply struct {
	err     error
	id      AppID // iAdd's
	pending int64
	markers []chan struct{}
	// a drain's recording stop, or an ended recording's queued records, waited for only until the drain's deadline (owner,
	// 2026-11-06)
	recMarker chan struct{}
	stats     Stats
}

type msgKind uint8

const (
	mWalk msgKind = iota + 1 // advances, and an event's offers
	mDrain
	mStats
	mAdd    // design s3-3-1 §2.3: build the app's slot on this shard; acked
	mRemove // free it; acked
)

type message struct {
	kind      msgKind
	app       AppID
	adv       []clock.WMAdvance
	tables    *table.Snapshot
	ev        event.Event
	offers    []offer
	token     uint64
	admStream int   // an event message: its stream, whose admission watermark is adm (owner D1 of S3.2b-1); -1: none
	adm       int64 // what an on-arrival rule's clock takes, in place of the stream's advances
	final     bool  // a drain or the end: on-arrival rules take the advances too, as nothing arrives after them
	add       *App  // mAdd
}

type offer struct {
	rule int
	key  partition.Key // Rest fresh per event, read-only (design s3-2b-2 §2.6)
}

type outKind uint8

const (
	oEvent outKind = iota + 1
	oAck
	oStats
)

type outItem struct {
	kind    outKind
	app     AppID
	ev      event.Event
	token   uint64
	pending int64
	stats   *shardStats
	err     error // oAck of an mAdd: the shard's build error
}

type shardStats struct {
	outHigh [][]int64           // [app][rule]: the most outputs one message made the rule emit
	rules   [][]partition.Stats // [app][rule]
	emits   [][]plan.Stats      // [app][rule]: each body's plan.Counted, zero for a body that is not
	panics  [][]int64           // [app][rule]: the shard boundary's last-resort recoveries
	parks   int64
}

type handoff struct {
	shard int
	items []outItem
}
