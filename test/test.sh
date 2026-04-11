#!/bin/bash
# Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

CUR_DIR=$(dirname "$(readlink -f "$0")")
PROJECT_DIR=$(cd "${CUR_DIR}/.."; pwd)
ROOT_PATH=$PROJECT_DIR

# go module prepare
export GO111MODULE=on
export GONOSUMDB=*
export CGO_ENABLED=1
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/axw/gocov/gocov@latest
go install github.com/matm/gocov-html/cmd/gocov-html@latest

# resolve missing go.sum entry
go env -w "GOFLAGS"="-mod=mod"

# coverage mode
# set: 每个语句是否执行？
# count: 每个语句执行了几次？
# atomic: 类似于count, 但表示的是并行程序中的精确计数
export GOCOVER_MODE="set"

# protoc
echo "generating fs proto pb objects"
. "${PROJECT_DIR}/build/compile_functions.sh"

# test module name
MODULE_LIST=(\
"common" \
"faasfrontend"
)

TARGET_MODULE=$1

if [ -z "$TARGET_MODULE" ]; then
    for module in "${MODULE_LIST[@]}"; do
        if ! sh -x "${CUR_DIR}/${module}/test.sh"; then
            echo "Failed to test ${module}"
            exit 1
        fi
        echo "Succeed to test ${module}"
    done
else
    found=0
    for module in "${MODULE_LIST[@]}"; do
        if [ "$module" = "$TARGET_MODULE" ]; then
            found=1
            if ! sh -x "${CUR_DIR}/${module}/test.sh"; then
                echo "Failed to test ${module}"
                exit 1
            fi
            echo "Succeed to test ${module}"
            break
        fi
    done
    if [ $found -eq 0 ]; then
        echo "Error: Module '$TARGET_MODULE' not found in MODULE_LIST"
        echo "Available modules: ${MODULE_LIST[*]}"
        exit 1
    fi
fi

exit 0
