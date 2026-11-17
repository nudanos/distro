#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
# A pointer from string("...").c_str() dies with the temporary at the end
# of the statement. chunker2 built opc's argv that way; GCC 14 at -O2
# reuses the storage, opc received garbage arguments and every REST op
# command answered "Invalid command: [$]". Keep such pointers out of the code.
set -eu
if grep -nE 'string\([^()]*\)\.c_str\(\)' src/server/*.cc src/server/*.hh; then
    echo "no-dangling-cstr: pointer into a temporary std::string (above)" >&2
    exit 1
fi
echo "no-dangling-cstr: OK"
