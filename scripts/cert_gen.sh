#!/bin/bash

set -eE
trap 'PREVIOUS_COMMAND=$THIS_COMMAND; THIS_COMMAND=$BASH_COMMAND' DEBUG
trap 'echo "FAILED COMMAND: $PREVIOUS_COMMAND"' ERR

CERTDIR=$PWD"/assets/cert"
CONFDIR=$PWD"/scripts/conf"

rm -rf $CERTDIR
mkdir $CERTDIR

# NOTE: we won't use passphrase for private keys in this exercise
#Generates a private key and certificate for the server certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/server_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/server_ca_key.pem -days 365 -out $CERTDIR/server_ca_cert.pem -subj "/C=FR/O=Jobworker/OU=ServerCA/CN=localhost/"

#Generates a private key and certificate for the client certificate authority
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/client_ca_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/client_ca_key.pem -days 365 -out $CERTDIR/client_ca_cert.pem -subj "/C=FR/O=Jobworker/OU=ClientCA/CN=localhost/"


#Generates a private key and certificate for the Job worker service
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/server_key.pem
openssl req -new -SHA256 -key $CERTDIR/server_key.pem -nodes -out $CERTDIR/server.csr -subj "/C=FR/O=Jobworker/OU=Server/CN=localhost/"
openssl x509 -req -SHA256 -extfile $CONFDIR/server_conf.ext -days 30 -in $CERTDIR/server.csr -CA $CERTDIR/server_ca_cert.pem -CAkey $CERTDIR/server_ca_key.pem -CAcreateserial -out $CERTDIR/server_cert.pem


#Generates a private key and certificate for the unauthorized server (self signed certificate)
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/unauthorized_server_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/unauthorized_server_key.pem -days 30 -out $CERTDIR/unauthorized_server_cert.pem -subj "/C=FR/O=Jobworker/OU=Server/CN=localhost/"


#Generates a private key and certificate for the clients, includes:
# Admin 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/admin_key.pem
openssl req -new -SHA256 -key $CERTDIR/admin_key.pem -nodes -out $CERTDIR/admin.csr -subj "/C=FR/O=Client/OU=Admin/CN=Admin 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/admin_conf.ext -days 30 -in $CERTDIR/admin.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/admin_cert.pem

# Observer 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/observer_key.pem
openssl req -new -SHA256 -key $CERTDIR/observer_key.pem -nodes -out $CERTDIR/observer.csr -subj "/C=FR/O=Client/OU=Observer/CN=Observer 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/observer_conf.ext -days 30 -in $CERTDIR/observer.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/observer_cert.pem

# Unauthorized 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/impostor_key.pem
openssl req -new -SHA256 -key $CERTDIR/impostor_key.pem -nodes -out $CERTDIR/impostor.csr -subj "/C=FR/O=Client/OU=Impostor/CN=Impostor 1/"
openssl x509 -req -SHA256 -extfile $CONFDIR/impostor_conf.ext -days 30 -in $CERTDIR/impostor.csr -CA $CERTDIR/client_ca_cert.pem -CAkey $CERTDIR/client_ca_key.pem -CAcreateserial -out $CERTDIR/impostor_cert.pem

# Self-signed client 
openssl ecparam -genkey -name prime256v1 -out $CERTDIR/selfsignedcli_key.pem
openssl req -x509 -new -SHA256 -nodes -key $CERTDIR/selfsignedcli_key.pem -days 30 -out $CERTDIR/selfsignedcli_cert.pem -subj "/C=FR/O=Client/OU=Admin/CN=localhost/"


# Cleanup
rm $CERTDIR/*.csr $CERTDIR/*.srl