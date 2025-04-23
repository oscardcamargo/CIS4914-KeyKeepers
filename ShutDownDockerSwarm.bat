@echo off
echo Shutting down docker swarm...
echo[

docker swarm leave --force
docker volume rm malwareHashes-db

echo[
echo Docker swarm is shut down.
echo[
pause