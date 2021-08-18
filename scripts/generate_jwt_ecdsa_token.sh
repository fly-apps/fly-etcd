#!/bin/bash

# ES256
# Private key
openssl ecparam -genkey -name prime256v1 -noout -out ecdsa-p256-private.pem

# Public key
openssl ec -in ecdsa-p256-private.pem -pubout -out ecdsa-p256-public.pem
