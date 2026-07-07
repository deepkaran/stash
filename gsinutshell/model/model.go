package model

// TSPoint is one timestamped sample in a time series.
type TSPoint struct {
	Ts   string // raw RFC3339 timestamp string from the log line
	Data Sample
}

// FieldPoint is one observed value of a single field at a point in time.
type FieldPoint struct {
	Ts string
	V  any
}

// Series tracks one stat stream. The indexer publishes fields at differing
// cadences (rich fields like memory_quota appear in only a few "full" samples,
// while a reduced set is published every tick), so a fixed window of whole
// lines is the wrong unit. Instead we keep, per field, the last-N observations
// of *that field* — "last 10 samples" then means the last 10 samples that
// actually carried the field. We also keep a newest-wins merged view for
// point-in-time lookups.
type Series struct {
	cap    int
	count  int                     // total points pushed
	merged Sample                  // newest-wins value per key (whole stream)
	hist   map[string][]FieldPoint // per-field bounded history
}

func NewSeries(capacity int) *Series {
	return &Series{cap: capacity, merged: Sample{}, hist: map[string][]FieldPoint{}}
}

// Push records a sample: updates the merged view and appends each present
// field to its bounded per-field history. Nested objects are stored whole
// under their top-level key (deep accessors handle the nesting).
func (s *Series) Push(p TSPoint) {
	s.count++
	for k, v := range p.Data {
		s.merged[k] = v
		h := append(s.hist[k], FieldPoint{Ts: p.Ts, V: v})
		if len(h) > s.cap {
			h = h[len(h)-s.cap:]
		}
		s.hist[k] = h
	}
}

// Empty reports whether any sample was recorded.
func (s *Series) Empty() bool { return s == nil || s.count == 0 }

// Merged returns the newest-wins view across the whole stream.
func (s *Series) Merged() (Sample, bool) {
	if s.Empty() {
		return nil, false
	}
	return s.merged, true
}

// FieldHistory returns the last-N observations of one field (oldest first).
func (s *Series) FieldHistory(field string) []FieldPoint {
	if s == nil {
		return nil
	}
	return s.hist[field]
}

// Snapshot holds the point-in-time payloads captured in couchbase.log at the
// moment cbcollect ran. These are authoritative for topology/settings/detail.
type Snapshot struct {
	Settings       Sample   // /settings
	IndexStatus    []Sample // /getIndexStatus -> status[]
	PartitionStats Sample   // /stats?partition=true
	StorageStats   []Sample // /stats/storage -> array
}

// Model is the full parsed view of one node's cbcollect.
type Model struct {
	// Time series (bounded to last-N samples), from the .log files.
	Node      *Series            // indexer_stats.log "indexer" line
	Index     map[string]*Series // indexer_stats.log "index_*" lines
	Storage   map[string]*Series // indexer_stats.log "indexstorage_*" lines
	Mem       *Series            // indexer.log "memstats"
	PlasmaAgg *Series            // indexer.log "Periodic Aggregated StorageStats"

	// Point-in-time snapshot, from couchbase.log.
	Snapshot Snapshot

	// Events aggregates notable log lines scanned from indexer.log, keyed by
	// event class (see loader/events.go).
	Events map[string]*Event

	// SampleWindow is how many recent samples each series retains.
	SampleWindow int

	// Source is the cbcollect directory path.
	Source string
}

func NewModel(source string, window int) *Model {
	return &Model{
		Node:         NewSeries(window),
		Index:        map[string]*Series{},
		Storage:      map[string]*Series{},
		Mem:          NewSeries(window),
		PlasmaAgg:    NewSeries(window),
		Events:       map[string]*Event{},
		SampleWindow: window,
		Source:       source,
	}
}

// IndexSeries returns (creating if needed) the series for an index key.
func (m *Model) IndexSeries(key string) *Series {
	s, ok := m.Index[key]
	if !ok {
		s = NewSeries(m.SampleWindow)
		m.Index[key] = s
	}
	return s
}

// StorageSeries returns (creating if needed) the series for a storage key.
func (m *Model) StorageSeries(key string) *Series {
	s, ok := m.Storage[key]
	if !ok {
		s = NewSeries(m.SampleWindow)
		m.Storage[key] = s
	}
	return s
}
