#!/bin/bash
all=$1

if [ "$all" == "all" ]; then
    echo "arch all" > /tmp/build_tarball.log
    ARCH_NAME=$(basename `git rev-parse --show-toplevel`)
else
    ARCH_NAME=${all}
    echo "arch ${ARCH_NAME}" > /tmp/build_tarball.log
fi

VERSION=$(tr -d '\n' <VERSION)
GOARCH=$(go env GOARCH)
GOHOSTOS=$(go env GOHOSTOS)

FILE="${ARCH_NAME}-${VERSION}.${GOHOSTOS}-${GOARCH}"

mkdir ${FILE}
cp README.md ${FILE}
cp LICENSE ${FILE}

if [ "$all" == "all" ]; then
    cp -p cmd/* ${FILE}/
else
    cp -p cmd/${ARCH_NAME} ${FILE}/
fi
cp -p ${GOBIN}/passwd_encrypt ${FILE}/
cp -pr contribs ${FILE}/

tar czf "${FILE}.tar.gz" ${FILE}

rm -rf ${FILE}
