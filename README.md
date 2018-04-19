# SkyBin

This repo contains the backend service implementations for SkyBin.

## Repo Structure

- `/cmd` contains command line tools.
- `/core` contains shared data structures.
- `/metaserver` contains the SkyBin metaserver.
- `/renter` contains the SkyBin renter service.
- `/provider` contains the SkyBin provider service.
- `/integration` contains integration tests. See the folder's README for more information.

## Getting Started

If you haven't installed Go, do so first [here](https://golang.org/doc/install).

Next, clone the repo: 

```
$ git clone https://github.com/uofu-skybin/skybin.git
```

Fetch dependencies:

```
$ cd skybin
$ go get
```

And finally build the skybin binary, which contains command line
entry points to start and interact with SkyBin's services.

```
$ go build -o skybin
```

Run a test network or integration tests to become more familiar with the system.
See the `/integration` folder for more information.

