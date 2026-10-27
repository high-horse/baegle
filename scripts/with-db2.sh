#!/usr/bin/env bash
set -euo pipefail

: "${IBM_DB_HOME:?Set IBM_DB_HOME to the extracted IBM CLI driver directory}"
if [[ ! -f "$IBM_DB_HOME/include/sqlcli1.h" ]]; then
  echo "IBM_DB_HOME must include the IBM CLI headers (include/sqlcli1.h)." >&2
  exit 1
fi
export CGO_ENABLED=1
export CGO_CFLAGS="${CGO_CFLAGS:-} -I$IBM_DB_HOME/include"
export CGO_LDFLAGS="${CGO_LDFLAGS:-} -L$IBM_DB_HOME/lib"
export LD_LIBRARY_PATH="$IBM_DB_HOME/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export DYLD_LIBRARY_PATH="$IBM_DB_HOME/lib${DYLD_LIBRARY_PATH:+:$DYLD_LIBRARY_PATH}"
if [[ $# -eq 0 ]]; then
  echo "Usage: IBM_DB_HOME=/path/to/clidriver bash scripts/with-db2.sh go test -tags db2 ./internal/..." >&2
  exit 1
fi
exec "$@"
