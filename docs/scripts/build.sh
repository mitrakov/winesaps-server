#!/bin/bash

echo Finding PID...
PID=`ps aux | grep winesaps | grep -v grep | awk '{print $2}'`
echo Killing $PID...
kill $PID
echo Building server...

#go build -race -i mitrakov.ru/home/winesaps # <- for Go 1.8
go  build -race    mitrakov.ru/home/winesaps
echo Removing nohup.out
rm nohup.out
echo Starting server
nohup ./start.sh winesaps &
echo Done!
