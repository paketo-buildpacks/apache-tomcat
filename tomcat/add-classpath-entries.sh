#!/bin/sh
set -e
mkdir -p "{{.libDir}}"
(
    IFS=:
    for path in $(printf '%s' "$CLASSPATH"); do
        printf "Linking %s to '{{.libDir}}/$(basename "$path")'\n" "$path"
        ln -s "$path" "{{.libDir}}/$(basename "$path")"
    done
)
