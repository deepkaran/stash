# gsinutshell

Health-check utility for the Couchbase GSI indexing service. It reads the
logs/stats captured in a `cbcollect_info` bundle and prints a summarised health
report, flagging outliers per the GSI Nutshell spec.

```
go run . [-window N] <cbcollect_dir>
```

`-window N` controls how many recent samples are retained per stat field
(default 10 — "last 10 samples").

## Design

Two complementary data planes are parsed from one node's cbcollect:

**A. Time series** (trend + last-N-sample flagging)
- `indexer_stats.log` — lines `<ts> <key> <json>`; key `indexer` (node),
  `index_*` (per-index), `indexstorage_*` (per-index plasma storage).
- `indexer.log` — `memstats` (heap/GC, valid JSON) and `Periodic Aggregated
  StorageStats` (plasma aggregate incl. assigned/current quota; pretty-printed
  *pseudo*-JSON, parsed line-by-line).

**B. Point-in-time snapshot** (authoritative topology/settings/detail), from
`couchbase.log` blocks: `Index definitions are:` (`/getIndexStatus`),
`Indexer settings are:` (`/settings`), `Indexer stats are:`
(`/stats?partition=true`), `Index storage stats are:` (`/stats/storage`).

### Key design choices

- **Dynamic key-addressed samples, not typed structs.** Node/index stats are
  owned by `indexer/stats_manager.go` and storage stats by plasma; both drift
  across releases. Every payload is a `model.Sample` (`map[string]any`) with
  tolerant getters, so a missing/renamed key makes a rule report "n/a" instead
  of breaking the parser.
- **Per-field history, not per-line window.** The indexer publishes fields at
  different cadences — rich node fields (memory_quota, num_cpu_core,
  cpu_utilization, …) appear in only a handful of "full" samples, while a
  reduced set is published every tick. `Series` keeps the last-N observations
  *per field*, plus a newest-wins merged view for point-in-time lookups.
- **Streaming, bounded memory.** Files are scanned line-by-line; only the
  retained per-field history is kept, so memory is independent of the
  (multi-hundred-MB) input size.

## Packages

- `model` — `Sample` + tolerant getters; `Series` (per-field history + merged);
  `Snapshot`; `Model`.
- `loader` — file discovery and the three parsers (stats log, indexer log,
  couchbase log).
- `analyze` — `Finding`/`Report`, thresholds, and rule functions.
- `report` — text renderer.

## Phase roadmap

- **Phase 1 (done):** `indexer_stats.log` + `memstats` + `Periodic Aggregated
  StorageStats` + `couchbase.log` snapshot → Sizing/Memory, Workload, index &
  indexer-level outliers, topology, usage top-N.
- **Phase 2:** `indexer.log` event scanning (restarts/OOM, rollbacks, hung-op
  warnings, memtuner quota decrements, GC pause spikes); richer
  `indexstorage_*` rules (reclaim_pending, lss_fragmentation).
- **Phase 3:** `rebalance_report_*.json` parsing & summary.
- **Phase 4:** projector-node support; multi-cbcollect aggregation (rebalance
  master, cross-node missing replicas).
- **Future:** Prometheus `stats_snapshot` reader; live-cluster mode; graphs.
