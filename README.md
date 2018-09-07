# Babelcast

A server which allows audio producers to broadcast to consumers on a channel

It uses websockets for signalling & WebRTC for audio.

## Usage

```
$ go get github.com/porjo/babelcast
$ babelcast -webRoot $GOPATH/src/github.com/porjo/babelcast/html -port 8080
```

Point your web browser at http://localhost:8080/
