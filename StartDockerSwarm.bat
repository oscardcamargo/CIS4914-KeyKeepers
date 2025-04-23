@echo off
SETLOCAL ENABLEDELAYEDEXPANSION

:: The sha1 hash that the client will ask the nodes for
set hash=29812108115024db02ae79ac853743d31c606455

:: Number of nodes that are launched.
set NumberOfNodes=6

:: The port that the first node starts on. The next will be 8081, then 8082, etc.
set startPort=8080

echo Starting %NumberOfNodes% nodes...
echo[

docker volume create malwareHashes-db
docker run --rm -v malwareHashes-db:/data -v %cd%:/host busybox cp /host/data/malware_hashes.db /data/

docker swarm init
docker network create --driver overlay --subnet 10.20.0.0/24 --gateway 10.20.0.1 --ip-range 10.20.0.0/24 keykeeper-net
docker build -t keykeeper .

docker service create --name KeyKeeper-1 --endpoint-mode dnsrr --network keykeeper-net --mount type=volume,source=malwareHashes-db,target=/usr/src/app/data keykeeper 10.20.0.0/24 %startPort% node1
set port=%startPort%
for /L %%i in (2,1,%NumberOfNodes%) do (
    set /A prev=%%i-1
    set prevPort=!port!
    set /A port=!port!+1
    set "cmd=docker service create --name KeyKeeper-%%i --endpoint-mode dnsrr --network keykeeper-net  keykeeper 10.20.0.0/24 !port! node%%i KeyKeeper-!prev! !prevPort! node!prev!"
    echo !cmd!
    !cmd!
)
:: Maybe pause here for a moment to allow containers to stabilize
cd client
docker build -t keykeeperclient .
docker service create --name KeyKeeperClient --endpoint-mode dnsrr --network keykeeper-net keykeeperclient KeyKeeper-1 %startPort% node1 %hash% 10.20.0.0/24

echo[
echo Check if the containers are running in Docker Desktop.
echo[
pause