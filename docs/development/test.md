# Development Setup
This section describes necessary steps to setup a test environment for running automation tests.


## Requirements
* `go` >= 1.20
* `git`, `make` and `kubectl`
* [metalbond](https://github.com/ironcore-dev/metalbond)
* [dp-service](https://github.com/ironcore-dev/net-dpservice)

## Metalbond and dp-service
To start metalbond server, we use the following command. Switch to metalbond's repo and run
```sh
./metalbond server --listen [::]:4711 --http [::1]:4712 --keepalive 3
```

To start dp-service, we use the following command. In contrast to running a dp-service in the default mode that operates on virtual functions, it needs to be started in the TAP mode that operates on linux tap devices.
```sh
sudo ./test/dp_service.py --no-init
```

## Run automation tests
Invoke automation tests by running
```sh
make test
```

## Common issues
### Residual claiming file
If automation tests fails or gets panic during execution, the interface claiming file under repository `/tmp/var/lib/metalnet` could be residual on the disk. Thus, if the following error appears, consider removing the files under this repository.

```
[FAILED] in [BeforeSuite] - /home/tli/go/src/github.com/onmetal/metalnet/controllers/suite_test.go:111 @ 12/01/23 13:59:41.01
[BeforeSuite] [FAILED] [4.845 seconds]
[BeforeSuite] 
/home/tli/go/src/github.com/onmetal/metalnet/controllers/suite_test.go:79

  [FAILED] Unexpected error:
      <*errors.errorString | 0xc0005aa530>: 
      claim 5e4c2887-19fe-4295-bba7-c4476d566a3f cannot claim non-existent address ::::net_tap5..
      {
          s: "claim 5e4c2887-19fe-4295-bba7-c4476d566a3f cannot claim non-existent address ::::net_tap5..",
      }
  occurred
  In [BeforeSuite] at: /home/tli/go/src/github.com/onmetal/metalnet/controllers/suite_test.go:111 @ 12/01/23 13:59:41.01
```


