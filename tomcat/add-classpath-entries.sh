#!/bin/sh

mkdir -p "{{.libDir}}"
while IFS=: read -d: -r path; do
    printf "Linking $path to '{{.libDir}}/$(basename $path)'\n"
    ln -s "$path" "{{.libDir}}/$(basename $path)"
done <<< "${CLASSPATH}"
