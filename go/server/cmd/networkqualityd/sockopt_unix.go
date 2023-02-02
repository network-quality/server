// Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

//go:build !windows
// +build !windows

package main

import (
	"log"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func setTCPNotSentLowat(conn syscall.RawConn, value int) error {
	var setsockoptErr error
	if err := conn.Control(func(fd uintptr) {
		setsockoptErr = syscall.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NOTSENT_LOWAT, value)
	}); err != nil {
		return err
	}
	return setsockoptErr
}

func setIPTos(network string, conn syscall.RawConn, value int) error {
	var setsockoptErr error
	if err := conn.Control(func(fd uintptr) {
		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TOS, value); err != nil {
			log.Fatalf("failed to configure IP_TOS: %v", os.NewSyscallError("setsockopt", err))
		}
		if strings.HasSuffix(network, "4") {
			return
		}

		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_TCLASS, value); err != nil {
			log.Printf("Error setting IPV6_TCLASS: %v", os.NewSyscallError("setsockopt", err))
		}
	}); err != nil {
		return err
	}
	return setsockoptErr
}
