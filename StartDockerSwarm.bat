@echo off
SETLOCAL ENABLEDELAYEDEXPANSION

:: Number of nodes that are launched.
set NumberOfNodes=5

:: The port that the first node starts on. The next will be 8081, then 8082, etc.
set startPort=8080



echo Starting %NumberOfNodes% nodes...
echo[

docker swarm init
docker network create --driver overlay --subnet 10.20.0.0/24 --gateway 10.20.0.1 --ip-range 10.20.0.0/24 keykeeper-net
docker build -t keykeeper .

docker service create --name KeyKeeper-1 --endpoint-mode dnsrr --network keykeeper-net  keykeeper 10.20.0.0/24 %startPort% node1
set port=%startPort%
for /L %%i in (2,1,%NumberOfNodes%) do (
    set /A prev=%%i-1
    set prevPort=!port!
    set /A port=!port!+1
    set "cmd=docker service create --name KeyKeeper-%%i --endpoint-mode dnsrr --network keykeeper-net  keykeeper 10.20.0.0/24 !port! node%%i KeyKeeper-!prev! !prevPort! node!prev!"
    echo !cmd!
    !cmd!
)

echo[
echo Check if the containers are running in Docker Desktop.
echo[
pause