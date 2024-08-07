#!/bin/bash

function check_endpoints {
	endpoints=( $("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get endpoints --no-headers | grep 'provider-' | awk '{print $1}') )
	for endpoint in ${endpoints[@]}; do
		port=$(${KUBECTL} -n "${CROSSPLANE_NAMESPACE}" get endpoints "$endpoint" -o jsonpath='{.subsets[*].ports[0].port}')
		if [[ -z "${port}" ]]; then
			echo "$endpoint - No served ports"
			return 1
		else
			echo "$endpoint - Ports present"
		fi
	done
}

attempt=1
max_attempts=10
while [[ $attempt -le $max_attempts ]]; do
	if check_endpoints; then
		exit 0
	else
		printf "Retrying... (%d/%d)\n" "$attempt" "$max_attempts" >&2
	fi
	((attempt++))
	sleep 5
done
exit 1
