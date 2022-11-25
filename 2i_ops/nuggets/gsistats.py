# This utility provides you with a variety of reports on GSI and Plasma internal statistics, 
# The reports show data on memory consumption, disk space consumption, plasma cache hit/miss ratio
# and other useful information
# for all the try, exception blocks,             
#     Just print(e) is cleaner and more likely what you want,
#     but if you insist on printing message specifically whenever possible...

import argparse
import requests
import json
import time
import urllib3
import pandas as pd
import numpy as np
from pandas.io.json import json_normalize 
import matplotlib.pyplot as plt
import sys

# Retrieve a list of all indexer node in a given cluster
def get_index_nodes(mainhost, qconn, query, up):

    url_main_host = "http://" + mainhost + ":8091/pools/default"
    list_index_nodes_stmt = 'select SPLIT(h.hostname,":")[0] as hostname \
                FROM CURL("%s", {"get":true, %s}) n \
                UNNEST n.nodes as h \
                where any v in h.services satisfies v = "index" end'

    s = list_index_nodes_stmt % (url_main_host, up)
    # print(s)
    query['statement'] = s
    # print(query)

    try:
        #print(qconn)
        response = qconn.request('POST', '/query/service', fields=query, encode_multipart=False)
        response.read(cache_content=False)
        body = json.loads(response.data.decode('utf8'))
        #print(body)
        df = pd.DataFrame(body['results'])
    except Exception as e:
        # Just print(e) is cleaner and more likely what you want,
        # but if you insist on printing message specifically whenever possible...
        if hasattr(e, 'message'):
            print(e.message)
        else:
            print(e)
        exit()

    # return just the hostname column of the data frame as an array of inddxer node
    # IP addresses
    return df['hostname'].values

# Report 1 ###############################################################################################
## function to run the GSI Stats report
def gsi_stats_rpt(conn, query, hs, up):
    #print("this function runs the gsi stats report")
    gsi_stats_stmt = 'SELECT t.iname indexname, bucketname, "%s" hostname, \
                    sum(t.icount) icnt, \
                    round(sum(data_size)/(1024*1024),2) dsz_MB, \
                    sum(ndindexed) ndi, \
                    avg(fpct) fpct, \
                    avg(rpct) rpct, \
                    avg(avgdr) avgdr, \
                    avg(avgmr) avgmr, \
                    sum(ndp) ndp \
                from \
                    (SELECT iname, bucketname, \
                        case when (sn = "data_size") then s.val else missing end as data_size, \
                        case when (sn = "items_count") then s.val else missing end as icount, \
                        case when (sn = "num_docs_indexed") then s.val else missing end as ndindexed, \
                        case when (sn = "frag_percent") then s.val else missing end as fpct, \
                        case when (sn = "resident_percent") then s.val else missing end as rpct, \
                        case when (sn = "avg_drain_rate") then s.val else missing end as avgdr, \
                        case when (sn = "avg_mutation_rate") then s.val else missing end as avgmr, \
                        case when (sn = "num_docs_pending") then s.val else missing end as ndp \
                    from \
                        (SELECT object_inner_pairs(plasma) as stats \
                        FROM CURL("%s:9102/stats", {"get":true, %s}) plasma) as p \
                        UNNEST p.stats as s \
                        let bucketname = SPLIT(s.name,":")[0], iname = SPLIT(s.name,":")[1], sn = SPLIT(s.name,":")[2] \
                        where sn in ["data_size","items_count", "num_docs_indexed", "frag_percent", \
                                    "resident_percent", "avg_drain_rate", "avg_mutation_rate", "num_docs_pending"] \
                    order by iname, sn) as t \
                group by t.iname, bucketname \
                order by indexname, hostname'
    dfu = pd.DataFrame()
    for h in hs:
        s = gsi_stats_stmt % (h, h, up)
        query['statement'] = s
        #print(s)
        try:
            response = conn.request('POST', '/query/service', fields=query, encode_multipart=False)
            response.read(cache_content=False)
            body = json.loads(response.data.decode('utf8'))
            # print(body)
            df = pd.DataFrame(body['results'])
            # print(df)
            dfu = dfu.append(df)
            # print("done host %s", h)
        except Exception as e:
            # Just print(e) is cleaner and more likely what you want,
            # but if you insist on printing message specifically whenever possible...
            if hasattr(e, 'message'):
                print(e.message)
            else:
                print(e)
            exit()
    dfu = dfu[[
        "indexname", "hostname", "bucketname", "dsz_MB","icnt", "ndi", 
        "fpct", "rpct", "avgdr", "avgmr", "ndp"]]
    print('report header: (dsz=datasize, icnt=items_count, ndi=num_docs_indexed, fpct=frag_percent, rpct=resident_percent, \
avgdr=avg_drain_rate, avgmr=avg_mutation_rate, npd=num_docs_pending)'
        )
    
    print(dfu.sort_values(by=['indexname','hostname']))
    dfu.sort_values(by=['indexname','hostname']).to_csv('gsi_stats.csv', index=False, header=True, sep=',')
    
# Report 2 ###############################################################################################
# Run a query to report on the average key size for all indexes in a given cluster
def plasma_avg_key_rpt(conn, query, hs, up):
    print("this function runs the plasma stats report on hosts")
    plasma_avg_key_stmt = 'SELECT "%s" hostname, indexname, bucketname, \
                    p.Stats.MainStore.items_count ms_icnt, \
                    p.Stats.BackStore.items_count bs_icnt, \
                    p.Stats.MainStore.avg_item_size ms_ais, \
                    p.Stats.BackStore.avg_item_size bs_ais, \
                    p.Stats.MainStore.lss_fragmentation ms_lssfr, \
                    p.Stats.BackStore.lss_fragmentation bs_lssfr, \
                    p.Stats.MainStore.reclaim_pending ms_rcp, \
                    p.Stats.BackStore.reclaim_pending bs_rcp, \
                    p.Stats.MainStore.avg_page_size ms_aps, \
                    p.Stats.BackStore.avg_page_size bs_aps, \
                    p.Stats.MainStore.mem_throttled ms_mth, \
                    p.Stats.BackStore.mem_throttled bs_mth, \
                    p.Stats.MainStore.mvcc_purge_ratio ms_mvccpr, \
                    p.Stats.BackStore.mvcc_purge_ratio bs_mvccpr \
                    FROM CURL("%s:9102/stats/storage", {"get":true, %s}) p \
                    let indexname = SPLIT(p.`Index`, ":")[1], bucketname = SPLIT(p.`Index`, ":")[0] \
                    order by ms_ais desc'
    dfu = pd.DataFrame()
    for h in hs:
        s = plasma_avg_key_stmt % (h, h, up)
        query['statement'] = s
        try:
            response = conn.request('POST', '/query/service', fields=query, encode_multipart=False)
            response.read(cache_content=False)
            body = json.loads(response.data.decode('utf8'))
            # print(body)
            df = pd.DataFrame(body['results'])
            dfu = dfu.append(df)
        except Exception as e:
            if hasattr(e, 'message'):
                print(e.message)
            else:
                print(e)
            exit()

    dfu = dfu[['hostname', 'indexname', 'bucketname', 'bs_icnt', 'ms_icnt', 'bs_ais', 'ms_ais', 'ms_lssfr', 
        'bs_lssfr', 'ms_rcp','bs_rcp', 'ms_aps', 'bs_aps', 'ms_mth', 'bs_mth', 'ms_mvccpr', 'bs_mvccpr']]
    print(dfu.sort_values(by=['indexname','hostname']))
    dfu.sort_values(by=['indexname','hostname']).to_csv('plasma_avg_key_size.csv', index=False, header=True, sep=',')

# Report 3 ###############################################################################################
# Run a query to report on the average key size for all indexes in a given cluster
def plasma_idx_mem_used(conn, query, hs, up):
    print("reporting on plasma_idx_mem_used ....")
    print(hs)
    plasma_avg_key_stmt = 'select "%s" hostname, indexname, bucketname, \
                ROUND(s.plasma.Stats.MainStore.memory_size/(1024*1024),2) ms_msize_MB,  \
                ROUND(s.plasma.Stats.MainStore.memory_size_index/(1024*1024),2) ms_msize_i_MB,  \
                ROUND(s.plasma.Stats.BackStore.memory_size/(1024*1024),2) bs_msize_MB,  \
                ROUND(s.plasma.Stats.BackStore.memory_size_index/(1024*1024),2)  bs_msize_i_MB \
                from \
                    (SELECT plasma FROM CURL("%s:9102/stats/storage", {"get":true, %s}) plasma) as s \
                let bucketname = SPLIT(s.plasma.`Index`,":")[0], indexname = SPLIT(s.plasma.`Index` ,":")[1] \
                order by ms_msize_MB'               
    dfu = pd.DataFrame()
    for h in hs:
        s = plasma_avg_key_stmt % (h, h, up)
        print(s)
        query['statement'] = s
        try:
            #print(query)
            response = conn.request('POST', '/query/service', fields=query, encode_multipart=False)
            response.read(cache_content=False)
            body = json.loads(response.data.decode('utf8'))
            #print(body)
            df = pd.DataFrame(body['results'])
            dfu = dfu.append(df)
        except Exception as e:
            if hasattr(e, 'message'):
                print(e.message)
            else:
                print(e)
            exit()

    dfu = dfu[['hostname', 'indexname', 'bucketname', 'ms_msize_MB', 'ms_msize_i_MB', 'bs_msize_MB', 'bs_msize_i_MB']]
    dfu.sort_values(by=['ms_msize_MB'], ascending=False)
    dfu = dfu.append(pd.Series(dfu.sum(),name='Total'))
    print(dfu)
    #dfu.sort_values(by=['ms_msize_MB']).to_csv('plasma_idx_mem_used.csv', index=False, header=True, sep=',')

# Report 4 ###############################################################################################
# Report on the bucket level GSI Stats, global stats at the bucket level
def gsi_bucket_stats(conn, query, hs, up):
    print("reporting on bucket level GSI stats ....")
    print(hs)
    plasma_bucket_stats_stmt = 'select "%s" hostname, f1 bucketname, f2 stat, s.val val from \
            (SELECT object_inner_pairs(plasma) as stats  \
             FROM CURL("%s:9102/stats", {"get":true, %s}) plasma) as p  \
	         UNNEST p.stats as s \
	         let f1 = SPLIT(s.name,":")[0], f2 = SPLIT(s.name,":")[1], f3 = SPLIT(s.name,":")[2] \
	         where (f3 is missing or f3 is null) and f2 in ["mutation_queue_size", "num_mutations_queued", "num_nonalign_ts", "num_rollbacks"]'               
    dfu = pd.DataFrame()
    for h in hs:
        s = plasma_bucket_stats_stmt % (h, h, up)
        #print(s)
        query['statement'] = s
        try:
            #print(query)
            response = conn.request('POST', '/query/service', fields=query, encode_multipart=False)
            response.read(cache_content=False)
            body = json.loads(response.data.decode('utf8'))
            #print(body)
            df = pd.DataFrame(body['results'])
            dfu = dfu.append(df)
        except Exception as e:
            if hasattr(e, 'message'):
                print(e.message)
            else:
                print(e)
            exit()

    dfu = dfu[['hostname', 'bucketname', 'stat', 'val']]
    print(dfu.pivot_table(index=['hostname','bucketname'], columns='stat', values='val'))

def unknown_rpt(conn, query, hs, up):
    print("the report # %d given is not one of the valid report #")
    print("1 - gsi stats report")
    print("2 - gsi avg key size report")
    print("3 - Plasma Per Index Memory Consumption")
    print("4 - gsi bucket level stats")
    exit

def pickReport(report):
    switcher = {
        1: gsi_stats_rpt,
        2: plasma_avg_key_rpt,
        3: plasma_idx_mem_used,
        4: gsi_bucket_stats,
    }
    return switcher.get(report, unknown_rpt)


def main():

    parser = argparse.ArgumentParser(description='python gsistats')
    parser.add_argument("-u", "--user", help="cb user login name", required=True)
    parser.add_argument("-p", "--password", help="cb user login password", required=True)
    parser.add_argument("-i", "--ip", help="IP address of a cb node", required=True)
    parser.add_argument("-r", "--report", 
        help="Usage: -r <report number>", required=True, type=int)

    # parse input arguments
    args = parser.parse_args()

    # construct the full url for the query end point & connect 
    urlcbq = "http://" + args.ip + ":8093"

    try:
        conn = urllib3.connection_from_url(urlcbq)
        print("connected!")
    except:
        print("failed to connect")
        exit()
    

    # Initalize the query object for use for subsequent queries
    query = {'max_parallelism': 1}
    query['creds'] = '[{"user":' + '"' + args.user + '"' + ',' + '"pass":' + '"' + args.password + '"' + '}]'

    # get the list of all indexer nodes which the queries will be sent to 
    #  up is the 
    up = '"user":' + '"' + args.user + ':' + args.password + '"'
    hl = get_index_nodes(args.ip, conn, query, up)

    # Setup Panda options
    pd.set_option('display.max_rows', 1000)
    pd.set_option('display.max_colwidth', 32)
    pd.set_option('display.column_space', 32)
    pd.set_option('display.width', 1024)

    # setup the query object with the credential and Run the report 
    pickReport(args.report)(conn, query, hl, up)

if __name__ == "__main__":
    main()
