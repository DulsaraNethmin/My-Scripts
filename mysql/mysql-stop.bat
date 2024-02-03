@echo off
REM Change directory to where your docker-compose.yml is
cd /d C:\Users\Nethmin\Documents\scripts\mysql\docker-compose.yml

REM Stop and remove containers, networks, and volumes
docker-compose down

REM Optionally, you can use 'docker-compose stop' if you wish to stop the containers without removing them

REM You can add additional commands after this line
