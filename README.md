# Babelcast

A server which allows audio publishers to broadcast to subscribers on a channel, using nothing more than a modern web browser.

It uses websockets for signalling & WebRTC for audio.

The designed use case is for live events where language translation is happening.
A translator would act as a publisher and people wanting to hear the translation would be subscribers.

## Building

Download a [precompiled binary](https://github.com/porjo/babelcast/releases/latest) or build it yourself.

## Usage

```
Usage of ./babelcast:
  -debug
        enable debug log
  -port int
        listen on this port (default 8080)
```

Then point your web browser to `http://localhost:8080/`

If the `PUBLISHER_PASSWORD` environment variable is set, then publishers will be required to enter the
password before they can connect.

### TLS

Except when testing against localhost, web browsers require that TLS (`https://`) be in use any time media devices (e.g. microphone) are in use. You should put Babelcast behind a reverse proxy that can provide SSL certificates e.g. [Caddy](https://github.com/caddyserver/caddy).

See this [Stackoverflow post](https://stackoverflow.com/a/34198101/202311) for more information.

### Using Publisher on a different computer in the same network

If you want to stream audio through a different device than the server, you may need to visit `chrome://flags/#unsafely-treat-insecure-origin-as-secure` and add that device's origin (e.g. `http://192.168.1.50:8080`) to the list so that Chrome allows you to use the microphone.

## Credit

Thanks to the excellent [Pion](https://github.com/pion/webrtc) library for making WebRTC so accessible.
