
gdb /opt/couchbase/bin/memcached core.memcached.7524 

t a a bt


//backtrace of all threads from a running process
gdb -p process_id -ex ‘thread apply all bt’ < /dev/null

