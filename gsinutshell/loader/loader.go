package loader

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"gsinutshell/model"
)

// Default retained samples per series ("last 10 samples" per the spec).
const DefaultWindow = 10

// File name candidates within a cbcollect directory. cbcollect prefixes the
// ns_server-captured copies with "ns_server."; we accept either.
var (
	statsLogNames   = []string{"ns_server.indexer_stats.log", "indexer_stats.log"}
	indexerLogNames = []string{"ns_server.indexer.log", "indexer.log"}
	couchbaseNames  = []string{"couchbase.log"}
)

// Load parses a cbcollect directory into a Model. Missing files are skipped
// (a partial collect still yields a partial report).
func Load(dir string, window int) (*model.Model, error) {
	if window <= 0 {
		window = DefaultWindow
	}
	m := model.NewModel(dir, window)

	if p := findFirst(dir, statsLogNames); p != "" {
		if err := parseStatsLog(p, m); err != nil {
			return nil, err
		}
	}
	if p := findFirst(dir, indexerLogNames); p != "" {
		if err := parseIndexerLog(p, m); err != nil {
			return nil, err
		}
	}
	if p := findFirst(dir, couchbaseNames); p != "" {
		if err := parseCouchbaseLog(p, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func findFirst(dir string, names []string) string {
	for _, n := range names {
		p := filepath.Join(dir, n)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// openScanner returns a line scanner with a large buffer; stat lines can be
// hundreds of KB (the indexstorage_ payloads especially).
func openScanner(path string) (*os.File, *bufio.Scanner, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20) // up to 16 MB per line
	return f, sc, nil
}

func firstToken(line string) string {
	if i := strings.IndexByte(line, ' '); i >= 0 {
		return line[:i]
	}
	return line
}
