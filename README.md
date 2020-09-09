# Avoca

[![PkgGoDev](https://pkg.go.dev/badge/vthiery/avoca)](https://pkg.go.dev/github.com/vthiery/avoca)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/vthiery/avoca.svg)](https://github.com/vthiery/avoca)
[![GoReportCard example](https://goreportcard.com/badge/github.com/vthiery/avoca)](https://goreportcard.com/report/github.com/vthiery/avoca)
![Build Status](https://github.com/vthiery/avoca/workflows/Test/badge.svg)
![GolangCI Lint](https://github.com/vthiery/avoca/workflows/GolangCI/badge.svg)
![License](https://img.shields.io/github/license/vthiery/avoca)

## Description

Yet another HTTP client \o/

## Installation

```sh
go get -u github.com/vthiery/avoca
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vthiery/avoca"
	"github.com/vthiery/retry"
)

func main() {
	client := avoca.NewClient(
		avoca.WithHTTPClient(
			&http.Client{
				// Timeout the HTTP connection after 200 ms.
				Timeout: 200 * time.Millisecond,
			},
		),
		// Perform maximum 10 attempts with an exponential backoff.
		avoca.WithRetrier(
			retry.New(
				retry.WithMaxAttempts(10),
				retry.WithBackoff(
					retry.NewExponentialBackoff(
						100*time.Millisecond, // minWait
						1*time.Second,        // maxWait
						2*time.Millisecond,   // maxJitter
					),
				),
			),
		),
		// Only retry when the status code is >= 500
		avoca.WithRetryPolicy(
			func(statusCode int) bool {
				return statusCode >= http.StatusInternalServerError
			},
		),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res, err := client.Get(ctx, "http://google.com", nil)
	if err != nil {
		fmt.Printf("failed to GET: %v\n", err)
	}
	defer res.Body.Close()
}
```
