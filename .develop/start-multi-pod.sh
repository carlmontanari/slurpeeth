#!/bin/bash

go mod tidy

service docker start
sleep 1

containerlab deploy -t .develop/clab.topo.multi-pod.node-"$NODE_LETTER".yaml
sleep 5

nodeAContainerID=$(docker ps -aq)
docker exec "$nodeAContainerID" bash -c "apt update -y && apt install -y iproute2 inetutils-ping"
docker exec "$nodeAContainerID" ip addr add 192.168.1."$LAST_OCTET"/24 dev eth1
