# homebot

[![ci](https://github.com/jfk9w-go/homebot/actions/workflows/ci.yml/badge.svg)](https://github.com/jfk9w-go/homebot/actions/workflows/ci.yml)

A collection of everyday utilities with Telegram bot as frontend.

## Installation

Simply download matching binary from [releases page](https://github.com/jfk9w-go/homebot/releases).

Alternatively, you can use:

`$ go install github.com/jfk9w-go/homebot`

In this case the binary will be available in your **$GOPATH**.

## Configuration and execution

The bot is configured with a YAML file. 
The following is a minimal viable configuration which will allow the bot to start and respond to `/start`.
The `/start` command allows you to detect your user ID and the chat ID you're currently in.
It also provides the buttons for available commands.

```yaml
telegram:
  # Your Telegram Bot API token.
  token: "4uhgeuygsuyfljNBLIUWEG712313:325326"
logging:
  # Either "json" or "text".
  format: json
  # Lowest log level.
  # Available: TRACE, DEBUG, INFO, WARN, ERROR.
  level: INFO

# Extensions configuration.
# ...
```

Pass the configuration file location to the binary as a CLI argument:

`$ homebot config.yml`

## Extensions

The bot is built with extensible architecture in mind. Extensions are services which optionally may provide bot command handlers.
Extensions are configured in the YAML configuration as additional root nodes. 
If an extension configuration node is not present, the extension is disabled.

### tinkoff

This extension provides the ability to synchronize your Tinkoff bank and trading operations to a PostgreSQL database instance.

Exposes `/update_bank_statement` command.

#### Configuration

Note that in order to use this extension you must encode your banking credentials in a Gob format.
See [this helper](https://github.com/jfk9w-go/homebot/blob/master/ext/tinkoff/helper/main.go) for more info.

```yaml
tinkoff:
  # PostgreSQL connection URL.
  database: "postgresql://username:password@instance:5432/db"
  # Data reload interval. The default is 60 days.
  # Must use format 1h, 5m or 10s.
  reload: 168h
  # Location of credentials binary file generated with the helper mentioned above.
  data: tdata.bin
```

### hassgpx

This extension provides the ability to get a GPX track from your Home Assistant location data.
The GPX track is generated only with data inside the current UTC day. Some assumptions are made (see configuration below for details).

Exposes `/get_GPX_track` command.

#### Configuration

```yaml
hassgpx:
  # PostgreSQL connection URL (the same used in Home Assistant instance).
  database: "postgresql://username:password@instance:5432/db"
  # Max speed in km/h. The default is 55.
  # Track segments with speed higher than maximum will be ignored.
  # Note that this is an approximation via latitude and longitude.
  maxspeed: 55
  # Last days to process. The default is 0 (only today).
  # 1 would mean "process yesterday and today" and so forth.
  lastdays: 0
  # Move tracking interval. The default is 1 minute.
  # Must use format 1h, 5m or 10s.
  # Note that while OwnTracks for Android allows you to set move tracking interval,
  # it seems to always use 30 seconds.
  moveinterval: 1m
  # Telegram user ID => entity ID map.
  # We use "like" in order to match entity records.
  users:
    12345678: "device_tracker.my_phone"
```