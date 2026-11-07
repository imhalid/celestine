package main

// version is the program's version, put in at the build:
//
//	go build -ldflags "-X main.version=v1.2.3"
//
// "dev" is a build from the source folder. It is in the crash logs and the backups' names, and the update
// check compares it with the latest release (update.go).
var version = "dev"
