"use strict";

function initializeCommon() {
  // Check for WebRTC support
  var isWebRTCSupported =
    navigator.getUserMedia ||
    navigator.webkitGetUserMedia ||
    navigator.mozGetUserMedia ||
    navigator.msGetUserMedia ||
    window.RTCPeerConnection;

  if (!isWebRTCSupported) {
    document.getElementById("not-supported").classList.remove("hidden");
    document.getElementById("supported").classList.add("hidden");
    throw new Error("WebRTC not supported");
  }

  var loc = window.location,
    ws_uri;
  if (loc.protocol === "https:") {
    ws_uri = "wss:";
  } else {
    ws_uri = "ws:";
  }
  ws_uri += "//" + loc.host;
  var path = loc.pathname.substring(0, loc.pathname.lastIndexOf("/"));
  ws_uri += path + "/ws";

  var ws = new WebSocket(ws_uri);

  // array of funcs to call when WS is ready
  var onWSReady = [];

  var error, msg;

  var debug = (...m) => {
    console.log(...m);
    msg(m.join(" "));
  };

  error = (...msgs) => {
    console.log(...msgs);
    var errorEle = document.getElementById("errors");
    msgs.forEach((m) => {
      let c = document.createElement("div");
      c.classList.add("error");
      c.innerText = m;
      errorEle.appendChild(c);
    });
    errorEle.classList.remove("hidden");
  };
  msg = (m) => {
    let d = new Date(Date.now()).toISOString();
    let msgEle = document.getElementById("message-log");
    msgEle.prepend(d + " " + m + "\n");
  };

  var wsSend = (m) => {
    let j = JSON.stringify(m);
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(j);
    } else {
      debug("ws: send not ready, skipping...", j);
    }
  };

  ws.onopen = function () {
    debug("ws: connection open");
    onWSReady.forEach((f) => {
      f();
    });
  };

  //
  // -------- WebRTC ------------
  //

  var pc = new RTCPeerConnection({
    iceServers: [
      {
        urls: "stun:stun.l.google.com:19302",
      },
    ],
  });

  var iceDisconnectedTimer = null;

  pc.oniceconnectionstatechange = (e) => {
    debug("ICE state:", pc.iceConnectionState);
    switch (pc.iceConnectionState) {
      case "connected":
      case "completed":
        document.getElementById("spinner").classList.add("hidden");
        if (iceDisconnectedTimer) {
          clearTimeout(iceDisconnectedTimer);
          iceDisconnectedTimer = null;
        }
        var cb = document.getElementById("connect-button");
        if (cb) {
          cb.classList.remove("hidden");
        }
        break;
      case "disconnected":
        // Transient — mobile networks often self-recover. Reload after 5s if still disconnected.
        iceDisconnectedTimer = setTimeout(function () {
          if (pc.iceConnectionState === "disconnected" || pc.iceConnectionState === "failed") {
            console.log("ICE disconnected timeout, reloading...");
            if (window.saveSettingsForReload) window.saveSettingsForReload();
            window.location.reload();
          }
        }, 5000);
        break;
      case "failed":
        if (iceDisconnectedTimer) {
          clearTimeout(iceDisconnectedTimer);
          iceDisconnectedTimer = null;
        }
        console.log("ICE failed, reloading...");
        if (window.saveSettingsForReload) window.saveSettingsForReload();
        window.location.reload();
        break;
      case "new":
      case "checking":
      case "closed":
        break;
      default:
        debug("webrtc: ice state unknown", e);
        break;
    }
  };

  var startSession = (sd) => {
    document.getElementById("spinner").classList.remove("hidden");
    try {
      debug("webrtc: set remote description");
      pc.setRemoteDescription(
        new RTCSessionDescription({ type: "answer", sdp: sd })
      );
    } catch (e) {
      alert(e);
    }
  };

  pc.onicecandidate = (e) => {
    if (e.candidate && e.candidate.candidate !== "") {
      let val = { Key: "ice_candidate", Value: e.candidate };
      wsSend(val);
    }
  };

  // Make functions and variables available globally
  window.ws_uri = ws_uri;
  window.onWSReady = onWSReady;
  window.error = error;
  window.msg = msg;
  window.debug = debug;
  window.wsSend = wsSend;
  window.ws = ws;
  window.pc = pc;
  window.startSession = startSession;
}

// Call the initialization function
initializeCommon();
