#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}
SERVICEACCOUNT_NAME=${2:-runner}

token=$(oc -n ${NAMESPACE} sa get-token ${SERVICEACCOUNT_NAME})
host=$(oc -n openshift-monitoring get routes prometheus-k8s -o jsonpath={.spec.host})

curl -k -H "Authorization: Bearer $token" https://$host/api/v1/query?query=up
exit $?
