'use strict';

// Check for WebRTC support
var isWebRTCSupported = navigator.getUserMedia ||
  navigator.webkitGetUserMedia ||
  navigator.mozGetUserMedia ||
  navigator.msGetUserMedia ||
  window.RTCPeerConnection;

if(!isWebRTCSupported) {
  document.getElementById("not-supported").style.display = 'block';
  document.getElementById("supported").style.display = 'none';
  throw new Error("WebRTC not supported");
}

class BabelCast {
  ws;
  pc;

  constructor() {
    console.log('init');
    var loc = window.location, ws_uri;
    if (loc.protocol === "https:") {
      ws_uri = "wss:";
    } else {
      ws_uri = "ws:";
    }
    ws_uri += "//" + loc.host;
    var path = loc.pathname.substring(0, loc.pathname.lastIndexOf("/"));
    ws_uri += path + "/ws";

    this.ws = new WebSocket(ws_uri);
    this.ws.onopen = () => {
      this.debug("WS connection open");
    };

    this.initPC();
  }

  debug(...m) {
    console.log(...m)
  }

  error(...m) {
    console.log(...m)
    let errorEle = document.getElementById('error');
    errorEle.innerText = m.join(", ");
    errorEle.classList.remove('hidden');
  }

  msg(m) {
    let d = new Date(Date.now()).toLocaleString();
    // strip html
    let div = document.createElement("div");
    div.innerHtml = m.Message;
    let a = div.innerText;
    let msgEle = document.getElementById('messages');
    msgEle.classList.add('message');
    let insEle = document.createElement("div");
    insEle.innerHTML = "<span class='time'>" + d + "</span><span class='sender'>" + m.Sender + "</span><span class='message'>" + a + "</span>";
    msgEle.insertBefore(insEle, msgEle.firstChild);
  }

  initPC() {
    //
    // -------- WebRTC ------------
    //
    this.pc = new RTCPeerConnection({
      iceServers: [
	{
	  urls: 'stun:stun.l.google.com:19302'
	}
      ]
    })

    this.pc.oniceconnectionstatechange = e => {
      this.debug("ICE state:", this.pc.iceConnectionState)
      switch (this.pc.iceConnectionState) {
	case "new":
	case "checking":
	case "failed":
	case "disconnected":
	case "closed":
	case "completed":
	  break;
	case "connected":
	  document.getElementById('spinner').classList.add('hidden');
	  let cb = document.getElementById('connect-button');
	  if(cb) { cb.classList.remove('hidden') };
	  break;
	default:
	  this.debug("ice state unknown", e);
	  break;
      }
    }


    this.pc.onicecandidate = e => {
      if (e.candidate) {
	let val = {Key: 'ice_candidate', Value: e.candidate};
	wsSend(val);
      }
    }
  }

  startSession(sd) {
    document.getElementById('spinner').classList.remove('hidden');
    try {
      this.pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}));
    } catch (e) {
      alert(e);
    }
  }

  wsSend(m) {
    if (this.ws.readyState === 1) {
      this.ws.send(JSON.stringify(m));
    } else {
      this.debug("WS send not ready, delaying...");
      setTimeout(() => {
	this.ws.send(JSON.stringify(m));
      }, 2000);
    }
  }
}
