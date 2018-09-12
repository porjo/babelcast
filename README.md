# Babelcast

A server which allows audio publishers to broadcast to subscribers on a channel.

It uses websockets for signalling & WebRTC for audio.

The designed use case is for live events where language translation is happening.
A translator would act as a publisher and people wanting to hear the translation would be subscribers.

## Usage

```
$ go get github.com/porjo/babelcast
$ babelcast \
	-webRootPublisher $GOPATH/src/github.com/porjo/babelcast/htmlPublisher \
	-webRootSubscriber $GOPATH/src/github.com/porjo/babelcast/htmlSubscriber \
	-port 8080
```

- Publishers should point their web browser to http://localhost:8080/static/publisher/
- Subscribers should point their web browser to http://localhost:8080/static/subscriber/

If the `PUBLISHER_PASSWORD` environment variable is set, then publishers will be required to enter the
password before they can connect.
