@echo off
REM Change directory to where your docker-compose.yml is
cd /d C:\Users\Nethmin\Documents\scripts\mysql\docker-compose.yml

REM Run Docker Compose up
docker-compose up -d

REM -d flag runs containers in the background

REM You can add additional commands after this line
