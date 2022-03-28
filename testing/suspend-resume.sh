#!/bin/bash
#
# Copyright (c) 2022 NEC Laboratories Europe GmbH.
#
# Authors: Sergey Fedorov <sergey.fedorov@neclab.eu>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu -o 'pipefail'

: ${NCLIENTS:=2}
: ${NFAULTY:=2}
: ${NREPLICAS:=$(( 2*$NFAULTY + 1 ))}

: ${NROUNDS:=5}
: ${MINSLEEP:=1}
: ${MAXSLEEP:=10}
: ${MINWAIT:=1}
: ${MAXWAIT:=5}
: ${FINALDELAY:=10}

echo
echo "Number of clients: $NCLIENTS"
echo "Number of replicas: $NREPLICAS"
echo "Max faulty replicas: $NFAULTY"
echo
echo "Number of rounds: $NROUNDS"
echo "Min suspend duration: $MINSLEEP seconds"
echo "Max suspend duration: $MAXSLEEP seconds"
echo "Min wait delay: $MINWAIT seconds"
echo "Max wait delay: $MAXWAIT seconds"
echo "Final delay: $FINALDELAY seconds"
echo

echo "Running in '$PWD' directory"
if which peer; then
    peer_bin=`which peer`
elif [ -x "$PWD/bin/peer" ]; then
    peer_bin="$PWD/bin/peer"
    echo "Adding '$PWD/lib' to LD_LIBRARY_PATH"
    export LD_LIBRARY_PATH="$PWD/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
else
    echo "Cannot locate peer binary" >&2
    exit 1
fi

echo "Using '$peer_bin' binary"
echo

declare -a client_pids replica_pids

cleanup () {
    set +e
    kill -CONT ${replica_pids[@]} 2>/dev/null
    kill ${client_pids[@]} ${replica_pids[@]} 2>/dev/null
    wait
}

trap cleanup EXIT

rm -rf logs
mkdir logs

echo "Starting replicas..."
for i in `seq 0 $((NREPLICAS-1))`; do
    $peer_bin run $i &>"logs/replica${i}.log" &
    replica_pids[$i]=$!
done

echo "Starting clients..."
for i in `seq 0 $((NCLIENTS-1))`; do
    ( k=0; while true; do echo "Client $i; Request $((k++))"; sleep 1; done ) |
        $peer_bin request --id=$i --timeout=0 &>"logs/client${i}.log" &
    client_pids[$i]=$!
done

check_crashed () {
    crashed_pids=()
    for r in ${!replica_pids[@]}; do
        pid=${replica_pids[$r]}
        if [ -z "$(ps -q $pid -o pid=)" ]; then
            crashed_pids[$r]=$pid
        fi
    done
    if [ ${#crashed_pids[@]} -gt 0 ]; then
        echo "Replica ${!crashed_pids[@]} crashed" >&2
        exit 1
    fi
}

echo
for i in `seq $NROUNDS`; do
    echo "===== Round $i ====="

    suspend_pids=()
    while [ ${#suspend_pids[@]} -lt $NFAULTY ]; do
        r=$(( RANDOM % NREPLICAS ))
        suspend_pids[$r]=${replica_pids[$r]}
    done

    delay=$(( MINSLEEP + RANDOM % (MAXSLEEP-MINSLEEP+1) ))
    echo "Suspending replica ${!suspend_pids[@]} for $delay seconds..."
    kill -STOP ${suspend_pids[@]}
    sleep $delay

    echo "Resuming replica ${!suspend_pids[@]}";
    kill -CONT ${suspend_pids[@]}

    delay=$(( MINWAIT + RANDOM % (MAXWAIT-MINWAIT+1) ))
    echo "Waiting $delay seconds..."
    sleep $delay
    echo

    check_crashed
done | awk '{ print strftime("[%T]"), $0 }'

echo
echo "Stopping clients..."
kill ${client_pids[@]}

delay=$FINALDELAY
echo "Waiting $delay seconds..."
sleep $delay

check_crashed

echo "Stopping replicas..."
kill ${replica_pids[@]}

trap - EXIT

block_out=()
for r in `seq 0 $((NREPLICAS-1))`; do
    block_out[$r]="logs/replica${r}_block.out"
    grep -o 'Received block\[[0-9]\+\].*' "logs/replica${r}.log" >"${block_out[$r]}"
done

for r in `seq 1 $((NREPLICAS-1))`; do
    if ! cmp -s "${block_out[0]}" "${block_out[$r]}"; then
        echo "Replicas produced different sequences of blocks" >&2
        exit 1
    fi
done

echo
echo "PASS"
