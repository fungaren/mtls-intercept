# mtls-intercept

[![PkgGoDev](https://pkg.go.dev/badge/github.com/fungaren/mtls-intercept)](https://pkg.go.dev/github.com/fungaren/mtls-intercept)

mtls-intercept is a reverse proxy to decrypt mTLS protected traffics.

## Motivation

[mitmproxy](https://github.com/mitmproxy/mitmproxy) is a nice tool for debugging TLS encrypted traffics, and it is enough for most use case.
However in Kubernetes clusters, the kube-apiserver is a REST server with RBAC enabled, where mTLS is used to indicate the user/client.

Currently mitmproxy have `options.client_certs` as a per-site config to enable mTLS. However, when mitmproxy is working as
a reverse proxy for a single server, there is no way for us to generate client certificates for each client.

I had made a [pull request](https://github.com/mitmproxy/mitmproxy/pull/6430/commits) to
mitmproxy but it is not accepted until now, therefore I decide to write a dedicated tool on my own.

[bettercap](https://github.com/bettercap/bettercap) is another awesome hack tool supporting MiTM attacks, and written in Go. It is developed on top of [goproxy](https://github.com/elazarl/goproxy), which has poor support for mTLS and long-connection requests.

## Get started

You can clone the repository to local then compile from the source, or simply run:

```bash
go install github.com/fungaren/mtls-intercept@latest
```

We also provide a pre-built container image `ghcr.io/fungaren/mtls-intercept:0.0.1`

```
Usage:
  mtls-intercept [flags]

Flags:
      --client-ca-cert string   client ca certificate (default "./certs/client-ca.crt")
      --client-ca-key string    client ca key (default "./certs/client-ca.key")
  -h, --help                    help for mtls-intercept
      --plugins stringArray     enable plugins (k8sapiserver)
  -p, --port int                listen port (default 8080)
      --server-ca-cert string   server ca certificate (default "./certs/server-ca.crt")
      --server-ca-key string    server ca key (default "./certs/server-ca.key")
  -u, --upstream string         upstream server:port
  -v, --verbose                 verbose output
      --version                 version for mtls-intercept
```

## Demo

### Decrypt traffics from/to Kubernetes API server

Assumes you already have a [K3s](https://k3s.io) based cluster.

```bash
mkdir certs
# Get apiserver CA certificate
cp /var/lib/rancher/k3s/server/tls/server-ca.crt ./certs/
cp /var/lib/rancher/k3s/server/tls/server-ca.key ./certs/
# Get client CA certificate
cp /var/lib/rancher/k3s/server/tls/client-ca.crt ./certs/
cp /var/lib/rancher/k3s/server/tls/client-ca.key ./certs/

mtls-intercept -p 8080 -u 192.168.64.2:6443
```

Access the cluster via:

```bash
kubectl get nodes --server https://127.0.0.1:8080
```

### General mTLS server

```bash
mkdir certs

# Generate a self-signed root CA for servers.
openssl req -new -x509 -newkey rsa:2048 -nodes -utf8 -sha256 -days 36500 \
  -subj "/CN=server-ca" -outform PEM -out ./certs/server-ca.crt -keyout ./certs/server-ca.key

# Generate a self-signed root CA for clients.
openssl req -new -x509 -newkey rsa:2048 -nodes -utf8 -sha256 -days 36500 \
  -subj "/CN=client-ca" -outform PEM -out ./certs/client-ca.crt -keyout ./certs/client-ca.key

# Generate the server cert.
cat > ./certs/server-csr.conf <<EOF
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[ dn ]
CN = mtls-server

[ req_ext ]
subjectAltName = @alt_names

[ alt_names ]
DNS.1 = example.org
DNS.2 = localhost
IP.1 = 127.0.0.1
IP.2 = 0:0:0:0:0:0:0:1

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment
extendedKeyUsage=serverAuth
subjectAltName=@alt_names
EOF
openssl genrsa -out ./certs/server.key 2048
openssl req -new -key ./certs/server.key -out ./certs/server.csr -config ./certs/server-csr.conf
openssl x509 -req -in ./certs/server.csr -CA ./certs/server-ca.crt -CAkey ./certs/server-ca.key \
    -CAcreateserial -out ./certs/server.crt -days 36500 \
    -extensions v3_ext -extfile ./certs/server-csr.conf -sha256

# Generate the client cert.
cat > ./certs/client-csr.conf <<EOF
[ req ]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn

[ dn ]
O = world
CN = hello

[ v3_ext ]
authorityKeyIdentifier=keyid,issuer:always
basicConstraints=CA:FALSE
keyUsage=keyEncipherment,dataEncipherment
extendedKeyUsage=clientAuth
EOF
openssl genrsa -out ./certs/client.key 2048
openssl req -new -key ./certs/client.key -out ./certs/client.csr -config ./certs/client-csr.conf
openssl x509 -req -in ./certs/client.csr -CA ./certs/client-ca.crt -CAkey ./certs/client-ca.key \
    -CAcreateserial -out ./certs/client.crt -days 36500 \
    -extensions v3_ext -extfile ./certs/client-csr.conf -sha256
```

Start the mTLS server:

```bash
# openssl s_server -port 4433 -www -verifyCAfile ./certs/client-ca.crt -cert ./certs/server.crt -key ./certs/server.key
go run example/server.go

mtls-intercept -p 8080 -u 127.0.0.1:4433
```

Send request to the proxy:

```bash
curl -v --cacert ./certs/server-ca.crt --cert ./certs/client.crt --key ./certs/client.key https://127.0.0.1:8080
```

## Plugins

You can pass `--plugins` to the command line to enable plugins.

| Plugin         | Description             |
|----------------|-------------------------|
| k8sapiserver   | Count inbound/outbound bytes and requests to/from kube-apiserver, and expose Prometheus style metrics. |

Contributions are welcomed.

## License

Apache-2.0
