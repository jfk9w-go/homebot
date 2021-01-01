package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/jfk9w-go/bank-statement/statement"
	"github.com/jfk9w-go/bank-statement/statement/alfa"
	"github.com/jfk9w-go/bank-statement/statement/tinkoff"
	"github.com/jfk9w-go/flu"
	"github.com/jfk9w-go/flu/csv"
	fluhttp "github.com/jfk9w-go/flu/http"
)

type Config struct {
	Selenium statement.SeleniumConfig `yaml:"selenium"`
	// Output is a report output destination.
	// It can either be:
	// a) a Postgres database connection string "postgresql://user:pass@host:port/db"
	// b) a path to a JSON file, ending with ".json" extension
	Output string `yaml:"output"`
	// If specified, MCC codes will be updated using this resource.
	// Note that since the MCC CSV format is pretty specific, this
	// can be basically be either an empty value (or not specified at all),
	// or https://raw.githubusercontent.com/greggles/mcc-codes/main/mcc_codes.csv.
	// Also note that in case of a database output "mcc" table will always be created.
	MCC string `yaml:"mcc"`
	// Tinkoff is a list of Tinkoff bank configurations.
	Tinkoff []tinkoff.InternetBankConfig `yaml:"tinkoff"`
	// Alfa is a list of Alfa bank configurations.
	Alfa []alfa.InternetBankConfig `yaml:"alfa"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var config Config
	if err := flu.DecodeFrom(flu.File(os.Args[1]), flu.YAML{Value: &config}); err != nil {
		panic(err)
	}

	var output statement.BatchOutput
	var err error
	if strings.HasPrefix(config.Output, "postgres") {
		output, err = statement.NewDatabase(config.Output,
			new(statement.MerchantCategoryCode),
			new(statement.BankStatement))
	} else if strings.HasSuffix(config.Output, ".json") {
		output, err = statement.NewJSONOutput(flu.File(config.Output))
	} else {
		panic("unsupported output: " + config.Output)
	}

	if err != nil {
		panic(err)
	}

	defer flu.Close(output)

	if config.MCC != "" {
		log.Printf("Updating merchant category codes")
		mccs := make(statement.MerchantCategoryCodeDictionary)
		if err := flu.DecodeFrom(flu.URL(config.MCC), &csv.Codec{Output: mccs, Comma: ','}); err != nil {
			panic(err)
		}
		if err := output.Update(mccs); err != nil {
			panic(err)
		}
	}

	service := &statement.Service{
		Auth:     statement.BasicTwoFactorAuthentication{},
		Selenium: config.Selenium,
	}

	banks := make([]statement.Bank, 0)
	for _, config := range config.Tinkoff {
		bank := tinkoff.InternetBank{
			Client: fluhttp.NewClient(nil),
			Config: config,
		}

		banks = append(banks, bank)
		log.Printf("Added %s username %s to the list", bank.ID(), config.Username)
	}

	for _, config := range config.Alfa {
		bank := alfa.InternetBank{
			Config: config,
		}

		banks = append(banks, bank)
		log.Printf("Added %s username %s to the list", bank.ID(), config.Username)
	}

	if err := service.UpdateStatement(ctx, banks, output); err != nil {
		panic(err)
	}
}
