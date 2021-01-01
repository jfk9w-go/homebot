## bank-statement

[![GoDoc](https://pkg.go.dev/github.com/jfk9w-go/bank-statement?status.svg)](https://pkg.go.dev/github.com/jfk9w-go/bank-statement)

This simple utility is designed for exporting bank statements from
multiple internet bank accounts.

At the moment Tinkoff and Alfa-Bank are supported.

### Requirements

We use Selenium and Chrome driver for logging on to internet banking accounts,
so these obviously have to be installed.

#### Mac OS (homebrew)
```code:bash
$ brew install selenium-server-standalone chromedriver
```

### Configuration & execution

The program uses a single YAML configuration file. Its path must be passed as
a first command line argument to the binary.

Configuration example:
```code:yaml
# Selenium configuration parameters examples are provided for Mac OS.
selenium:
  jar: /usr/local/Cellar/selenium-server-standalone/3.141.59_2/libexec/selenium-server-standalone-3.141.59.jar
  chrome: /usr/local/bin/chromedriver
  wait_timeout: 10s
# Use this if you want to export to Postgres.
# output: postgresql://user:pass@host:port/db
output: statement.json
# Uncomment for updating MCC codes.
# mcc: https://raw.githubusercontent.com/greggles/mcc-codes/main/mcc_codes.csv
tinkoff:
  - username: your_tinkoff_username
    password: your_tinkoff_password
    accounts:
      Black RUB: 0000000000
      Black USD: 1111111111
      All Airlines: 2222222222
      Savings RUB: 3333333334
      Deposit RUB: 4444444444
alfa:
  - username: your_alfa_username
    password: your_alfa_password
    accounts:
    - 00000000000000000000
    ...
```

You can find a more complete configuration explanation in godoc.