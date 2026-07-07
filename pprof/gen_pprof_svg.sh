#!/bin/bash

usage() {
    cat <<EOF
Usage: $(basename "$0") <version-build> <pprof-dir> <cpu|mem> [toy]

Arguments:
  version-build  Version and build number in the form <version>-<build>,
                 e.g. 8.1.0-1937. Used to locate or download the RPM and
                 as the directory name under \$TRIAGE_RPM_DIR.
  pprof-dir      Directory containing the .pprof capture files.
  cpu|mem        Profile type to generate:
                   cpu  -> cpu_prof.svg        (CPU flame graph)
                   mem  -> mprofa.svg          (alloc_space)
                           mprofi.svg          (inuse_space)
  toy            Optional. Pass 'toy' to download from the toy builds URL
                 instead of the regular release builds.

Environment:
  TRIAGE_RPM_DIR  Root directory for RPM builds. Defaults to ~/triage/rpms if unset.
                  Set this in ~/.bashrc or ~/.zshrc:
                    export TRIAGE_RPM_DIR=~/workspace/triage/rpms

RPM Download:
  The script will automatically download the RPM from latestbuilds if not already
  present under \$TRIAGE_RPM_DIR/<version-build>/. Supported versions:
    7.6.x  (trinity)
    8.0.x  (morpheus)
    8.1.x  (totoro)
  The indexer binary is extracted on first run and reused on subsequent runs.

  For toy builds, pass 'toy' as the 4th argument. The RPM is then fetched from:
    https://latestbuilds.service.couchbase.com/builds/latestbuilds/couchbase-server/toybuilds/<build>/
  (no codename mapping is required for toy builds).

Examples:
  $(basename "$0") 8.1.0-1937 /tmp/pprof_files cpu
  $(basename "$0") 8.1.0-1937 /tmp/pprof_files mem
  $(basename "$0") 8.1.0-23037 /tmp/pprof_files cpu toy
EOF
}

if [ "$1" == "-h" ] || [ "$1" == "--help" ] || [ $# -lt 3 ]; then
    usage
    exit 0
fi

original_dir=$(pwd)

rpm_dir="${TRIAGE_RPM_DIR:-$HOME/triage/rpms}"

this_rpm=$1
is_toy=""
if [ "$4" == "toy" ]; then
    is_toy=1
fi

version="${this_rpm%-*}"
build="${this_rpm##*-}"
major_minor="${version%.*}"

rpm_filename="couchbase-server-enterprise-${version}-${build}-linux.x86_64.rpm"

if [ -n "$is_toy" ]; then
    rpm_url="https://latestbuilds.service.couchbase.com/builds/latestbuilds/couchbase-server/toybuilds/${build}/${rpm_filename}"
else
    case "$major_minor" in
        8.1) codename="totoro" ;;
        8.0) codename="morpheus" ;;
        7.6) codename="trinity" ;;
        *)   echo "Unknown version '$major_minor'. Add a codename mapping for it."; exit 1 ;;
    esac
    rpm_url="https://latestbuilds.service.couchbase.com/builds/latestbuilds/couchbase-server/${codename}/${build}/${rpm_filename}"
fi

full_path="${rpm_dir}/${this_rpm}"
indexer_bin=/opt/couchbase/bin/indexer
full_bin="${full_path}${indexer_bin}"

if [ -e "$full_bin" ]; then
    echo "File '$full_bin' exists. Skip expanding rpm."
else
    mkdir -p "$full_path"
    if [ ! -f "${full_path}/${rpm_filename}" ]; then
        echo "Downloading ${rpm_filename}..."
        curl -f --progress-bar -o "${full_path}/${rpm_filename}" "$rpm_url" \
            || { echo "Download failed: $rpm_url"; exit 1; }
    fi
    cd "$full_path" || { echo "Failed to change directory"; exit 1; }
    rpm2cpio *.rpm | cpio -idmv
fi

cd "$original_dir" || { echo "Failed to return to original directory"; exit 1; }

pprof_dir=$2
pprof_type=$3

cd "$pprof_dir" || { echo "Failed to change directory"; exit 1; }

if [ "$pprof_type" == "cpu" ]; then
    echo "Generating CPU profile..."
    go tool pprof -svg "$full_bin" *index_cpu_*.pprof > cpu_prof.svg
elif [ "$pprof_type" == "mem" ]; then
    echo "Generating mem profile..."
    go tool pprof -alloc_space -svg "$full_bin" *index_heap_*.pprof > mprofa.svg
    go tool pprof -inuse_space -svg "$full_bin" *index_heap_*.pprof > mprofi.svg
else
    echo "Invalid input string. Please use 'cpu' or 'mem'."
fi
