# Babelcast

A server which allows audio publishers to broadcast to subscribers on a channel, using nothing more than a modern web browser.

It uses websockets for signalling & WebRTC for audio.

The designed use case is for live events where language translation is happening.
A translator would act as a publisher and people wanting to hear the translation would be subscribers.

## Building

This project uses ['dep'](https://github.com/golang/dep) for [vendoring](https://blog.gopheracademy.com/advent-2015/vendor-folder/).
- Install Go e.g. `yum install golang` or `apt-get install golang`
- Define your Go Path e.g. `export GOPATH=$HOME/go`
- Fetch the project `go get -d github.com/porjo/babelcast`
- run `dep ensure` in the project root
- run `go build`

You will find the compiled binary under `~/go/bin` and the html+css under `~/go/src/github.com/porjo/babelcast/*`

## Usage

```
$ babelcast \
	-webRootPublisher $GOPATH/src/github.com/porjo/babelcast/htmlPublisher \
	-webRootSubscriber $GOPATH/src/github.com/porjo/babelcast/htmlSubscriber \
	-port 8080
```

- Publishers should point their web browser to `http://<server-ip>:8080/static/publisher/`
- Subscribers should point their web browser to `http://<server-ip>:8080/static/subscriber/`

If the `PUBLISHER_PASSWORD` environment variable is set, then publishers will be required to enter the
password before they can connect.
