
grep "Indexer::handleCreateIndex" indexer.log
grep MutationCount indexer.log |tail -1
grep FlushedCount indexer.log |tail -1
grep maybeSetPersistFlag indexer.log
grep SnapshotInfo indexer.log
grep BuildDone indexer.log

