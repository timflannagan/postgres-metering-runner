#! /bin/bash

set -eou pipefail

oc -n tflannag port-forward svc/postgres 5432:5432 &
