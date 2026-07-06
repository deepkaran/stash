package loader

import (
	"strings"

	"gsinutshell/model"
)

// parseStatsLog reads indexer_stats.log. Each data line is:
//
//	<timestamp> <key> <json>
//
// where <key> is "indexer" (node), "index_<...>" (per-index) or
// "indexstorage_<...>" (per-index plasma storage). Other keys (projlat, etc.)
// are ignored. Payloads are kept raw and parsed lazily so we only decode the
// last-N retained samples, not all ~268k lines.
func parseStatsLog(path string, m *model.Model) error {
	f, sc, err := openScanner(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 || line[0] < '0' || line[0] > '9' {
			continue // not a timestamped data line
		}
		// Format: "<ts> <key> <json>". The key may itself contain spaces for
		// replica indexes (e.g. "index_b:idx (replica 1):<instId>"), so the
		// payload boundary is the first '{', not the second space.
		sp1 := strings.IndexByte(line, ' ')
		if sp1 < 0 {
			continue
		}
		ts := line[:sp1]
		rest := line[sp1+1:]
		brace := strings.IndexByte(rest, '{')
		if brace < 0 {
			continue
		}
		key := strings.TrimSpace(rest[:brace])
		payload := rest[brace:]

		switch {
		case key == "indexer":
			pushRaw(m.Node, ts, payload)
		case strings.HasPrefix(key, "indexstorage_"):
			name := strings.TrimPrefix(key, "indexstorage_")
			pushRaw(m.StorageSeries(name), ts, payload)
		case strings.HasPrefix(key, "index_"):
			name := strings.TrimPrefix(key, "index_")
			pushRaw(m.IndexSeries(name), ts, payload)
		}
	}
	return sc.Err()
}

// pushRaw parses the payload and pushes onto the bounded series. We parse at
// push time for simplicity; the series only retains the last-N points so peak
// memory stays bounded regardless of file size.
func pushRaw(s *model.Series, ts, payload string) {
	sample, err := model.ParseSample([]byte(payload))
	if err != nil {
		return // tolerate malformed lines
	}
	s.Push(model.TSPoint{Ts: ts, Data: sample})
}
