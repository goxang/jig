#!/bin/sh
# Arguments arrive as JSON in $JIG_ARGS and on stdin.
name=$(printf '%s' "$JIG_ARGS" | sed -n 's/.*"name": *"\([^"]*\)".*/\1/p')
echo "hello $name"
