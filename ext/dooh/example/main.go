package main

import (
	"context"

	"github.com/jfk9w-go/homebot/ext/dooh"
	"github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()
	client := dooh.NewClient("i.kulkov@qvant.ru", "admin-")
	surfaces, err := client.Surfaces(ctx)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info(surfaces)
}
