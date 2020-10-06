#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}

oc -n ${NAMESPACE} port-forward svc/postgres 5432:5432 &
