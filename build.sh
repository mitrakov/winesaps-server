#!/bin/bash

IMAGE=mitrakov/winesaps
docker build -t $IMAGE .
docker push $IMAGE
