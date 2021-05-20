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

### Usage

#### Service

In order to run the service, from the `./bin` directory, you can run:

```bash
$ ./worker -port=7777 -cert=cert.pem -key=key.pem -ca=client_ca1.pem client_ca2.pem
```

* `-port` specifies the port that you want to run your server.
* `-cert` specifies the location of certificate that you will use for the server
* `-key`  specifies the location of private key associates with the certificate
* `-ca`   specifies the list of trusted client certificate authorities. 

#### Client CLI

Come along with the service is the client CLI for user to access to the service. from the `./bin` directory, you can run the client with `./worker_cli`. Same as the service, the basic command line options of the client are: 
* `-a` specifies the address of the server that we want to access. Ex: `-a=domain.name:port`
* `-cert` specifies the location of certificate that you will use for the client. Ex: `-cert=cert.pem`
* `-key`  specifies the location of private key associates with the certificate. Ex: `-key=key.pem`
* `-ca`   specifies the trusted server certificate authorities. Ex:

For each supported functionalities, we have the following subcommands:

* `help` provides the detail instructions on how to use the CLI.
* `start` starts a job on the server. Come along with `start` is the option `-cmd`, which specifies the command to run and its arguments. Ex: `-cmd="ls" "-la"`
* `stop` stops a job on the server. Come along with `stop` is the option `-job`, which specifies the job ID, and an optional flag `-force` to force the job to terminate immediately. 
* `query` queries the status of a job on the server. Come along with `query` is the option `-job`, which specifies the job ID.
* `stream` start a stream of log of a job on the server. Come along with `stream` is the option `-job`, which specifies the job ID.

Examples:

```bash
# help
$ ./worker_cli help
# start
$ ./worker_cli start -a=domain.name:port -cert=cert.pem -key=key.pem -ca=server_ca.pem -cmd=”ls” “-la”
# stop
$ ./worker_cli stop -force -a=domain.name:port -cert=cert.pem -key=key.pem -ca=server_ca.pem -job=job-id
# query
$ ./worker_cli query -a=domain.name:port -cert=cert.pem -key=key.pem -ca=server_ca.pem -job=job-id
# stream 
$ ./worker_cli stream -a=domain.name:port -cert=cert.pem -key=key.pem -ca=server_ca.pem -job=job-id
```

## More Information

### Design documentation

The design documentation of this project can be found at <https://github.com/MinhNghiaD/jobworker/blob/master/docs/design/worker_design.pdf> 
