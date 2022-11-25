#!/usr/bin/env python

import sys

def ts(f):
    m = {}
    f = open(f).read()
    f = f.replace("[","")
    f = f.replace("]","")
    f = f.split(",")
    for itm in f:
        vbseq = itm.split("=")
        m[int(vbseq[0])] = int(vbseq[1])

    return m


def diff(t1, t2):
    for i in t1.keys():
        if t1[i] != t2[i]:
            print "vb%d : %d!=%d" %(i, t1[i], t2[i])

def less(t1,t2):
    for i in t1.keys():
        if t1[i] < t2[i]:
            print "vb%d : %d < %d" %(i, t1[i], t2[i])


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print sys.argv[0], "requested_timestamp_file", "snapshot_timestamp_file"
        sys.exit(1)

    t1 = ts(sys.argv[2])
    t2 = ts(sys.argv[1])

    less(t1,t2)


