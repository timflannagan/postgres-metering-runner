#! /bin/bash

set -eou pipefail

token=$(oc -n openshift-monitoring sa get-token prometheus-k8s)
host=$(oc -n openshift-monitoring get routes prometheus-k8s -o jsonpath={.spec.host})

if [[ -z $1 ]]; then
    echo "Invalid usage: \$1 is set to an empty string"
    exit 1
fi

query=$1

# Note on the usage:
# Curl is particularly annoying to pass variables to parameters, so in order to
# a query successfully you need to ensure that the `""` and `''` are properly formatted
# in order to avoid a parsing error. You either need to avoid using the same string
# deliminitor entirely or character escape them.
#
# The following shows this issue as an example:
# $ ./hack/run-prom-query.sh 'rate(container_cpu_usage_seconds_total{image!="", container_name!="POD"}[5m])'
curl -k -s -G -H "Authorization: Bearer $token" --data-urlencode "query=$query" https://${host}/api/v1/query | faq -f json
