#! /bin/bash

set -eou pipefail

NAMESPACE=${1:-tflannag}

oc -n ${NAMESPACE} exec -it $(oc -n ${NAMESPACE} get po -l app=postgres --no-headers | awk '{ print $1 }') -- psql --username=testuser --dbname=metering -c 'SELECT schemaname,relname,n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC;'
