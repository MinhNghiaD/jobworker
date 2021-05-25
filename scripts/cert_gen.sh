#!/bin/bash

set -eE
trap 'PREVIOUS_COMMAND=$THIS_COMMAND; THIS_COMMAND=$BASH_COMMAND' DEBUG
trap 'echo "FAILED COMMAND: $PREVIOUS_COMMAND"' ERR

CERTDIR=$PWD"/assets/cert"
CONFDIR=$PWD"/scripts/conf"

rm -rf $CERTDIR
mkdir -p $CERTDIR

#####################################################################################################
# In this file we will generate some keys and certificates for the following test cases
#   - Authorized Server CA and Client CA
#   - Server signed by Authorized Server CA 
#   - Clients signed by Authorized Client CA with Admin role and Observer role and Invalid role
#   - Unauthorized Server CA and Client CA
#   - Server signed by Unauthorized Server CA 
#   - Client signed by Unauthorized Client CA 
#   - Selfsigned Server and Selfsigned Client
#####################################################################################################


# Happy cases
#Generates a private key and certificate for the server certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/server_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/server_ca_key.pem -days 365 \
    -out $CERTDIR/server_ca_cert.pem -subj "/C=FR/O=Jobworker/OU=ServerCA/CN=localhost/"

#Generates a private key and certificate for the client certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/client_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/client_ca_key.pem -days 365 \
    -out $CERTDIR/client_ca_cert.pem -subj "/C=FR/O=Jobworker/OU=ClientCA/CN=localhost/"


#Generates a private key and certificate for the Job worker service
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/server_key.pem
openssl req -new -SHA256 -key $CERTDIR/server_key.pem -nodes -out $CERTDIR/server.csr -subj "/C=FR/O=Jobworker/OU=Server/CN=localhost/"
openssl x509 -req -SHA256 -extfile $CONFDIR/server_conf.ext -days 30 \
    -in $CERTDIR/server.csr -CA $CERTDIR/server_ca_cert.pem -CAkey $CERTDIR/server_ca_key.pem -CAcreateserial -out $CERTDIR/server_cert.pem


#Generates a private key and certificate for the clients, includes:
# Admin 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/admin_key.pem
openssl req -new -SHA256 -key $CERTDIR/admin_key.pem -nodes -out $CERTDIR/admin.csr -subj "/C=FR/O=Client/OU=Admin/CN=Admin 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/admin_conf.ext -days 30 \
    -in $CERTDIR/admin.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/admin_cert.pem

# Observer 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/observer_key.pem
openssl req -new -SHA256 -key $CERTDIR/observer_key.pem -nodes -out $CERTDIR/observer.csr -subj "/C=FR/O=Client/OU=Observer/CN=Observer 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/observer_conf.ext -days 30 \
    -in $CERTDIR/observer.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/observer_cert.pem




# Unauthorized test cases
#Generates a private key and certificate for an untrusted server certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/untrusted_server_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/untrusted_server_ca_key.pem -days 365 \
    -out $CERTDIR/untrusted_server_ca_cert.pem -subj "/C=FR/O=Untrusted/OU=NotServerCA/CN=localhost/"

#Generates a private key and certificate for an untrusted client certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/untrusted_client_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/untrusted_client_ca_key.pem -days 365 \
    -out $CERTDIR/untrusted_client_ca_cert.pem -subj "/C=FR/O=Untrusted/OU=NotClientCA/CN=localhost/"

#Server certificate signed by the Untrusted Server CA
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/untrusted_server_key.pem
openssl req -new -SHA256 -key $CERTDIR/untrusted_server_key.pem -nodes -out $CERTDIR/untrusted_server.csr -subj "/C=FR/O=Untrusted/OU=Server/CN=localhost/"
openssl x509 -req -SHA256 -extfile $CONFDIR/server_conf.ext -days 30 \
    -in $CERTDIR/untrusted_server.csr -CA $CERTDIR/untrusted_server_ca_cert.pem -CAkey $CERTDIR/untrusted_server_ca_key.pem \
    -CAcreateserial -out $CERTDIR/untrusted_server_cert.pem

# A client certificate signed by the Untrusted Client CA
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/untrusted_client_key.pem
openssl req -new -SHA256 -key $CERTDIR/untrusted_client_key.pem -nodes -out $CERTDIR/untrusted_client.csr -subj "/C=FR/O=Untrusted/OU=Admin/CN=Admin 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/admin_conf.ext -days 30 \
    -in $CERTDIR/untrusted_client.csr -CA $CERTDIR/untrusted_client_ca_cert.pem -CAkey $CERTDIR/untrusted_client_ca_key.pem \
    -CAcreateserial -out $CERTDIR/untrusted_client_cert.pem


#Sefl-signed server
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/selfsigned_server_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/selfsigned_server_key.pem -days 30  \
    -out $CERTDIR/selfsigned_server_cert.pem -subj "/C=FR/O=Jobworker/OU=Server/CN=localhost/"

# Self-signed client 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/selfsigned_client_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/selfsigned_client_key.pem -days 30  \
    -out $CERTDIR/selfsigned_client_cert.pem -subj "/C=FR/O=Client/OU=Admin/CN=localhost/"


# A client certificate with no access right for RBAC testing
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/impostor_key.pem
openssl req -new -SHA256 -key $CERTDIR/impostor_key.pem -nodes -out $CERTDIR/impostor.csr -subj "/C=FR/O=Client/OU=Impostor/CN=Impostor 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/impostor_conf.ext -days 30  \
    -in $CERTDIR/impostor.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/impostor_cert.pem



# Cleanup
rm $CERTDIR/*.csr $CERTDIR/*.srl