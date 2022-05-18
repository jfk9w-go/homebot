# homebot

[![test](https://github.com/jfk9w-go/homebot/actions/workflows/test.yml/badge.svg)](https://github.com/jfk9w-go/homebot/actions/workflows/test.yml)

A collection of everyday utilities with Telegram bot as frontend.

## Installation

Build from sources (requires Go 1.18):

```bash
$ git clone https://github.com/jfk9w-go/homebot.git
$ go build -o homebot
$ ./homebot --help
```

Alternatively, you can use Docker image:

```bash
$ docker pull ghcr.io/jfk9w-go/homebot:master 
$ docker run ghcr.io/jfk9w-go/homebot:master --help
```

## Configuration

The bot is configured with a YAML file. You can obtain configuration schema with `--config.schema=yaml` CLI option.
The `/start` command allows you to detect your user ID and the chat ID you're currently in.

You can use `--config.values=yaml` CLI option in order to generate configuration template.
Please refer to configuration schema to fill configuration values properly.

Configuration file(s) is (are) passed to the executable using `--config.file=<file_path>` option.
Alternatively, you can use environment variables for configuration (see `--help` for more details).

Minimal viable YAML configuration:

```yaml
telegram:
  token: "your_telegram_bot_api_token"
```

The same configuration passed as env to Docker image:

```bash
$ docker run -e homebot_telegram_token='your_telegram_bot_api_token' ghcr.io/jfk9w-go/homebot:master
```

### tinkoff

This extension provides the ability to synchronize your Tinkoff bank and trading operations to a PostgreSQL database
instance.

Exposes `/update_bank_statement` command.

#### Configuration

Note that in order to use this extension you should encode your banking credentials in a Gob format.

This is done using `--tinkoff.encode=gob` CLI option. Example:

```bash
$ docker run \
  -e homebot_tinkoff_credentials_<your_tg_user_id>_username='your_username' \
  -e homebot_tinkoff_credentials_<your_tg_user_id>_phone='phone_number_used_for_tinkoff_login' \
  -e homebot_tinkoff_credentials_<your_tg_user_id>_password='password_used_for_tinkoff_login' \
  ghcr.io/jfk9w-go/homebot:master --tinkoff.encode=gob > credentials.gob # generate credentials file
$ docker run \
  -v $PWD/config.yml:/config.yml:ro \
  -v $PWD/credentials.gob:/credentials.gob:ro \
  ghcr.io/jfk9w-go/homebot:master --config.file=/config.yml --config.file=/credentials.gob --config.values=yaml # check configuration values
$ docker run \
  -v $PWD/config.yml:/config.yml:ro \
  -v $PWD/credentials.gob:/credentials.gob:ro \
  ghcr.io/jfk9w-go/homebot:master --config.file=/config.yml --config.file=/credentials.gob # run application
```

### hassgpx

This extension provides the ability to get a GPX track from your Home Assistant location data.
The GPX track is generated only with data inside the current UTC day. Some assumptions are made (see configuration below
for details).

Exposes `/get_gpx_track` command.
