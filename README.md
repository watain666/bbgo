# bbgo

A trading bot framework written in Go. The name bbgo comes from the BB8 bot in the Star Wars movie. aka Buy BitCoin Go!

## Current Status

_Working hard in progress_

[![Build Status](https://travis-ci.org/c9s/bbgo.svg?branch=main)](https://travis-ci.org/c9s/bbgo)

Aim to release v1.0 before 11/14

## Features

- Exchange abstraction interface
- Stream integration (user data websocket)
- PnL calculation.
- Slack notification

## Supported Exchanges

- MAX Exchange (located in Taiwan)
- Binance Exchange

## Installation

Install the builtin commands:

```sh
go get -u github.com/c9s/bbgo/cmd/bbgo
```

Add your dotenv file:

```
SLACK_TOKEN=

BINANCE_API_KEY=
BINANCE_API_SECRET=

MAX_API_KEY=
MAX_API_SECRET=

MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USERNAME=root
MYSQL_PASSWORD=
MYSQL_DATABASE=bbgo
MYSQL_URL=root@tcp(127.0.0.1:3306)/bbgo
```

You can get your API key and secret after you register the accounts:

- For MAX: <https://max.maicoin.com/signup?r=c7982718>
- For Binance: <https://www.binancezh.com/en/register?ref=VGDGLT80>

Then run the `migrate` command to initialize your database:

```sh
dotenv -f .env.local -- bbgo migrate up
```

There are some other commands you can run:

```sh
dotenv -f .env.local -- bbgo migrate status
dotenv -f .env.local -- bbgo migrate redo
```

(It internally uses `goose` to run these migration files, see [migrations](migrations))


To query transfer history:

```sh
dotenv -f .env.local -- bbgo transfer-history --exchange max --asset USDT --since "2019-01-01"
```

To calculate pnl:

```sh
dotenv -f .env.local -- bbgo pnl --exchange binance --asset BTC --since "2019-01-01"
```


## Examples

Please check out the example directory: [examples](examples)

Initialize MAX API:

```go
key := os.Getenv("MAX_API_KEY")
secret := os.Getenv("MAX_API_SECRET")

maxRest := maxapi.NewRestClient(maxapi.ProductionAPIURL)
maxRest.Auth(key, secret)
```

Creating user data stream to get the orderbook (depth):

```go
stream := max.NewStream(key, secret)
stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})

streambook := types.NewStreamBook(symbol)
streambook.BindStream(stream)
```

## New Trading Bot API design

_**still under construction**_

```go
package main

import (
    "github.com/c9s/bbgo"
)

mysqlURL := viper.GetString("mysql-url")
mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
db, err := sqlx.Connect("mysql", mysqlURL)

if err != nil {
    return err
}

t := bbgo.New(bbgo.Config{
    DB: db,
})
t.AddNotifier(slacknotifier.New(slackToken))
t.AddLogHook(slacklog.NewLogHook(slackToken))

t.AddExchange("binance", binance.New(viper.Getenv("binance-api-key"), viper.Getenv("binance-api-secret")))).
    Subscribe("binance", "btcusdt", "kline@5m", "book", "trade").
    AddStrategy(bondtrade.New, bondtrade.New).
    Symbols("btcusdt", "bnbusdt")

t.AddExchange("max", max.New(viper.Getenv("max-key"), viper.Getenv("max-secret")))).
    Subscribe("max", "btctwd", "kline@5m", "book", "trade").
    AddStrategy(flashdrop.New, bondtrade.New)

t.AddCrossExchangeStrategy(hedgemaker.New(...))
```

## Support

### Contribute pull requests

Any pull request is welcome, documentation, format fixing, testing, features.

### Register account with referral ID

You may register your exchange account with my referral ID to support this project.

- For MAX Exchange: <https://max.maicoin.com/signup?r=c7982718> (default commission rate to your account)
- For Binance Exchange: <https://www.binancezh.com/en/register?ref=VGDGLT80> (5% commission back to your account)

### Some cryptos

To support this project by some small amount cryptos:

- BTC address `3J6XQJNWT56amqz9Hz2BEVQ7W4aNmb5kiU`
- USDT ERC20 address `0x63E5805e027548A384c57E20141f6778591Bac6F`

## License

MIT License