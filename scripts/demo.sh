#!/bin/sh
set -eu

: "${REFRESH_TOKEN:?set REFRESH_TOKEN}"

curl --fail-with-body --request POST http://localhost:8080/work-orders/follow-up \
  --header 'Content-Type: application/json' \
  --data "{\"work_order_id\":\"wo-1042\",\"technician_id\":\"tech-7\",\"photo_ref\":\"photo-88\",\"dispatch_status\":\"completed\",\"refresh_token\":\"${REFRESH_TOKEN}\"}"
