# Babelcast

A server which allows audio publishers to broadcast to subscribers on a channel.

It uses websockets for signalling & WebRTC for audio.

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
