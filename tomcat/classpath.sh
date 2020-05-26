[[ -z "${CLASSPATH+x}" ]] && return

printf "Linking \${CLASSPATH} entries to %s\n" "{{.path}}"

mkdir -p "{{.path}}"
IFS=':' read -ra PATHS <<< "${CLASSPATH}"
ln -s "${PATHS[@]}" "{{.path}}"
