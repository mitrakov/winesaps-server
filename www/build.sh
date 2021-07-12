#!/bin/bash

IMAGE=mitrakov/winesaps-web
docker build -t $IMAGE .
docker push $IMAGE
