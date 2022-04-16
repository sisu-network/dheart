#!/usr/bin/env bash

# Source the common.sh script
# shellcheck source=./common.sh
. "$(git rev-parse --show-toplevel || echo "..")/scripts/common.sh"

cd "$PROJECT_DIR" || exit 1

GOLANGCI_LINT="golangci-lint"
if [ ! -z ${CI} ]; then
 if [ ! -f "./bin/golangci-lint" ]; then
 echo_info "Install golangci-lint for static code analysis (via curl)"
 # install into ./bin/
 # because different project might need different golang version,
 # and thus, need to use different linter version
 curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.45.2
 GOLANGCI_LINT="./bin/golangci-lint"
 fi
fi

GOLANGCI_LINT="$GOLANGCI_LINT --config ./.golangci.yml"

$GOLANGCI_LINT run

EXIT_CODE=$?
exit ${EXIT_CODE}
