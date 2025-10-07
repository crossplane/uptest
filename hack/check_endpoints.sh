#!/bin/bash

function check_endpoints {
	slices=($("${KUBECTL}" -n "${CROSSPLANE_NAMESPACE}" get endpointslices --no-headers | grep 'provider-' | awk '{print $1}'))
	for s in "${slices[@]}"; do
		addresses=$(${KUBECTL} -n "${CROSSPLANE_NAMESPACE}" get endpointslice "${s}" -o go-template='{{ range .endpoints }} {{- if and (eq .conditions.serving true) (eq .conditions.terminating false) }} {{- .addresses }}{{ "\n" }}{{- end }} {{- end }}')
		if [[ -z "${addresses}" ]]; then
			echo "${s} - No serving addresses in endpointslice"
			return 1
		else
			echo "${s} - Serving addresses ${addresses} found in endpointslice"
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
