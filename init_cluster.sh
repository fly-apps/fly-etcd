#!/bin/sh

set -eu

IPS=$(dig aaaa $FLY_APP_NAME.internal +short)
INITIAL_CLUSTER=""
FIRST_NODE=""
for ip in $IPS 
do 
  ID=$(echo $ip | md5sum | awk '{ print $1 }')
  NODE="$ID=http://[$ip]:2380"
  echo "NODE: $NODE"

  if [ $FIRST_NODE -eq "" ] 
  then
    INITIAL_CLUSTER=$NODE
    FIRST_NODE=$NODE
  else 
    INITIAL_CLUSTER="$INITIAL_CLUSTER,$NODE"
  fi
done

IP=$(ip -6 addr show eth0 | grep -Eo '(fdaa.*)\/')
IP=${IP%?}
NAME=$(echo $IP | md5sum | awk '{ print $1 }')

LOCAL_PEER_URL="http://0.0.0.0:2380"
PEER_URL="http://[$IP]:2380"

LOCAL_CLIENT_URL="http://0.0.0.0:2379"
CLIENT_URL="http://[$IP]:2379"

exec etcd \
  --name=$NAME \
  --initial-advertise-peer-urls=$PEER_URL \
  --listen-peer-urls=$PEER_URL \
  --listen-client-urls="$LOCAL_CLIENT_URL" \
  --advertise-client-urls=$CLIENT_URL \
  --data-dir=/etcd_data \
  --initial-cluster=$INITIAL_CLUSTER \
  --initial-cluster-token=cluster-token \
  --auto-compaction-retention=1 \
  --initial-cluster-state new 

