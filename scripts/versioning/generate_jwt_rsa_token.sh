#!/bin/bash

# Private key
openssl genrsa -out rsa-2048-private.pem 2048

# Public key
openssl rsa -in rsa-2048-private.pem -pubout > rsa-2048-public.pub
