package analyze

// Thresholds for "flag if ..." rules, sourced from the GSI Nutshell spec.
// Centralised so they are easy to tune; where the indexer exposes a configured
// limit in /settings, rules prefer that over these constants.
const (
	ResidentPercentFloor  = 10                // flag avg_resident_percent < 10%
	CPUUtilPercentCeil    = 90                // flag per-core cpu utilization > 90%
	HeapInusePctOfQuota   = 10                // flag HeapInuse > 10% of quota (plasma)
	PlasmaQuotaPctFloor   = 90                // flag current_quota < 90% of assigned_quota
	IndexDataSharePct     = 20                // flag a single index > 20% of total data size
	CacheMissPctCeil      = 50                // flag cache miss > 50% (cache_hit_percent < 50)
	FragPercentCeil       = 50                // flag fragmentation > 50%
	FragMinDiskSizeBytes  = 100 * 1024 * 1024 // skip fragmentation flag below 100MB disk size
	MutationsPendingCeil  = 50000
	MutationsQueuedCeil   = 10000
	MVCCPurgeRatioCeil    = 3.0
	KeySizeBytesCeil      = 512 // flag index key size > 512 bytes
	RowsScannedPerReqCeil = 10000
	NumRowsScannedTopN    = 5
)

// Section names (mirror the spec's report layout).
const (
	SecSizing     = "Indexer Node Health / Sizing-Memory"
	SecWorkload   = "Indexer Node Health / Workload"
	SecIndexOut   = "Outliers / Index Level"
	SecIndexerOut = "Outliers / Indexer Level"
	SecTopology   = "Topology / Replicas-Partitions"
	SecUsage      = "Usage Analytics"
)

// Categories group findings into sub-headings within a section.
const (
	CatFragmentation = "Fragmentation"
	CatCacheMiss     = "Cache miss"
	CatMutationLag   = "Mutation backlog"
	CatScanErrors    = "Scan errors / timeouts"
	CatDataSkew      = "Data size skew"
	CatLargeKeys     = "Large keys"
	CatMVCC          = "MVCC purge ratio"

	CatCPU      = "CPU saturation"
	CatResident = "Resident ratio"
	CatRestart  = "Restart / health"

	CatNotReady     = "Not Ready"
	CatPartCoverage = "Partition coverage"
)
