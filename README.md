# Babelcast

A server which allows audio publishers to broadcast to subscribers on a channel, using nothing more than a modern web browser.

It uses websockets for signalling & WebRTC for audio.

The designed use case is for live events where language translation is happening.
A translator would act as a publisher and people wanting to hear the translation would be subscribers.

## Building

Download [precompiled binary for Linux](https://github.com/porjo/babelcast/releases/latest) or build it yourself.

Requires Go >= 1.19

Fetch the project `go get github.com/porjo/babelcast`

## Usage

```
Usage of ./babelcast:
  -port int
    	listen on this port (default 8080)
  -webRoot string
    	web root directory (default "html")
```

Then point your web browser to `http://<server-ip>:8080/`

If the `PUBLISHER_PASSWORD` environment variable is set, then publishers will be required to enter the
password before they can connect.

## Credit

Thanks to the excellent [Pion](https://github.com/pion/webrtc) library for making WebRTC so accessible.
