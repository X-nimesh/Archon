#!/bin/sh
# Fails when any Go file is not gofmt-formatted.
#
# `gofmt -l` exits 0 even when it lists files, so it cannot be used directly as
# a lint gate.
unformatted=$(gofmt -l .)

if [ -n "$unformatted" ]; then
  echo "These files are not gofmt-formatted."
  echo "Run: pnpm --filter @archon/backend-go format"
  echo "$unformatted"
  exit 1
fi