@echo off
echo Shutting down docker swarm...
echo[

docker swarm leave --force

echo[
echo Docker swarm is shut down.
echo[
pause