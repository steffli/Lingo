//go:build !windows

package main

import "log"

func runQuick(args []string) {
	log.Fatal("quick translate is only supported on Windows")
}
