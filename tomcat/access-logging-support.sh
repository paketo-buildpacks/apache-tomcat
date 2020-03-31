ENABLED=${BPL_TOMCAT_ACCESS_LOGGING:=n}

[[ "${ENABLED}" = "n" ]] && return

printf "Tomcat Access Logging enabled\n"

export JAVA_OPTS="${JAVA_OPTS} -Daccess.logging.enabled=true"
