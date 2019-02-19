./periodicstats.py --kind idxstats --params ".*mutation_queue_size.*" ~/tmp/triage/CBSE-2727/1hr-indexer.log

for ploting indexer log profile, --kind should always be idxstats.
    --params should be reg-ex matching key-name in PeriodicStats JSON map.
    For plotly registration/credentials.
    https://plot.ly/python/getting-started/
