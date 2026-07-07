package model

// Event class keys. Shared contract between the loader (which records them) and
// the analyze layer (which maps them to findings).
const (
	EvRestart    = "indexer_restart"    // indexer start/version banner
	EvCrash      = "crash"              // panic / fatal error
	EvRollback   = "rollback"           // storage rollback
	EvFlushMon   = "flush_monitor"      // timekeeper flush stall warning
	EvSlowOp     = "slow_op"            // long lock hold / slow op warning
	EvMemtuner   = "memtuner_decrement" // plasma adaptive quota decrement
	EvCommErr    = "comm_error"         // transport/peer/dataport errors
	EvNonAlignTS = "nonalign_ts"        // non-aligned timestamps (stream)
	EvRepair     = "stream_repair"      // timekeeper stream repair (stream)
)

// Event is an aggregated view of one class of log line scanned from
// indexer.log. Individual occurrences are collapsed into a count plus the
// first/last timestamp and the most significant occurrence, so memory stays
// bounded no matter how many times the event fires.
type Event struct {
	Key       string
	Count     int
	First     string  // timestamp of first occurrence
	Last      string  // timestamp of last occurrence
	Max       float64 // largest magnitude seen (unit is event-specific)
	MaxDetail string  // context for the Max occurrence
}

// AddEvent records one occurrence of an event, updating its aggregate.
func (m *Model) AddEvent(key, ts string, mag float64, detail string) {
	e := m.Events[key]
	if e == nil {
		e = &Event{Key: key, First: ts}
		m.Events[key] = e
	}
	e.Count++
	e.Last = ts
	if mag > e.Max {
		e.Max = mag
		e.MaxDetail = detail
	}
}
