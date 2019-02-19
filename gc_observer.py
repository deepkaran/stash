import requests
import json
import time

last = None

gct = None

def loop():
    global last
    global gct

    while True:
        #r = requests.get('http://54.179.233.148:9102/stats/mem')
        r = requests.get('http://127.0.0.1:9102/stats/mem')
        itm = json.loads(r.content)

        if last:
            print "TotalAlloc", itm["TotalAlloc"]/(1024*1024.0)
            print "AllocsPerSec", (itm["TotalAlloc"]-last["TotalAlloc"])/(1024*1024.0)
            print "HeapInuse", itm["HeapInuse"] / (1024*1024.0)
            print "AvgGCpause", (sum(itm["PauseNs"])/(len(itm["PauseNs"])))/(10**9 * 1.000)
            print "NumGCs", itm["NumGC"]

            gcint = itm["LastGC"] - last["LastGC"]
            if gcint:
                gct = gcint/(10**9 * 1.0)
            print "GCInterval", gct
            print "NumHeapObjects", itm["HeapObjects"]
            print "NumHeapObjectsPerSec", (itm["HeapObjects"] - last["HeapObjects"])

            print

        last = itm
        time.sleep(1)


loop()
