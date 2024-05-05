#!/bin/bash

REPO_NAME=$(basename `git rev-parse --show-toplevel`)
VERSION=$(tr -d '\n' <VERSION)
GOARCH=$(go env GOARCH)
GOHOSTOS=$(go env GOHOSTOS)

FILE="${REPO_NAME}-${VERSION}.${GOHOSTOS}-${GOARCH}"


mkdir ${FILE}
cp README.md ${FILE}
cp LICENSE ${FILE}
cp -pr cmd/* ${FILE}/
cp -pr contribs ${FILE}/

tar czf "${FILE}.tar.gz" ${FILE}

rm -rf ${FILE}
