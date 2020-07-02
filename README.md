# Ferrum

This repository contains a demo backend server application for managing patient
records.

## Features

- It implements the following REST API:

| Method | URL                  | Description                                  | Auth-protected |
|--------|----------------------|----------------------------------------------|----------------|
| GET    | /health              | Get service health                           | No             |
| GET    | /generate-token      | Generate a valid JWT token for the API calls | No             |
| GET    | /api/v1/patients     | Get all patients                             | Yes            |
| GET    | /api/v1/patients/:id | Get one patient                              | Yes            |
| POST   | /api/v1/patients     | Add one patient                              | Yes            |

- The /api endpoint is auth-protected via JWT tokens.

- The request and response bodies are in JSON format

- It uses an exponential backoff algorithm for establishing the database connection
in case it takes a while for the database to come online or if connecting to it
is slow.

- It uses [docker healthchecks](https://docs.docker.com/engine/reference/builder/#healthcheck)

- It injects the current version into the executable binary at build time and it
reports it via the health check.

- It produces a single static binary that is stored in an Alpine Linux docker
container.

- It has unit and integration tests (see [http_test.go](server/http_test.go) and
[integration_test.sh](integration_test.sh)).

- It has an MIT [LICENSE](LICENSE)

- It follows the [Twelve-Factor App](https://12factor.net/) guidelines:
    1. Codebase: One version-controlled codebase in this repo.
    2. Dependencies: Its direct 3rd party dependencies are listed in [go.mod](go.mod).
    3. Config: Its configuration is done through environment variables (see the configuration section below).
    4. Backing services: This app requires access to a Postgres database, to which it connects via the configured.
    parameters. Both this app and the database can be launched via [docker-compose](docker-compose.yml).
    5. Build, release, run: Out of scope for this demo, but I could, for example, use
    [GitHub Actions](https://github.com/features/actions) to build a CI/CD pipeline which triggers on every pull request,
    builds a new version of the [docker](Dockerfile) container and releases the new version into a cloud-based test
    environment, for example on Google Cloud Platform, once the pull request is merged.
    6. Processes: The app is executed as one or more stateless processes. Since the patient data is stored in a database,
    multiple instances of this app can be deployed simultaneously to offer high availability and horizontal scaling.
    Some caching layer(s) should be considered, depending on the workflows and traffic.
    7. Port binding: The app exposes its HTTP API through an embedded HTTP server which listens on a configurable port.
    8. Concurrency: As mentioned under 6., this app can be scaled out by running multiple instances of it simultaneosly.
    9. Disposability: The Go runtime is quite fast, so this app usually starts up in less than a second if the database
    is running and accepting connections immediately. This app is also designed to shut down gracefully. When it
    receives either SIGINT or SIGTERM, it rejects any new incoming connections and waits for the connections that are
    currently in flight to finish before shutting down (see [main.go](cmd/ferrum/main.go) for details).
    10. Dev/prod parity: Out of scope for this demo, but as mentioned under 5., multiple cloud-based environments can
    be provisioned for this app and have the CI/CD pipeline trigger automatic deploys to each of them as needed.
    11. Logs: This app emits logs on STDOUT which get collected by docker in the docker logs. A system such as
    [logspot](https://github.com/gliderlabs/logspout) can be put in place to collect these logs and send them to a
    centralised aggregator, or, as is popular nowadays, a 3rd party cloud-based log management system can be used (for
    example Sumo Logic), which provides a log collector that is capable of picking up all docker logs and streaming them
    directly to their cloud for storage and aggregation.
    12. Admin processes: This app doesn't require such processes and it uses the docker Alpine Linux container as a base
    layer which contains the `ferrum` server binary. It provides various tools for debugging and troubleshooting issues,
    most of which needing to be installed before they can be used. I could also use the docker `scratch` container as a
    base layer instead of Alpine Linux, but this doesn't have `wget` in it (because it's empty), which is needed by the
    docker healthcheck to determine if the `ferrum` process is up and running.

## Build, test and run instructions

Please note that I have tested this app only on OSX, but I expect it to work
just fine on Linux as well.

It is assumed that you have Go 1.14+ and Docker 2.2.0.5+ installed locally and
configured correctly.

```shell
> # A few 3rd party Go dependencies need to be installed first.
> make install-deps
> # If desired, the database layer code generator can be executed again.
> # I have committed the output in git, since the files are quite small.
> make generate
> # Run linters on the code to make sure everything is sane.
> make lint
> # Run the unit tests.
> make test
> # Build the actual executable if you want to try launching it directly from
> # your console.
> # This will create an executable called `ferrum` in the root directory of the repo.
> make build
> # Build the `mihaitodor/ferrum` docker image.
> # (or you can let docker-compose build it for you when it starts up)
> make docker
> # Launch the `mihaitodor/ferrum` and `postgres` containers via docker-compose.
> make up
> # Feel free to use `docker ps` to observe the container healthchecks turn healthy.
> docker ps
> # Stop docker-compose and tear down the containers once you're finished with them.
> make down
> # Run the integration tests.
> # This will run a hopefully-not-too-ugly bash script (integration_test.sh).
> make test-integration
```

## Configuration

- `FERRUM_DATABASE_HOST`:        The host for the database server (default `localhost`)
- `FERRUM_DATABASE_PORT`:        The port for the database server (default `5432`)
- `FERRUM_DATABASE_USER`:        The user for the database server (default `postgres`)
- `FERRUM_DATABASE_PASSWORD`:    The password for the database server (default `postgres`)
- `FERRUM_DATABASE_NAME`:        The database name (default `ferrum`)
- `FERRUM_HTTP_API_PORT`:        The embedded HTTP server port (default `80`)
- `FERRUM_HTTP_REQUEST_TIMEOUT`: The maximum HTTP request timeout (default `3s`)
- `FERRUM_HTTP_MAX_POST_SIZE`:   The maximum POST request content size (default `1MiB`)
- `FERRUM_HTTP_JWT_SIGNING_KEY`: The JWT token signing key (default `deadbeef`)
- `FERRUM_HTTP_JWT_CLAIM_NAME`:  The JWT token claim name (default `ferrum`)
- `FERRUM_HTTP_JWT_EXPIRATION`:  The JWT token expiration (default `1h`)
- `FERRUM_LOG_LEVEL`:            The logging level (default `info`)

## Ports

This app listens on port 80 by default, which is also the port exposed by the
docker container.

## Performance

Go produces very efficient code with static binaries and very fast runtime. It
does, however have a lightweight garbage collector, which places it somewhere in
between C++ / Rust and Java / C# in terms of raw performance. Considering the
difficulty of writing such a service in a language like C++, Go is highly
suitable for developing small web microservices and, alongside the generous Go
standard library, there exists a plethora of open source libraries and
frameworks which enable developing such services quickly and efficiently.

## TODO

Happy to drill deeper into any of these and maybe even implement some if you'd
like me to. I just need more time.

- Add more unit tests to have better coverage for corner cases and some of the
code paths which aren't currently being exercised.

- I'd write the integration tests in Python, or, if this app becomes more complex
and requires more dependencies, I would investigate using a framework such as
[testcontainers-go](https://github.com/testcontainers/testcontainers-go).

- Add performance unit tests and perform some load testing using a tool such as
[hey](https://github.com/rakyll/hey) or [locust](https://locust.io/).

- Deploying to a cloud environment: While outside of the scope of this assignment,
I'm happy to write some Terraform code which can orchestrate the deployment of
this service in a cloud provider (prefferably AWS, since I have extensive
experience with it).

- Monitoring: While outside of the scope of this assignment, monitoring is
a very important topic. For production readiness, one open source possibility is
to embed the [Prometheus agent](https://github.com/prometheus/client_golang)
into the code and have it send metrics to a Prometheus server. Then, for example
using [Grafana](https://grafana.com/) I'd implement some charts which show
several key metrics over time: CPU and memory usage, various HTTP status counts,
container restarts (assuming we're running multiple instances), request
latencies (median for each relevant percentile), garbage collector stats, etc.

- Caching: Depending on the intended usage and performance requirements, some of
the lookup results from the database can be cached either in memory or on disk
or via a separate service.

- Use a process manager in the container, such as [s6](https://skarnet.org/software/s6/)
In some cases, the docker container can be heavyweight and it might also keep
some state, such as a large cache, in which case it can be desirable to have a
process manager restart the app immediately inside the container in case it dies
for whatever reason.

- Run the process in the container as a regular user instead of `root`. There
are some valid security concerns with allowing processes to run as root in
docker containers. Using `scratch` as a base layer makes it difficult to run
as non-root, but Alpine Linux is a very small image, so this can be a good
tradeoff if this configuration is mandated.

- Implement CORS properly. Adding one patient via the `/api/v1/patients` endpoint
requires a preflight `OPTIONS` request before the actual `POST` request is issued
when run from a browser. This requires special handling of certain headers, which
I have implemented, but it's probably insecure. This needs to be reviewed in
detail.

- Rate limiting might be nice to have, although that can be achieved via the
service mesh proxies.

- The healthcheck should be enhanced and perhaps split up into two separate
probes: `livenessProbe` and `readinessProbe`. This would be useful if this app
will be run in a Kubernetes cluster because K8s will be able to make better
scheduling decisions if the `livenessProbe` reports healthy as soon as the
process is running, while it waits for the `readinessProbe` to report OK after
the connection to the database has been established.

- Investigate what happens when the database connection drops. Does it reconnect
automatically?

- Mask secrets when dumping the configuration via [rubberneck](github.com/relistan/rubberneck).

- Input data validation. There is some implicit validation being done, by the
JSON serialisation / deserialisation and by the database engine, but more is
required.