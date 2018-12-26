
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

var loc = window.location, ws_uri;
if (loc.protocol === "https:") {
	ws_uri = "wss:";
} else {
	ws_uri = "ws:";
}
ws_uri += "//" + loc.host;
var path = loc.pathname.replace(/\/$/, '');
ws_uri += "/ws";

var ws = new WebSocket(ws_uri);

var error, msg;

var debug = log = (...m) => {
	console.log(...m)
}

error = (...m) => {
	console.log(...m)
	var errorEle = document.getElementById('error');
	errorEle.innerText(m.join(", "));
	errorEle.classList.remove('hidden');
}
msg = m => {
	var d = new Date(Date.now()).toLocaleString();
	// strip html
	var a = document.createElement("div").innerText(m.Message).innerText();
	var msgEle = document.getElementById('messages');
	msgEle.classList.add('message');
	var insEle = document.createElement("div");
	insEle.innerHTML("<span class='time'>" + d + "</span><span class='sender'>" + m.Sender + "</span><span class='message'>" + a + "</span>");
	msgEle.insertBefore(insEle, msgEle.firstChild);
}

var wsSend = m => {
	if (ws.readyState === 1) {
		ws.send(JSON.stringify(m));
	} else {
		debug("WS send not ready, delaying...")
		setTimeout(function() {
			ws.send(JSON.stringify(m));
		}, 2000);
	}
}

ws.onopen = function() {
	debug("WS connection open")
};

//
// -------- WebRTC ------------
//
var pc = new RTCPeerConnection({
	iceServers: [
		{
			urls: 'stun:stun.l.google.com:19302'
		}
	]
})

pc.oniceconnectionstatechange = e => {
	debug("ICE state:", pc.iceConnectionState)
	switch (pc.iceConnectionState) {
		case "new":
		case "checking":
		case "failed":
		case "disconnected":
		case "closed":
		case "completed":
			break
		case "connected":
			document.getElementById('spinner').classList.add('hidden');
			var cbEle = document.getElementById('connect-button')
			if(cbEle) { cbEle.classList.remove('hidden') };
			break
		default:
			debug("ice state unknown", e)
			break
	}
}

pc.onicecandidate = event => {
	document.getElementById('spinner').classList.remove('hidden');
	if (event.candidate === null) {
		var params = {};
		params.SessionDescription = pc.localDescription.sdp
		var val = {Key: 'session', Value: params};
		wsSend(val)
	}
}

startSession = sd => {
	try {
		pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
	} catch (e) {
		alert(e)
	}
}
