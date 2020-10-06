#! /bin/bash

set -eou pipefail

token=$(oc -n openshift-monitoring sa get-token prometheus-k8s)
host=$(oc -n openshift-monitoring get routes prometheus-k8s -o jsonpath={.spec.host})

curl -k -s -H "Authorization: Bearer $token" https://${host}/api/v1/rules | faq -f json
