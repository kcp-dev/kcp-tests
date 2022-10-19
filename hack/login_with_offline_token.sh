#!/bin/bash

set -o errexit
set -o pipefail

# Check the curl package exist
if ! [ -x "$(command -v curl)" ]; then
    echo 'Error: curl is not installed.'
    exit 1
fi

# Check the jq package exist
if ! [ -x "$(command -v jq)" ]; then
    echo 'Error: jq is not installed.'
    exit 1
fi

# The login util is only used for kcp services which deploy with redhat sso
# Please export your OFFLINE_TOKEN before execute the util
# https://access.redhat.com/articles/3626371#bgenerating-a-new-offline-tokenb-3
if [ -z "$OFFLINE_TOKEN" ]; then
	echo 'Error: Please export your OFFLINE_TOKEN before execute the util.'
	exit 1
fi

# Get token info by SSO
SSO_TOKEN=$(curl -s https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token -d grant_type=refresh_token -d client_id=cloud-services -d refresh_token="${OFFLINE_TOKEN}" | jq .)
REFRESH_TOKEN=$(echo "$SSO_TOKEN" | jq .refresh_token)
ID_TOKEN=$(echo "$SSO_TOKEN" | jq .id_token)

# Write the token to oidc-login cache
echo "{\"id_token\":${ID_TOKEN},\"refresh_token\":${REFRESH_TOKEN}}" > ~/.kube/cache/oidc-login/de0b44c30948a686e739661da92d5a6bf9c6b1fb85ce4c37589e089ba03d0ec6
echo 'INFO: Log in to kcp service with "OFFLINE_TOKEN" succeed'
