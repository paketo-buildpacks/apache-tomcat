ENABLED=${BPL_TOMCAT_ACCESS_LOGGING:=n}

[[ "${ENABLED}" = "n" ]] && return

printf "Tomcat Access Logging enabled\n"

export JAVA_TOOL_OPTIONS="${JAVA_TOOL_OPTIONS} -Daccess.logging.enabled=true"
