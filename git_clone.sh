git clone https://github.com/couchbase/testrunner.git
cd testrunner
cp ~/workspace/stash/commit-msg .git/hooks
chmod 755 .git/hooks/commit-msg
git remote add gerrit ssh://review.couchbase.org:29418/testrunner.git
git fetch gerrit
