#!/bin/bash

set -eE
trap 'PREVIOUS_COMMAND=$THIS_COMMAND; THIS_COMMAND=$BASH_COMMAND' DEBUG
trap 'echo "FAILED COMMAND: $PREVIOUS_COMMAND"' ERR

CERTDIR=$PWD"/assets/cert"
TOKENDIR=$PWD"/assets/jwt"
CLAIMSDIR=$PWD"/assets/claims"

mkdir -p $TOKENDIR

#####################################################################################################
# In this file we will generate some JWT token to demonstrate RBAC implementation of job worker service
#   - Admin token with access right to all API on any jobs
#   - User tokens with the right to Start/Query but can only Stop, Stream their own jobs
#   - Observer token with accessonly to Query API
#   - Unknown role token with an unregistered role, which has no access to the service
#   - Invalid token signed by an unknown key
#####################################################################################################

# Admin token with access to Start/Stop/Query/Stream APIs of all jobs in the system
./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/jwt_cert.pem  --key=$CERTDIR/jwt_key.pem \
    --in=$CLAIMSDIR/admin.yaml --out=$TOKENDIR/admin.jwt

# User token with access to Start/Stop APIs and /Query/Stream API of their created jobs
./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/jwt_cert.pem  --key=$CERTDIR/jwt_key.pem \
    --in=$CLAIMSDIR/user.yaml --out=$TOKENDIR/user.jwt

./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/jwt_cert.pem  --key=$CERTDIR/jwt_key.pem \
    --in=$CLAIMSDIR/user2.yaml --out=$TOKENDIR/user2.jwt

# Observer token with access to Query API of all jobs in the system
./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/jwt_cert.pem  --key=$CERTDIR/jwt_key.pem \
    --in=$CLAIMSDIR/observer.yaml --out=$TOKENDIR/observer.jwt

# Unknown role token with no access to any API 
./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/jwt_cert.pem  --key=$CERTDIR/jwt_key.pem \
    --in=$CLAIMSDIR/unknown_role.yaml --out=$TOKENDIR/unknown_role.jwt

# Invalid token signed by invalid key
./bin/jwt_gen --issuer=jobworker --cert=$CERTDIR/user1_cert.pem  --key=$CERTDIR/user1_key.pem \
    --in=$CLAIMSDIR/admin.yaml --out=$TOKENDIR/invalid.jwt