#!/bin/bash
# One quality round over the given units (default: all): blind solve, editor review, repair.
cd "$(dirname "$0")"
W=${1:-4}; shift
python3 -u solve_run.py $W "$@"
python3 -u review_run.py $W "$@"
python3 -u repair_run.py $W "$@"
