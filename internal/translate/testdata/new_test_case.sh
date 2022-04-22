#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly HERE=$(cd "$(dirname "$0")" && pwd)

mkdir -p ${HERE}/$1
touch ${HERE}/$1/in-gateway.yaml
touch ${HERE}/$1/in-proxies.yaml
touch ${HERE}/$1/out-gateway.yaml
touch ${HERE}/$1/out-httproutes.yaml
touch ${HERE}/$1/out-tlsroutes.yaml
touch ${HERE}/$1/error.txt
