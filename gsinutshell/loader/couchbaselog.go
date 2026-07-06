package loader

import (
	"encoding/json"
	"strings"

	"gsinutshell/model"
)

// Section markers in couchbase.log. Each block has the shape:
//
//	<marker>
//	curl -X GET ... <endpoint>
//	====================================== (separator)
//	<payload, single or multi-line>
//	====================================== (separator)
const (
	mkIndexDefs = "Index definitions are:"
	mkSettings  = "Indexer settings are:"
	mkStats     = "Indexer stats are:"
	mkStorage   = "Index storage stats are:"
)

func isSep(line string) bool { return strings.HasPrefix(line, "====") }

// parseCouchbaseLog extracts the four point-in-time index sections.
func parseCouchbaseLog(path string, m *model.Model) error {
	f, sc, err := openScanner(path)
	if err != nil {
		return err
	}
	defer f.Close()

	const (
		stIdle    = iota // looking for a marker
		stWaitSep        // marker seen, skip curl line until first separator
		stPayload        // accumulate until next separator
	)

	state := stIdle
	var section string
	var buf strings.Builder

	finish := func() {
		routeSection(section, buf.String(), m)
		buf.Reset()
		section = ""
		state = stIdle
	}

	for sc.Scan() {
		line := sc.Text()
		switch state {
		case stIdle:
			if s := matchMarker(line); s != "" {
				section = s
				state = stWaitSep
			}
		case stWaitSep:
			if isSep(line) {
				state = stPayload
			}
		case stPayload:
			if isSep(line) {
				finish()
				continue
			}
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if state == stPayload {
		finish() // file ended mid-block
	}
	return sc.Err()
}

func matchMarker(line string) string {
	t := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(t, mkIndexDefs):
		return mkIndexDefs
	case strings.HasPrefix(t, mkSettings):
		return mkSettings
	case strings.HasPrefix(t, mkStats):
		return mkStats
	case strings.HasPrefix(t, mkStorage):
		return mkStorage
	}
	return ""
}

func routeSection(section, payload string, m *model.Model) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return
	}
	switch section {
	case mkSettings:
		if s, err := model.ParseSample([]byte(payload)); err == nil {
			m.Snapshot.Settings = s
		}
	case mkStats:
		if s, err := model.ParseSample([]byte(payload)); err == nil {
			m.Snapshot.PartitionStats = s
		}
	case mkIndexDefs:
		// {"code":"success","status":[ ... ]}
		var env struct {
			Status []model.Sample `json:"status"`
		}
		if err := json.Unmarshal([]byte(payload), &env); err == nil {
			m.Snapshot.IndexStatus = env.Status
		}
	case mkStorage:
		var arr []model.Sample
		if err := json.Unmarshal([]byte(payload), &arr); err == nil {
			m.Snapshot.StorageStats = arr
		}
	}
}
