#!/bin/bash

service_url=${1:-localhost}

set -e

die() {
	echo $1
	exit 1
}

cleanup () {
    echo "Tearing down services"
    docker-compose down
}
trap cleanup EXIT

is_healthy() {
    service="$1"
    container_id="$(docker-compose ps -q "$service")"
    health_status="$(docker inspect -f "{{.State.Health.Status}}" "$container_id")"

    if [ "$health_status" = "healthy" ]; then
        return 0
    else
        return 1
    fi
}

echo "Starting services"

docker-compose up -d

echo "Waiting for services to start..."

while ! is_healthy ferrum; do sleep 1; done

echo "Starting integration tests"

echo "Generating authentication token"
token="$(curl -s http://${service_url}/generate-token | jq -r '.token')" || die "Failed generate token test"
[ -n "${token}" ] || die "Failed to generate authentication token"

echo "Testing unauthorised requests"
status="$(curl -s -o /dev/null -w "%{http_code}" http://${service_url}/api/v1/patients)" || die "Failed unauthorised access test"
[ "${status}" == "401" ] || die "Failed unauthorised access test with status: ${status}"

echo "Testing returned status when adding a patient"
status="$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer ${token}" --data '{"first_name":"Hoenir"}' http://${service_url}/api/v1/patients)" || die "Failed add patient test"
[ "${status}" == "201" ] || die "Failed add patient test with status: ${status}"

echo "Testing retrieving an added patient"
patient_link="$(curl -s -o /dev/null -H "Authorization: Bearer ${token}" --data '{"first_name":"Heimdall"}' -D - http://${service_url}/api/v1/patients | grep "Location: " | cut -d' ' -f2- | tr -cd '[:print:]')" || die "Failed retrieve added patient test"
first_name="$(curl -s -H "Authorization: Bearer ${token}" "${patient_link}" | jq -r '.first_name')"  || die "Failed retrieve added patient test"
[ "${first_name}" == "Heimdall" ] || die "Failed add second patient test from URL '${patient_link}' with wrong name: ${first_name}"

echo "Testing added patients count"
patient_count="$(curl -s -H "Authorization: Bearer ${token}" http://${service_url}/api/v1/patients | jq length)"  || die "Failed added patients count test"
[ "${patient_count}" == "2" ] || die "Failed get patients test with wrong patient count: ${patient_count}"