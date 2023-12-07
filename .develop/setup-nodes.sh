#!/bin/bash

nodeAContainerID=$(docker ps -aqf "name=clab-slurpeeth-node-a")

echo "configuring node a with containerd $nodeAContainerID"
docker exec "$nodeAContainerID" bash -c "apt update -y && apt install -y iproute2 inetutils-ping"
docker exec "$nodeAContainerID" ip addr add 192.168.1.1/24 dev eth1

nodeBContainerID=$(docker ps -aqf "name=clab-slurpeeth-node-b")

echo "configuring node b with containerd $nodeBContainerID"
docker exec "$nodeBContainerID" bash -c "apt update -y && apt install -y iproute2 inetutils-ping"
docker exec "$nodeBContainerID" ip addr add 192.168.1.2/24 dev eth1