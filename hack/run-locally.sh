#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}

route=$(oc -n openshift-monitoring get routes thanos-querier -o jsonpath={.spec.host})
token=$(oc -n openshift-monitoring sa get-token prometheus-k8s)

echo "Starting the metric importer runner"
go run cmd/driver/main.go exec \
    --log-level="debug" \
    --prometheus-address=https://${route} \
    --prometheus-bearer-token=${token} \
    --postgres-address=localhost
