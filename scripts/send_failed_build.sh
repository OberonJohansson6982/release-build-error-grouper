#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/build-events \
  --header 'Content-Type: application/json' \
  --data '{"build_id":"build-1842","release_id":"release-91","service":"compiler","stage":"compile","status":"failed","error_message":"module graph failed","exception":"compile: missing module"}'
