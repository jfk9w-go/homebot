package main

import (
	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/homebot/ext/tinkoff"
)

// Use this helper in order to encode your Tinkoff bank credentials with Gob.
func main() {
	file := flu.File("tdata.bin")
	credentials := tinkoff.CredentialStore{
		12345: tinkoff.Credential{
			Username: "username",
			Password: "password",
		},
	}

	if err := flu.EncodeTo(flu.Gob(credentials), file); err != nil {
		panic(err)
	}
}
