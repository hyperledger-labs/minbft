#!/bin/sh

. /opt/intel/sgxsdk/environment

(
    flock 3
    if [ -f /data/keys.yaml ]; then exit; fi
    keytool -o /data/keys.yaml generate -u lib/libusig.signed.so
) 3>/data/.keys.yaml.lock

exec peer --keys /data/keys.yaml "$@"
