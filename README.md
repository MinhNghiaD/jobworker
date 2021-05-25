# Job Worker

## About 

Job worker is a service that provides user the APIs to run arbitrary processes on Linux environment in a secure way. The prototype support the following features:

* Start a job which executes a linux command on a remote linux environment.
* Stop a scheduled job.
* Query the current status of a scheduled job.
* Stream logs of a scheduled job.

Job worker is written in Golang, with gRPC APIs. The communication is secured with mTLS authentication.

## Table of contents

> * [Job Worker](#job_worker)
>   * [About](#about)
>   * [Table of contents](#table-of-contents)
>   * [Building from source](#building-from-source)
>     * [Test](#test) 
>     * [Usage](#usage)
>       * [Service](#service)
>       * [Client CLI](#client-cli)
>   * [More Information](#more-information)
>     * [Design documentation](#design-documentation)


## Building from source

The Job worker source code contains the worker library and a user CLI written in Golang. Make sure you have Golang v1.16 or newer, then run:


```bash
# get the source & build:
$ git clone https://github.com/MinhNghiaD/jobworker.git
$ cd jobworker
$ make build
```

If the build succeeds, the binaries can be found in the following directory: `./bin`

### Test

To run all tests, please run `make test`

### Security configuration
#### mTLS 
To ensure the integrity of gRPC commucation, we use TLS 1.3 for our mTLS configuration. With Golang, the current cipher suites supported are: 
* TLS_AES_128_GCM_SHA256
* TLS_AES_256_GCM_SHA384
* TLS_CHACHA20_POLY1305_SHA256

#### Authentication

For this service, 256-bit ECDSA is reasonable choice for private key and certificate encryption. Compared to the traditional RSA, ECDSA provides better security, with smaller keys and better performance.

To generate the template certificates for testing, please run `make cert`

#### Authorization
For authorization, we use Role-based access control to limit user access. The current roles supported are:
* Admin: Gain access right to start/stop/query/stream all jobs in the system.
* Observer: Gain read-only access to query job status and stream log.

As a tradeoff of this project, we borrow the organizational_unit field of the x.509 certificate to encode the value of user role. For future implementation, RBAC should be implement properly with credential-based authorization.


### Usage

#### Service

In order to run the service, from the `./bin` directory, you can run

```bash
usage: worker [<flags>]

Flags:
  --help                 Show context-sensitive help (also try --help-long and --help-man).
  --port=7777            server port
  --cert=server_cert.pem server certificate
  --key=server_key.pem   server private key
  --ca                   list of trusted client certificate authorities.

```

#### Client CLI

Come along with the service is the client CLI for user to access to the service. from the `./bin` directory, you can run the client with `./worker_cli`.

```bash
usage: worker_cli [<flags>] <command> [<args> ...]

Flags:
  --help                 Show context-sensitive help (also try --help-long and --help-man).
  --a="127.0.0.1:7777"   server address
  --cert=client_cert.pem client certificate
  --key=client_key.pem   client private key
  --ca                   server certificate authority

Commands:
  help [<command>...]
    Show help.


  start [<flags>]
    Start a job on worker service.

    --cmd=""  command to be executed

  stop [<flags>]
    Stop a job on worker service.

    --force   force job to terminate immediately
    --job=""  job id

  query [<flags>]
    Query status of a job on worker service.

    --job=""  job id
  
  stream [<flags>]
    Stream log of a job on worker service.

    --job=""  job id
```

For examples:
```bash
$ ./worker_cli --a=127.0.0.1:7777 --key=client_key.pem --key=client_key.pem start --ca=server_ca.pem --cmd=bash -- "-c" "trap -- SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"
```

## More Information

### Design documentation

The design documentation of this project can be found at <https://github.com/MinhNghiaD/jobworker/blob/master/docs/design/worker_design.pdf> 
