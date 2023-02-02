// Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
)

func setTCPNotSentLowat(conn syscall.RawConn, value int) error {
	return fmt.Errorf("platform not supported")
}

func setIPTos(network string, conn syscall.RawConn, value int) error {
	return fmt.Errorf("platform not supported")
}
