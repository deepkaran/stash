package loader

import (
	"strconv"
	"strings"

	"gsinutshell/model"
)

const (
	memstatsTag  = "[Info] memstats "
	periodicAggT = "[Info] Periodic Aggregated StorageStats:"
)

// parseIndexerLog scans indexer.log for the two periodic structured payloads we
// need in Phase 1:
//
//   - "[Info] memstats {...}" — single-line, valid-JSON Go runtime memstats.
//   - "[Info] Periodic Aggregated StorageStats:" followed by a multi-line,
//     pretty-printed *pseudo*-JSON block (plasma aggregate incl. assigned_quota
//     / current_quota). Logged ~every 10 minutes. NOTE: this block is not valid
//     JSON — it contains empty values and "N (delta)" forms — so it is parsed
//     line-by-line rather than with encoding/json.
func parseIndexerLog(path string, m *model.Model) error {
	f, sc, err := openScanner(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for sc.Scan() {
		line := sc.Text()

		if i := strings.Index(line, memstatsTag); i >= 0 {
			ts := firstToken(line)
			payload := strings.TrimSpace(line[i+len(memstatsTag):])
			if strings.HasPrefix(payload, "{") {
				pushRaw(m.Mem, ts, payload)
			}
			continue
		}

		if strings.Contains(line, periodicAggT) {
			ts := firstToken(line)
			if sample, ok := readAggBlock(sc); ok {
				m.PlasmaAgg.Push(model.TSPoint{Ts: ts, Data: sample})
			}
		}
	}
	return sc.Err()
}

type scanner interface {
	Scan() bool
	Text() string
}

// readAggBlock consumes the brace-delimited pseudo-JSON block following a
// "Periodic Aggregated StorageStats:" marker and returns it as a Sample. Brace
// depth (computed only from structural braces, ignoring those inside quoted
// strings) determines the block boundary; each interior line is parsed into a
// key/value with tolerant rules.
func readAggBlock(sc scanner) (model.Sample, bool) {
	out := model.Sample{}
	depth := 0
	started := false

	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if !started {
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "{") {
				return nil, false
			}
			started = true
		}

		depth += braceDelta(line)

		if k, v, ok := parseAggLine(trimmed); ok {
			out[k] = v
		}
		if depth <= 0 {
			return out, len(out) > 0
		}
	}
	return out, len(out) > 0
}

// braceDelta counts net structural braces on a line, ignoring those inside
// double-quoted strings.
func braceDelta(line string) int {
	d, inStr, esc := 0, false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			d++
		case '}':
			d--
		}
	}
	return d
}

// parseAggLine parses one `"key":   value,` line. Values may be empty,
// quoted strings, plain numbers, or "number (delta)" forms; for the last we
// keep the leading numeric token. Returns ok=false for non key/value lines
// (e.g. the opening "{" or closing "}").
func parseAggLine(line string) (string, any, bool) {
	if !strings.HasPrefix(line, `"`) {
		return "", nil, false
	}
	q := strings.IndexByte(line[1:], '"')
	if q < 0 {
		return "", nil, false
	}
	key := strings.TrimSpace(line[1 : 1+q])
	rest := line[1+q+1:]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return "", nil, false
	}
	val := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest[colon+1:]), ","))
	if val == "" {
		return key, nil, true // present but empty
	}
	if strings.HasPrefix(val, `"`) {
		return key, strings.Trim(val, `"`), true
	}
	tok := val
	if sp := strings.IndexByte(val, ' '); sp >= 0 {
		tok = val[:sp]
	}
	if f, err := strconv.ParseFloat(tok, 64); err == nil {
		return key, f, true
	}
	return key, val, true
}
