package loader

import (
	"strconv"
	"strings"
	"time"

	"gsinutshell/model"
)

// Event class keys live in the model package (shared with analyze); aliased
// here for brevity.
const (
	EvRestart    = model.EvRestart
	EvCrash      = model.EvCrash
	EvRollback   = model.EvRollback
	EvFlushMon   = model.EvFlushMon
	EvSlowOp     = model.EvSlowOp
	EvMemtuner   = model.EvMemtuner
	EvCommErr    = model.EvCommErr
	EvNonAlignTS = model.EvNonAlignTS
	EvRepair     = model.EvRepair
)

// scanEvent inspects one indexer.log line and records any matching event.
// Called for every line of a multi-hundred-MB file, so it must stay cheap: it
// routes on the O(1) "[Level]" prefix (which sits at a fixed offset after the
// timestamp) and only runs the substring tests relevant to that level. The
// dominant [Info] LSS/PlasmaSlice lines thus cost just a couple of checks.
func scanEvent(ts, line string, m *model.Model) {
	// Go runtime panics/fatals are emitted without a "[Level]" tag.
	if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "fatal error:") {
		m.AddEvent(EvCrash, ts, 0, "")
		return
	}
	if len(line) < len(ts)+2 {
		return
	}
	rest := line[len(ts)+1:] // "[Info] ..." etc.

	switch {
	case strings.HasPrefix(rest, "[Info]"):
		switch {
		case strings.Contains(rest, "Adaptive memory quota tuning") && strings.Contains(rest, "decrement"):
			m.AddEvent(EvMemtuner, ts, 0, "")
		case strings.Contains(rest, "Indexer::indexer version") || strings.Contains(rest, "Indexer started with command line"):
			m.AddEvent(EvRestart, ts, 0, "")
		case anyContains(rest, rollbackAnchors):
			m.AddEvent(EvRollback, ts, 0, "")
		}
	case strings.HasPrefix(rest, "[Warn]"):
		switch {
		case strings.Contains(rest, "flushMonitor Waiting for flush to finish"):
			mag, d := extractFlushSecs(rest)
			m.AddEvent(EvFlushMon, ts, mag, d)
		case strings.Contains(rest, "traceLockLog: Long hold") || strings.Contains(rest, "Slow operation"):
			mag, d := extractSlowDur(rest)
			m.AddEvent(EvSlowOp, ts, mag, d)
		case anyContains(rest, rollbackAnchors):
			m.AddEvent(EvRollback, ts, 0, "")
		case anyContains(rest, nonAlignAnchors):
			m.AddEvent(EvNonAlignTS, ts, 0, "")
		case anyContains(rest, repairAnchors):
			m.AddEvent(EvRepair, ts, 0, "")
		}
	case strings.HasPrefix(rest, "[Error]"):
		switch {
		case anyContains(rest, commErrAnchors):
			m.AddEvent(EvCommErr, ts, 0, "")
		case anyContains(rest, repairAnchors):
			m.AddEvent(EvRepair, ts, 0, "")
		}
	case strings.HasPrefix(rest, "[Fatal]"):
		m.AddEvent(EvCrash, ts, 0, "")
	}
}

var (
	rollbackAnchors = []string{"rollback", "Rollback"}
	nonAlignAnchors = []string{"nonalign", "NonAlign", "non-aligned"}
	repairAnchors   = []string{"stream repair", "StreamRepair", "repairMissingStreamBegin", "RepairStream"}
	commErrAnchors  = []string{"transport -", "PeerPipe.", "PeerListener.", "DATP[->dataport"}
)

func anyContains(l string, subs []string) bool {
	for _, s := range subs {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// extractFlushSecs pulls N from "...finish for N seconds...".
func extractFlushSecs(line string) (float64, string) {
	const a = "finish for "
	i := strings.Index(line, a)
	if i < 0 {
		return 0, ""
	}
	rest := line[i+len(a):]
	j := strings.Index(rest, " seconds")
	if j < 0 {
		return 0, ""
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(rest[:j]), 64)
	if err != nil {
		return 0, ""
	}
	// Context: the "Stream X KeyspaceId Y" suffix if present.
	detail := ""
	if k := strings.Index(rest, "Stream "); k >= 0 {
		detail = strings.TrimRight(rest[k:], ". \t")
	}
	return n, detail
}

// extractSlowDur pulls the Go duration and call site from
// "...Long hold of <lock> <dur> in <where>", returning the duration in ms.
func extractSlowDur(line string) (float64, string) {
	i := strings.Index(line, "Long hold of ")
	if i < 0 {
		return 0, ""
	}
	fields := strings.Fields(line[i+len("Long hold of "):]) // [lock, dur, "in", where...]
	if len(fields) < 2 {
		return 0, ""
	}
	d, err := time.ParseDuration(fields[1])
	if err != nil {
		return 0, ""
	}
	detail := ""
	if len(fields) >= 4 && fields[2] == "in" {
		detail = strings.Join(fields[3:], " ")
	}
	return float64(d.Milliseconds()), detail
}
