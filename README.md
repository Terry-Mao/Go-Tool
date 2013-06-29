## Terry-Mao/Go-Tool

`Terry-Mao/Go-Tool` is a tool box for golang.

## Requeriments

Golang evn is required.

## Installation
Just pull `Terry-Mao/Go-Tool` from github using `go get`:

```sh
$ go get github.com/Terry-Mao/Go-Tool
# Enter sub dir (etc auto-model) then go install
```

## Usage

`auto-model` is a auto-generation struct tool for golang models which read `mysql` information_schema.columns
```
$ go get github.com/go-sql-driver/mysql
$ cd auto-model
$ go intall
$ cd ${GOPATH}/bin
# for a help
$ ./auto-model -h
```
