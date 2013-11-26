# syslogbot

syslogbot relays syslogd messages from UDP port 514 to IRC

## Features

* content routing by regex
* source routing by IP, with catch-all
* dynamic regex JIT filtering
* JSON configuration
* written in Go

## Installation

`go get -u github.com/presbrey/syslogbot`

Please copy the example [JSON config](example.json) and modify to suit your system.

Use [supervisord](http://supervisord.org) to keep syslogbot running with `autorestart=true`.

## Dynamic Filtering

Dynamic filter configuration is accomplished by setting the channel topic.

Destination channel topics are compiled to regex and filter all destined messages.

## License

[MIT](http://joe.mit-license.org)
