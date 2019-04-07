#!/bin/bash

# go get github.com/google/pprof
go get github.com/gomodule/redigo/redis
go get github.com/go-martini/martini
go get github.com/go-sql-driver/mysql
go get github.com/martini-contrib/render
go get github.com/martini-contrib/sessions
go build -o golang-webapp .
