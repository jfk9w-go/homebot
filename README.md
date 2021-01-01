## bank-statement

[![Go Reference](https://pkg.go.dev/badge/github.com/jfk9w-go/bank-statement.svg)](https://pkg.go.dev/github.com/jfk9w-go/bank-statement)

This simple utility is designed for exporting bank statements from
multiple internet bank accounts.

At the moment Tinkoff (all time data) and Alfa-Bank (last two years data) are supported.

### Requirements

We use Selenium and Chrome driver for logging in to internet banking accounts,
so these obviously have to be installed. 

#### Mac OS (homebrew)
```code:bash
$ brew install selenium-server-standalone chromedriver
```

Note that you also have to keep your phone close in order 
to receive and enter the confirmation code which will be requested for authorization.

### Installation

You can either download a binary for Mac OS from [releases page](https://github.com/jfk9w-go/bank-statement/releases)
or use go to install the package and run it from GOPATH:
```code:bash
$ go install github.com/jfk9w-go/bank-statement
$ $GOPATH/bin/bank-statement config.yml
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

You can find a more complete configuration explanation in [godoc](https://pkg.go.dev/github.com/jfk9w-go/bank-statement).