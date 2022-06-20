#!/bin/bash
set -e

[ -d /var/lib/zookeeper/data ] || mkdir /var/lib/zookeeper/data
[ -z "$ID_OFFSET" ] && ID_OFFSET=1
export ZOOKEEPER_SERVER_ID=$((${HOSTNAME##*-} + $ID_OFFSET))
echo "${ZOOKEEPER_SERVER_ID:-1}" | tee /var/lib/zookeeper/data/myid
cp -Lur /etc/kafka-configmap/* /etc/kafka/
