package analyze

import (
	"fmt"
	"sort"
	"strings"

	"gsinutshell/model"
)

// Analyze runs all Phase-1 rules over the model and returns a Report.
func Analyze(m *model.Model) *Report {
	r := NewReport()
	sizingMemory(m, r)
	workload(m, r)
	indexOutliers(m, r)
	indexerOutliers(m, r)
	logEvents(m, r)
	topology(m, r)
	usage(m, r)
	return r
}

// last returns a merged newest-wins view of the series. The indexer publishes
// heterogeneous stat schemas, so merging the retained window is more robust
// than reading a single final sample.
func last(s *model.Series) (model.Sample, bool) {
	return s.Merged()
}

// ---- Indexer Node Health: Sizing/Memory --------------------------------

func sizingMemory(m *model.Model, r *Report) {
	node, ok := last(m.Node)
	if !ok {
		r.add(SecSizing, SevWarn, "Node stats", "no 'indexer' samples found in indexer_stats.log")
		return
	}
	storageMode, _ := node.Str("storage_mode")
	isPlasma := storageMode == "plasma"

	quota, hasQuota := node.Int("memory_quota")
	if hasQuota {
		r.add(SecSizing, SevInfo, "Indexer memory quota", humanBytes(quota))
	}

	// RSS over the retained window, flag any sample above quota.
	if vals := windowInts(m.Node, "memory_rss"); len(vals) > 0 {
		maxRSS := maxInt(vals)
		sev := SevInfo
		note := ""
		if hasQuota && maxRSS > quota {
			sev = SevFlag
			note = " — exceeds memory quota"
		}
		r.add(SecSizing, sev, "Memory RSS (last samples)",
			fmt.Sprintf("last=%s max=%s%s", humanBytes(vals[len(vals)-1]), humanBytes(maxRSS), note))
	}

	// Go heap from memstats.
	if mem, ok := last(m.Mem); ok {
		hi, _ := mem.Int("HeapInuse")
		hidle, _ := mem.Int("HeapIdle")
		hrel, _ := mem.Int("HeapReleased")
		sev := SevInfo
		note := ""
		if isPlasma && hasQuota && quota > 0 && pct(hi, quota) > HeapInusePctOfQuota {
			sev = SevFlag
			note = fmt.Sprintf(" — HeapInuse is %.1f%% of quota (>%d%%)", pct(hi, quota), HeapInusePctOfQuota)
		}
		r.add(SecSizing, sev, "Go heap",
			fmt.Sprintf("HeapInuse=%s HeapIdle=%s HeapReleased=%s%s",
				humanBytes(hi), humanBytes(hidle), humanBytes(hrel), note))
	}

	// Average resident percent.
	if v, ok := node.Int("avg_resident_percent"); ok {
		sev := SevInfo
		if v < ResidentPercentFloor {
			sev = SevFlag
		}
		r.add(SecSizing, sev, "Avg resident percent",
			fmt.Sprintf("%d%% (floor %d%%)", v, ResidentPercentFloor))
	}

	if v, ok := node.Int("memory_used_storage"); ok {
		r.add(SecSizing, SevInfo, "Memory used (storage)", humanBytes(v))
	}
	if v, ok := node.Int("memory_total_storage"); ok {
		r.add(SecSizing, SevInfo, "Memory total (storage incl. jemalloc)", humanBytes(v))
	}
	if v, ok := node.Int("total_data_size"); ok {
		r.add(SecSizing, SevInfo, "Total index data size", humanBytes(v))
	}
	if v, ok := node.Int("total_disk_size"); ok {
		r.add(SecSizing, SevInfo, "Total index disk size", humanBytes(v))
	}

	// Plasma assigned/current quota from Periodic Aggregated StorageStats.
	if agg, ok := last(m.PlasmaAgg); ok {
		assigned, hasA := agg.Int("assigned_quota")
		current, hasC := agg.Int("current_quota")
		if hasA {
			r.add(SecSizing, SevInfo, "Plasma assigned quota", humanBytes(assigned))
		}
		if hasC {
			sev := SevInfo
			note := ""
			if hasA && assigned > 0 && pct(current, assigned) < PlasmaQuotaPctFloor {
				sev = SevFlag
				note = fmt.Sprintf(" — %.1f%% of assigned (<%d%%)", pct(current, assigned), PlasmaQuotaPctFloor)
			}
			r.add(SecSizing, sev, "Plasma current quota", humanBytes(current)+note)
		}
	}

	// Estimated resident memory: sum est_resident_mem across all storage
	// instances (MainStore + BackStore) from the couchbase.log /stats/storage
	// snapshot. 10% of this is roughly the memory needed for a 10% resident
	// ratio.
	if total, ok := sumEstResidentMem(m); ok {
		r.add(SecSizing, SevInfo, "Total est_resident_mem", humanBytes(total))
		r.add(SecSizing, SevInfo, "Plasma estimated resident memory @10%",
			fmt.Sprintf("%.2f GB", float64(total)/10/(1024*1024*1024)))
	}
}

// sumEstResidentMem totals est_resident_mem over every storage instance's
// MainStore and BackStore in the point-in-time /stats/storage snapshot.
func sumEstResidentMem(m *model.Model) (int64, bool) {
	var total int64
	found := false
	for _, e := range m.Snapshot.StorageStats {
		stats, ok := e.Sub("Stats")
		if !ok {
			continue
		}
		for _, store := range []string{"MainStore", "BackStore"} {
			if st, ok := stats.Sub(store); ok {
				if v, ok := st.Int("est_resident_mem"); ok {
					total += v
					found = true
				}
			}
		}
	}
	return total, found
}

// ---- Indexer Node Health: Workload -------------------------------------

func workload(m *model.Model, r *Report) {
	node, ok := last(m.Node)
	if !ok {
		return
	}
	cores, hasCores := node.Int("num_cpu_core")
	if hasCores {
		r.add(SecWorkload, SevInfo, "CPU cores assigned", fmt.Sprintf("%d", cores))
	}
	// cpu_utilization is summed across cores (100% per core). Normalise.
	if vals := windowFloats(m.Node, "cpu_utilization"); len(vals) > 0 && hasCores && cores > 0 {
		lastNorm := vals[len(vals)-1] / float64(cores)
		maxNorm := maxFloat(vals) / float64(cores)
		sev := SevInfo
		note := ""
		if maxNorm > CPUUtilPercentCeil {
			sev = SevFlag
			note = fmt.Sprintf(" — peak >%d%%", CPUUtilPercentCeil)
		}
		r.add(SecWorkload, sev, "CPU utilization (per-core, last samples)",
			fmt.Sprintf("last=%.0f%% peak=%.0f%%%s", lastNorm, maxNorm, note))
	}
	if v, ok := node.Int("avg_mutation_rate"); ok {
		r.add(SecWorkload, SevInfo, "Avg mutation rate", fmt.Sprintf("%d /s", v))
	}
	if v, ok := node.Int("avg_drain_rate"); ok {
		r.add(SecWorkload, SevInfo, "Avg drain rate", fmt.Sprintf("%d /s", v))
	}
	if v, ok := node.Int("avg_disk_bps"); ok {
		r.add(SecWorkload, SevInfo, "Avg disk throughput", humanBytes(v)+"/s")
	}
	// Overall scan efficiency: rows scanned per request across the node.
	if rows, ok1 := node.Int("total_rows_scanned"); ok1 {
		if reqs, ok2 := node.Int("total_requests"); ok2 && reqs > 0 {
			r.add(SecWorkload, SevInfo, "Rows scanned / request",
				fmt.Sprintf("%.1f rows/req (%d rows / %d reqs)", float64(rows)/float64(reqs), rows, reqs))
		}
	}
	if v, ok := node.Int("total_codebook_mem_usage"); ok {
		r.add(SecWorkload, SevInfo, "Total codebook mem usage", humanBytes(v))
	}
	if v, ok := node.Int("num_indexes"); ok {
		r.add(SecWorkload, SevInfo, "Num indexes", fmt.Sprintf("%d", v))
	}
	if v, ok := node.Str("indexer_state"); ok {
		r.add(SecWorkload, SevInfo, "Indexer state", v)
	}
	if v, ok := node.Str("uptime"); ok {
		r.add(SecWorkload, SevInfo, "Uptime", v)
	}
}

// ---- Outliers: Index Level ---------------------------------------------

func indexOutliers(m *model.Model, r *Report) {
	node, _ := last(m.Node)
	totalData, hasTotal := int64(0), false
	if node != nil {
		totalData, hasTotal = node.Int("total_data_size")
	}

	names := sortedKeys(m.Index)
	for _, name := range names {
		s, ok := last(m.Index[name])
		if !ok {
			continue
		}
		if v, ok := s.Int("frag_percent"); ok && v > FragPercentCeil {
			// Fragmentation on a tiny index is not actionable; skip it.
			disk, hasDisk := s.Int("disk_size")
			if !hasDisk || disk >= FragMinDiskSizeBytes {
				r.addCat(SecIndexOut, CatFragmentation, SevFlag, name,
					fmt.Sprintf("fragmentation %d%% (>%d%%), disk_size %s", v, FragPercentCeil, humanBytes(disk)))
			}
		}
		if v, ok := s.Int("cache_hit_percent"); ok && (100-v) > CacheMissPctCeil {
			r.addCat(SecIndexOut, CatCacheMiss, SevFlag, name, fmt.Sprintf("cache miss %d%% (>%d%%)", 100-v, CacheMissPctCeil))
		}
		if v, ok := s.Int("num_docs_pending"); ok && v > MutationsPendingCeil {
			r.addCat(SecIndexOut, CatMutationLag, SevFlag, name, fmt.Sprintf("mutations pending %d (>%d)", v, MutationsPendingCeil))
		}
		if v, ok := s.Int("num_docs_queued"); ok && v > MutationsQueuedCeil {
			r.addCat(SecIndexOut, CatMutationLag, SevFlag, name, fmt.Sprintf("mutations queued %d (>%d)", v, MutationsQueuedCeil))
		}
		if v, ok := s.Int("num_scan_timeouts"); ok && v > 0 {
			r.addCat(SecIndexOut, CatScanErrors, SevFlag, name, fmt.Sprintf("scan timeouts = %d", v))
		}
		if v, ok := s.Int("num_scan_errors"); ok && v > 0 {
			r.addCat(SecIndexOut, CatScanErrors, SevFlag, name, fmt.Sprintf("scan errors = %d", v))
		}
		if hasTotal && totalData > 0 {
			if d, ok := s.Int("data_size"); ok && pct(d, totalData) > IndexDataSharePct {
				r.addCat(SecIndexOut, CatDataSkew, SevWarn, name,
					fmt.Sprintf("data_size %s = %.0f%% of node total (>%d%%)", humanBytes(d), pct(d, totalData), IndexDataSharePct))
			}
		}
		// Large keys: buckets above ~1KB (closest greppable boundary to 512B).
		if dist, ok := s.Sub("key_size_distribution"); ok {
			large := int64(0)
			for _, b := range []string{"(1025-4096)", "(4097-102400)", "(102401-max)"} {
				if v, ok := dist.Int(b); ok {
					large += v
				}
			}
			if large > 0 {
				r.addCat(SecIndexOut, CatLargeKeys, SevWarn, name,
					fmt.Sprintf("%d keys >1KB (spec flags >%dB)", large, KeySizeBytesCeil))
			}
		}
	}

	// MVCC purge ratio from storage stats (nested under slice_*/BackStore).
	for _, name := range sortedKeys(m.Storage) {
		s, ok := last(m.Storage[name])
		if !ok {
			continue
		}
		if ratio, ok := deepFloat(s, "mvcc_purge_ratio"); ok && ratio > MVCCPurgeRatioCeil {
			detail := fmt.Sprintf("mvcc_purge_ratio %.2f (>%.0f)", ratio, MVCCPurgeRatioCeil)
			if mu, ok := indexMemoryUsed(m, name); ok {
				detail += fmt.Sprintf(", memory_used %s", humanBytes(mu))
			}
			r.addCat(SecIndexOut, CatMVCC, SevFlag, name, detail)
		}
	}
}

// indexMemoryUsed correlates a storage-stat key back to its index_* series and
// returns that index's memory_used. The two naming schemes differ:
//
//	storage: bucket:index:instId:partition
//	index:   bucket:index[ (replica N)]:instId   (no partition)
//
// so the reliable join is the instId (storage's second-to-last segment, index's
// trailing segment). For single-instance indexes whose key carries no instId we
// fall back to a longest colon-prefix match.
func indexMemoryUsed(m *model.Model, storageName string) (int64, bool) {
	parts := strings.Split(storageName, ":")
	if len(parts) >= 2 {
		if instID := parts[len(parts)-2]; isDigits(instID) {
			for k, s := range m.Index {
				if trailingSegment(k) == instID {
					if d, ok := last(s); ok {
						return d.Int("memory_used")
					}
				}
			}
		}
	}
	for i := len(parts); i >= 2; i-- {
		if s, ok := m.Index[strings.Join(parts[:i], ":")]; ok {
			if d, ok := last(s); ok {
				return d.Int("memory_used")
			}
		}
	}
	return 0, false
}

func trailingSegment(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[i+1:]
	}
	return s
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ---- Outliers: Indexer Level -------------------------------------------

func indexerOutliers(m *model.Model, r *Report) {
	// CPU >= 95% in any retained sample.
	node, _ := last(m.Node)
	if node != nil {
		if cores, ok := node.Int("num_cpu_core"); ok && cores > 0 {
			for _, v := range windowFloats(m.Node, "cpu_utilization") {
				if v/float64(cores) >= 95 {
					r.addCat(SecIndexerOut, CatCPU, SevFlag, "CPU saturation",
						fmt.Sprintf("a sample reached %.0f%% per-core utilization", v/float64(cores)))
					break
				}
			}
		}
		for _, v := range windowInts(m.Node, "avg_resident_percent") {
			if v < ResidentPercentFloor {
				r.addCat(SecIndexerOut, CatResident, SevFlag, "Low resident ratio",
					fmt.Sprintf("avg_resident_percent dropped to %d%% (<%d%%)", v, ResidentPercentFloor))
				break
			}
		}
		if b, ok := node.Bool("needs_restart"); ok && b {
			r.addCat(SecIndexerOut, CatRestart, SevFlag, "Needs restart", "indexer reports needs_restart=true")
		}
	}
}

// ---- Outliers from indexer.log event scanning (Phase 2) ----------------

func logEvents(m *model.Model, r *Report) {
	ev := func(key string) *model.Event { return m.Events[key] }

	// Crashes / restarts.
	if e := ev(model.EvCrash); e != nil {
		r.addCat(SecIndexerOut, CatCrash, SevFlag, "Panic / fatal error",
			fmt.Sprintf("%d occurrence(s), first %s, last %s", e.Count, shortTs(e.First), shortTs(e.Last)))
	}
	if e := ev(model.EvRestart); e != nil && e.Count > 1 {
		// More than one start banner in the log window implies a restart.
		r.addCat(SecIndexerOut, CatRestart, SevFlag, "Indexer restarts",
			fmt.Sprintf("%d start banners in log (first %s, last %s)", e.Count, shortTs(e.First), shortTs(e.Last)))
	}
	if e := ev(model.EvRollback); e != nil {
		r.addCat(SecIndexerOut, CatRollback, SevFlag, "Rollbacks",
			fmt.Sprintf("%d occurrence(s), last %s", e.Count, shortTs(e.Last)))
	}

	// Flush-monitor stalls (magnitude = max seconds waited).
	if e := ev(model.EvFlushMon); e != nil {
		sev := SevWarn
		if e.Max >= FlushWaitSecFlag {
			sev = SevFlag
		}
		detail := fmt.Sprintf("%d warning(s), max wait %.0fs", e.Count, e.Max)
		if e.MaxDetail != "" {
			detail += " (" + e.MaxDetail + ")"
		}
		r.addCat(SecIndexerOut, CatFlushMonitor, sev, "Flush stalls", detail)
	}

	// Slow operations / long lock holds (magnitude = max ms).
	if e := ev(model.EvSlowOp); e != nil {
		sev := SevWarn
		if e.Max >= SlowOpMsFlag {
			sev = SevFlag
		}
		detail := fmt.Sprintf("%d warning(s), max %.0fms", e.Count, e.Max)
		if e.MaxDetail != "" {
			detail += " in " + e.MaxDetail
		}
		r.addCat(SecIndexerOut, CatSlowOp, sev, "Slow operations", detail)
	}

	// Memtuner decrementing the plasma quota (memory pressure).
	if e := ev(model.EvMemtuner); e != nil {
		r.addCat(SecIndexerOut, CatMemtuner, SevWarn, "Plasma quota decrements",
			fmt.Sprintf("%d event(s) between %s and %s (adaptive quota under memory pressure)",
				e.Count, shortTs(e.First), shortTs(e.Last)))
	}

	// Communication errors.
	if e := ev(model.EvCommErr); e != nil {
		r.addCat(SecIndexerOut, CatCommError, SevWarn, "Transport/peer/dataport errors",
			fmt.Sprintf("%d error line(s), last %s", e.Count, shortTs(e.Last)))
	}

	// GC pause spikes from memstats.
	if mem, ok := last(m.Mem); ok {
		if maxNs, ok := maxArray(mem, "PauseNs"); ok && maxNs >= GCPauseNsFlag {
			r.addCat(SecIndexerOut, CatGCPause, SevFlag, "GC pause spike",
				fmt.Sprintf("max recent GC pause %.0fms (>=%dms)", maxNs/1e6, GCPauseNsFlag/1000000))
		}
	}

	// Stream-level events.
	if e := ev(model.EvNonAlignTS); e != nil {
		r.addCat(SecStreamOut, CatNonAlign, SevFlag, "Non-aligned timestamps",
			fmt.Sprintf("%d occurrence(s), last %s", e.Count, shortTs(e.Last)))
	}
	if e := ev(model.EvRepair); e != nil {
		r.addCat(SecStreamOut, CatRepair, SevFlag, "Stream repair activity",
			fmt.Sprintf("%d occurrence(s), last %s", e.Count, shortTs(e.Last)))
	}
}

// maxArray returns the max numeric value of a JSON array field (e.g. PauseNs).
func maxArray(s model.Sample, field string) (float64, bool) {
	raw, ok := s[field]
	if !ok {
		return 0, false
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return 0, false
	}
	max, found := 0.0, false
	for _, v := range arr {
		if f, ok := v.(float64); ok {
			if !found || f > max {
				max, found = f, true
			}
		}
	}
	return max, found
}

// shortTs trims a log timestamp to seconds precision for display.
func shortTs(ts string) string {
	if i := strings.IndexByte(ts, '.'); i >= 0 {
		return ts[:i]
	}
	return ts
}

// ---- Topology: replicas / partitions -----------------------------------

func topology(m *model.Model, r *Report) {
	st := m.Snapshot.IndexStatus
	if len(st) == 0 {
		return
	}
	notReady := 0
	for _, e := range st {
		name := indexLabel(e)
		status, _ := e.Str("status")
		if status != "" && status != "Ready" {
			notReady++
			detail := "status=" + status
			if p, ok := e.Int("progress"); ok {
				detail += fmt.Sprintf(" progress=%d%%", p)
			}
			r.addCat(SecTopology, CatNotReady, SevFlag, name, detail)
		}
		// Partition coverage: entries in partitionMap vs numPartition.
		if np, ok := e.Int("numPartition"); ok && np > 1 {
			if pm, ok := e.Sub("partitionMap"); ok {
				seen := map[int64]bool{}
				for _, plist := range pm {
					if arr, ok := plist.([]any); ok {
						for _, pid := range arr {
							if f, ok := pid.(float64); ok {
								seen[int64(f)] = true
							}
						}
					}
				}
				if int64(len(seen)) < np {
					r.addCat(SecTopology, CatPartCoverage, SevFlag, name,
						fmt.Sprintf("only %d of %d partitions present in partitionMap", len(seen), np))
				}
			}
		}
	}
	r.add(SecTopology, SevInfo, "Index inventory",
		fmt.Sprintf("%d index instances in getIndexStatus, %d not Ready", len(st), notReady))
}

// ---- Usage Analytics: top-N --------------------------------------------

func usage(m *model.Model, r *Report) {
	type kv struct {
		name string
		val  int64
	}
	collect := func(field string) []kv {
		var out []kv
		for name, s := range m.Index {
			if d, ok := last(s); ok {
				if v, ok := d.Int(field); ok {
					out = append(out, kv{name, v})
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].val > out[j].val })
		if len(out) > NumRowsScannedTopN {
			out = out[:NumRowsScannedTopN]
		}
		return out
	}
	emit := func(title, field string, human bool) {
		top := collect(field)
		if len(top) == 0 {
			return
		}
		var b strings.Builder
		for _, e := range top {
			val := fmt.Sprintf("%d", e.val)
			if human {
				val = humanBytes(e.val)
			}
			b.WriteString(fmt.Sprintf("\n%-14s %s", val, e.name))
		}
		r.add(SecUsage, SevInfo, title, b.String())
	}
	// emitRatio ranks indexes by numField/denField (skipping zero denominators).
	emitRatio := func(title, numField, denField, unit string) {
		type rkv struct {
			name string
			val  float64
		}
		var out []rkv
		for name, s := range m.Index {
			d, ok := last(s)
			if !ok {
				continue
			}
			num, ok1 := d.Int(numField)
			den, ok2 := d.Int(denField)
			if !ok1 || !ok2 || den == 0 {
				continue
			}
			out = append(out, rkv{name, float64(num) / float64(den)})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].val > out[j].val })
		if len(out) > NumRowsScannedTopN {
			out = out[:NumRowsScannedTopN]
		}
		if len(out) == 0 {
			return
		}
		var b strings.Builder
		for _, e := range out {
			b.WriteString(fmt.Sprintf("\n%-14s %s", fmt.Sprintf("%.1f%s", e.val, unit), e.name))
		}
		r.add(SecUsage, SevInfo, title, b.String())
	}

	emit("Top data_size", "data_size", true)
	emit("Top memory_used", "memory_used", true)
	emit("Top num_requests", "num_requests", false)
	emit("Top num_rows_scanned", "num_rows_scanned", false)
	emitRatio("Top num_rows_scanned/request", "num_rows_scanned", "num_requests", " rows/req")
}

// ---- helpers -----------------------------------------------------------

// windowInts returns the last-N observed integer values of a field (the values
// are coerced via a one-key Sample so json number/string forms are handled).
func windowInts(s *model.Series, field string) []int64 {
	var out []int64
	for _, fp := range s.FieldHistory(field) {
		if v, ok := (model.Sample{field: fp.V}).Int(field); ok {
			out = append(out, v)
		}
	}
	return out
}

func windowFloats(s *model.Series, field string) []float64 {
	var out []float64
	for _, fp := range s.FieldHistory(field) {
		if v, ok := (model.Sample{field: fp.V}).Float(field); ok {
			out = append(out, v)
		}
	}
	return out
}

// deepFloat looks for a field directly, then one level down (slice_0.MainStore
// / BackStore), to find plasma storage values whatever the wrapping.
func deepFloat(s model.Sample, field string) (float64, bool) {
	if v, ok := s.Float(field); ok {
		return v, true
	}
	for _, v := range s {
		if sub, ok := v.(map[string]any); ok {
			ss := model.Sample(sub)
			if f, ok := ss.Float(field); ok {
				return f, true
			}
			for _, store := range []string{"MainStore", "BackStore"} {
				if st, ok := ss.Sub(store); ok {
					if f, ok := st.Float(field); ok {
						return f, true
					}
				}
			}
		}
	}
	return 0, false
}

func indexLabel(e model.Sample) string {
	b, _ := e.Str("bucket")
	sc, _ := e.Str("scope")
	col, _ := e.Str("collection")
	nm, _ := e.Str("indexName")
	if nm == "" {
		nm, _ = e.Str("name")
	}
	rid, _ := e.Int("replicaId")
	label := fmt.Sprintf("%s:%s:%s:%s", b, sc, col, nm)
	if rid > 0 {
		label += fmt.Sprintf(" (replica %d)", rid)
	}
	return label
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func pct(part, whole int64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) / float64(whole) * 100
}

func maxInt(v []int64) int64 {
	m := v[0]
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func maxFloat(v []float64) float64 {
	m := v[0]
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func humanBytes(b int64) string {
	const u = 1024
	if b < u {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(u), 0
	for n := b / u; n >= u; n /= u {
		div *= u
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
