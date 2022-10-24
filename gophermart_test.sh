#!/bin/bash
          ./gophermarttest_darwin_amd64 \
            -test.v -test.run=^TestGophermart$ \
            -gophermart-binary-path=cmd/gophermart/gophermart \
            -gophermart-host=localhost \
            -gophermart-port=8080 \
            -gophermart-database-uri="postgres://root:12345@localhost/gophermart?sslmode=disable" \
            -accrual-binary-path=cmd/accrual/accrual_darwin_amd64 \
            -accrual-host=localhost \
            -accrual-port=8081 \
            -accrual-database-uri="postgres://root:12345@localhost/gophermart?sslmode=disable"