#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}

oc -n ${NAMESPACE} exec -it $(oc -n ${NAMESPACE} get po -l app=postgres --no-headers | awk '{ print $1 }') -- psql --username=testuser --dbname=metering
