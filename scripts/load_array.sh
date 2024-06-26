#!/usr/bin/env bash

set -e

jq -c '.[]' "$1" | while read -r data; do
  curl -X POST -H 'Content-Type: application/json' -d "$data" http://localhost:8080/docs
done
