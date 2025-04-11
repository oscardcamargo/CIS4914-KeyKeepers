@echo off
:: Number of nodes that are launched.
set NumberOfNodes=4

echo Starting %NumberOfNodes% nodes...
echo[

docker swarm init
docker build -t keykeeper .
docker service create --name KeyKeeper-swarm --replicas %NumberOfNodes% -p 8080:8080 keykeeper

echo[
echo Check if the containers are running in Docker Desktop.
echo[
pause