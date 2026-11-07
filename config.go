package cep

import (
	"time"

	"github.com/vorsar/cep/internal/runtime"
)

// Config is the runtime's configuration; zero values take the engine's defaults.
type Config struct {
	Shards      int              // 0: GOMAXPROCS
	Mailbox     int              // per shard, in messages; 0: 1024
	Intake      int              // the router's intake queue; 0: 1024
	SinkQueue   int              // per sink binding; 0: 1024
	RecordQueue int              // each recording's queue; 0: 65,536
	TickEvery   time.Duration    // live idle ticks; 0: none
	Now         func() time.Time // the wall clock; nil: the engine's default
}

// engine converts to the engine's configuration; a nil Now stays nil (the engine's default).
func (c Config) engine() runtime.Config {
	rc := runtime.Config{Shards: c.Shards, Mailbox: c.Mailbox, Intake: c.Intake, SinkQueue: c.SinkQueue,
		TickEvery: c.TickEvery, RecordQueue: c.RecordQueue}
	if c.Now != nil {
		now := c.Now
		rc.Now = func() int64 { return now().UnixMilli() }
	}
	return rc
}
