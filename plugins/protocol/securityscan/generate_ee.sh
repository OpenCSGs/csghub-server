#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
protoc \
  --go_out=. --go_opt=module=opencsg.com/csghub-server \
  --go-grpc_out=. --go-grpc_opt=module=opencsg.com/csghub-server \
  plugins/protocol/securityscan/v1/security_scan_ee.proto

python3 - <<'PY'
from pathlib import Path

for path in Path('plugins/protocol/securityscan/v1').glob('security_scan_ee*.go'):
    data = path.read_text()
    if not data.startswith('//go:build ee || saas\n'):
        path.write_text('//go:build ee || saas\n\n' + data)
    target = path.with_name(path.name.replace('security_scan_ee', 'security_scan').replace('.pb.go', '.pb_ee.go'))
    path.rename(target)
PY
