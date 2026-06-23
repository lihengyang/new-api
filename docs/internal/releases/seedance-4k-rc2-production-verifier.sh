#!/usr/bin/env bash
set -euo pipefail

# Internal verifier for Seedance 2.0 4K RC2 production smoke evidence.
#
# This script does not send video requests, does not switch containers, and does
# not require an API key. It verifies already-created production smoke evidence
# from MySQL using sanitized output only.
#
# Required environment:
#   MYSQL_DEFAULTS_FILE: path to a secret-safe mysql defaults file.
#   STANDARD_NOVIDEO_CLIENT_REQUEST_ID: client_request_id for Standard 4K no-video.
#   STANDARD_VIDEO_CLIENT_REQUEST_ID: client_request_id for Standard 4K video_url.
#   FAST_CLIENT_REQUEST_ID: client_request_id for Fast 4K reject.
#   FAST_HTTP_STATUS: HTTP status captured by the Fast 4K reject smoke.
#   SMOKE_START_EPOCH: lower-bound Unix timestamp for Fast reject billing log check.
#
# Optional environment:
#   FAST_ERROR_SHAPE: nested_error_type, top_level_code, or top_level_type.
#   FAST_ERROR_JSON_FILE: JSON response file to parse when FAST_ERROR_SHAPE is unset.
#   TOKEN_NAME: defaults to henrytest.
#   FAST_MODEL_ALIAS: defaults to lsf-seedance-2.0-fast-henrytest.
#   QUOTA_PER_UNIT: defaults to 500000.
#   SETTLEMENT_ATTEMPTS: defaults to 3.
#   SETTLEMENT_SLEEP_SECONDS: defaults to 30.

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    printf 'verifier_config=missing_%s\n' "$name"
    exit 64
  fi
}

require_env MYSQL_DEFAULTS_FILE
require_env STANDARD_NOVIDEO_CLIENT_REQUEST_ID
require_env STANDARD_VIDEO_CLIENT_REQUEST_ID
require_env FAST_CLIENT_REQUEST_ID
require_env FAST_HTTP_STATUS
require_env SMOKE_START_EPOCH

if [ ! -r "$MYSQL_DEFAULTS_FILE" ]; then
  echo 'verifier_config=mysql_defaults_file_not_readable'
  exit 64
fi

TOKEN_NAME="${TOKEN_NAME:-henrytest}"
FAST_MODEL_ALIAS="${FAST_MODEL_ALIAS:-lsf-seedance-2.0-fast-henrytest}"
QUOTA_PER_UNIT="${QUOTA_PER_UNIT:-500000}"
SETTLEMENT_ATTEMPTS="${SETTLEMENT_ATTEMPTS:-3}"
SETTLEMENT_SLEEP_SECONDS="${SETTLEMENT_SLEEP_SECONDS:-30}"

mysql_q() {
  mysql --defaults-extra-file="$MYSQL_DEFAULTS_FILE" --batch --skip-column-names -e "$1"
}

fast_error_shape() {
  if [ -n "${FAST_ERROR_SHAPE:-}" ]; then
    printf '%s\n' "$FAST_ERROR_SHAPE"
    return 0
  fi
  if [ -z "${FAST_ERROR_JSON_FILE:-}" ] || [ ! -r "$FAST_ERROR_JSON_FILE" ]; then
    echo 'missing_fast_error_shape'
    return 0
  fi
  python3 - "$FAST_ERROR_JSON_FILE" <<'PY'
import json
import sys

try:
    with open(sys.argv[1]) as fh:
        data = json.load(fh)
except Exception:
    print("invalid_json")
    raise SystemExit(0)

if isinstance(data.get("error"), dict) and data["error"].get("type") == "invalid_request_error":
    print("nested_error_type")
elif data.get("code") == "invalid_request_error":
    print("top_level_code")
elif data.get("type") == "invalid_request_error":
    print("top_level_type")
else:
    print("unaccepted")
PY
}

verify_fast_reject() {
  local shape
  shape="$(fast_error_shape)"

  printf 'fast_4k_http_status=%s\n' "$FAST_HTTP_STATUS"
  printf 'fast_4k_error_shape=%s\n' "$shape"

  if [ "$FAST_HTTP_STATUS" != "400" ]; then
    echo 'fast_4k_reject=fail_http_status'
    return 1
  fi
  case "$shape" in
    nested_error_type|top_level_code|top_level_type) ;;
    *)
      echo 'fast_4k_reject=fail_error_shape'
      return 1
      ;;
  esac

  local task_count log_count
  task_count="$(mysql_q "SELECT COUNT(*) FROM tasks WHERE client_request_id = '$FAST_CLIENT_REQUEST_ID';")"
  log_count="$(mysql_q "SELECT COUNT(*) FROM logs WHERE token_name = '$TOKEN_NAME' AND model_name = '$FAST_MODEL_ALIAS' AND created_at >= $SMOKE_START_EPOCH;")"

  printf 'fast_4k_no_task_count=%s\n' "$task_count"
  printf 'fast_4k_billing_log_count=%s\n' "$log_count"

  if [ "$task_count" != "0" ]; then
    echo 'fast_4k_reject=fail_task_created'
    return 1
  fi
  if [ "$log_count" != "0" ]; then
    echo 'fast_4k_reject=fail_billing_log_found'
    return 1
  fi

  echo 'fast_4k_no_upstream_call_evidence=pass_prevalidation_no_task_no_billing'
  echo 'fast_4k_reject=pass'
}

verify_standard_billing_once() {
  python3 - "$MYSQL_DEFAULTS_FILE" \
    "$STANDARD_NOVIDEO_CLIENT_REQUEST_ID" \
    "$STANDARD_VIDEO_CLIENT_REQUEST_ID" \
    "$QUOTA_PER_UNIT" <<'PY'
import decimal
import hashlib
import subprocess
import sys

cnf, novideo_id, video_id, quota_per_unit = sys.argv[1:]
quota_per_unit = decimal.Decimal(quota_per_unit)

CASES = {
    novideo_id: ("no-video", decimal.Decimal("4.0")),
    video_id: ("video_url", decimal.Decimal("2.4")),
}

def shq(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"

def mysql(sql: str) -> str:
    return subprocess.check_output(
        ["mysql", f"--defaults-extra-file={cnf}", "--batch", "--skip-column-names", "-e", sql],
        text=True,
    )

task_sql = f"""
SELECT
  client_request_id,
  task_id,
  status,
  progress,
  quota,
  COALESCE(JSON_EXTRACT(data, '$.usage.completion_tokens'), 0) AS completion_tokens,
  COALESCE(JSON_EXTRACT(data, '$.usage.total_tokens'), 0) AS total_tokens,
  COALESCE(JSON_EXTRACT(private_data, '$.billing_context.group_ratio'), 0) AS group_ratio,
  COALESCE(JSON_EXTRACT(private_data, '$.billing_context.other_ratios.seedance_intl_billing'), 0) AS seedance_ratio
FROM tasks
WHERE client_request_id IN ({shq(novideo_id)}, {shq(video_id)})
ORDER BY FIELD(client_request_id, {shq(novideo_id)}, {shq(video_id)});
"""

task_rows = [line.split("\t") for line in mysql(task_sql).splitlines() if line.strip()]
if len(task_rows) != 2:
    print("standard_billing=fail_missing_task")
    raise SystemExit(1)

pending = False
failed = False

for row in task_rows:
    cid, public_task_id, status, progress, precharge, completion_tokens, total_tokens, group_ratio, seedance_ratio = (row + [""] * 9)[:9]
    content_summary, expected_price = CASES[cid]
    task_ref = hashlib.sha256(public_task_id.encode()).hexdigest()[:16]

    print(f"standard_case={content_summary}")
    print(f"task_ref={task_ref}")
    print(f"client_request_id={cid}")
    print(f"status={status}")
    print(f"progress={progress}")
    print(f"precharge={precharge}")
    print(f"completion_tokens={completion_tokens}")
    print(f"total_tokens={total_tokens}")
    print(f"group_ratio={group_ratio}")
    print(f"seedance_ratio={seedance_ratio}")

    if status == "FAILURE":
        print("standard_billing=fail_task_failure")
        failed = True
        continue

    if status != "SUCCESS":
        print("standard_billing=billing_settlement_pending")
        pending = True
        continue

    log_sql = f"""
SELECT
  type,
  quota,
  COALESCE(JSON_EXTRACT(other, '$.pre_consumed_quota'), 'null') AS pre_consumed_quota,
  COALESCE(JSON_EXTRACT(other, '$.actual_quota'), 'null') AS actual_quota,
  COALESCE(JSON_EXTRACT(other, '$.group_ratio'), 'null') AS log_group_ratio,
  COALESCE(JSON_EXTRACT(other, '$.seedance_intl_billing'), 'null') AS log_seedance_ratio
FROM logs
WHERE JSON_VALID(other)
  AND JSON_UNQUOTE(JSON_EXTRACT(other, '$.task_id')) = {shq(public_task_id)}
  AND JSON_EXTRACT(other, '$.actual_quota') IS NOT NULL
ORDER BY id DESC
LIMIT 1;
"""
    settlement_rows = [line.split("\t") for line in mysql(log_sql).splitlines() if line.strip()]
    if not settlement_rows:
        print("settlement_actual_quota=missing")
        print("final_net_quota=pending")
        print("standard_billing=billing_settlement_pending")
        pending = True
        continue

    _, settlement_delta, log_precharge, actual_quota, log_group_ratio, log_seedance_ratio = (settlement_rows[0] + [""] * 6)[:6]
    final_net_quota = decimal.Decimal(actual_quota)
    completion = decimal.Decimal(completion_tokens)
    group = decimal.Decimal(group_ratio)

    print(f"settlement_delta={settlement_delta}")
    print(f"settlement_precharge={log_precharge}")
    print(f"settlement_actual_quota={actual_quota}")
    print(f"final_net_quota={actual_quota}")
    print(f"settlement_group_ratio={log_group_ratio}")
    print(f"settlement_seedance_ratio={log_seedance_ratio}")

    if completion <= 0 or group <= 0:
        print("effective_upstream_price=unknown")
        print("price_match=false")
        failed = True
        continue

    effective = (final_net_quota / quota_per_unit) / completion * decimal.Decimal("1000000") / group
    price_match = abs(effective - expected_price) <= decimal.Decimal("0.05")
    print(f"effective_upstream_price={effective.quantize(decimal.Decimal('0.000001'))}")
    print(f"expected_price={expected_price}")
    print(f"price_match={str(price_match).lower()}")

    if not price_match:
        failed = True

if failed:
    print("standard_billing=fail")
    raise SystemExit(1)
if pending:
    print("standard_billing=billing_settlement_pending")
    raise SystemExit(20)

print("standard_billing=pass")
PY
}

verify_standard_billing() {
  local attempt rc
  for attempt in $(seq 1 "$SETTLEMENT_ATTEMPTS"); do
    printf 'standard_billing_attempt=%s\n' "$attempt"
    set +e
    verify_standard_billing_once
    rc=$?
    set -e
    case "$rc" in
      0)
        return 0
        ;;
      20)
        if [ "$attempt" -lt "$SETTLEMENT_ATTEMPTS" ]; then
          printf 'standard_billing_pending_wait_seconds=%s\n' "$SETTLEMENT_SLEEP_SECONDS"
          sleep "$SETTLEMENT_SLEEP_SECONDS"
        else
          echo 'standard_billing=billing_settlement_pending'
          return 20
        fi
        ;;
      *)
        return "$rc"
        ;;
    esac
  done
}

verify_fast_reject
standard_rc=0
verify_standard_billing || standard_rc=$?

case "$standard_rc" in
  0)
    echo 'production_smoke_verifier=pass'
    ;;
  20)
    echo 'production_smoke_verifier=billing_settlement_pending'
    exit 20
    ;;
  *)
    echo 'production_smoke_verifier=fail'
    exit "$standard_rc"
    ;;
esac
