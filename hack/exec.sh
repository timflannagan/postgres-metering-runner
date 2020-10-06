#! /bin/bash

oc -n tflannag exec -it $(oc -n tflannag get po -l app=postgres --no-headers | awk '{ print $1 }') -- psql --username=testuser --dbname=metering
