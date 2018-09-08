
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

var pc;
var sd_uri = loc.protocol + "//" + loc.host + path + "/sdp";

$(function(){

	var log = m => {
		console.log(m)
		// strip html
		var a = $("<div />").text(m).html();
		$("#status").prepend("<div class='message'>" + a + '</div>');
	}
	var msg = m => {
		var d = new Date(Date.now()).toLocaleString();
		// strip html
		var a = $("<div />").text(m.Message).html();
		$("#messages").prepend("<div class='message'><span class='time'>" + d + "</span><span class='sender'>" + m.Sender + "</span><span class='message'>" + a + "</span></div>");
	}

	$("#connect-button").click(function() {
		if (ws.readyState === 1) {
			$("#output").show();
			var params = {};
			params.Username = $("#username").val();
			params.Channel = $("#channel").val();
			var val = {Key: 'connect_subscriber', Value: params};
			ws.send(JSON.stringify(val));
		} else {
			log("WS socket not ready");
		}
	});

	/*
	ws.onopen = function() {
	};
	*/

	ws.onmessage = function (e)	{
		var wsMsg = JSON.parse(e.data);
		if( 'Key' in wsMsg ) {
			switch (wsMsg.Key) {
				case 'info':
					log("Info: " + wsMsg.Value);
					break;
				case 'msg':
					msg(wsMsg.Value);
					break;
				case 'error':
					log("Error: " + wsMsg.Value);
					break;
				case 'sd_answer':
					startSession(wsMsg.Value);
					break;
			}
		}
	};

	ws.onclose = function()	{
		log("WS connection closed");
		pc.close()
	};


	//
	// -------- WebRTC ------------
	//

	let pc = new RTCPeerConnection({
		iceServers: [
			{
				urls: 'stun:stun.l.google.com:19302'
			}
		]
	})

	pc.oniceconnectionstatechange = e => {
		switch (pc.iceConnectionState) {
			case "new":
			case "checking":
				log("ICE checking")
				break
			case "connected":
				log("ICE connected")
				$("#spinner").hide()
				$("#connect-button").show()
				break
			case "failed":
			case "disconnected":
			case "closed":
				log("ICE stopped")
				break
			default:
				console.log("ice state unknown", e)
				break
		}
	}

	pc.onicecandidate = event => {
		$("#spinner").show()
		if (event.candidate === null) {
			var params = {};
			params.SessionDescription = pc.localDescription.sdp
			var val = {Key: 'session', Value: params};
			ws.send(JSON.stringify(val));
		}
	}

	pc.ontrack = function (event) {
		var el = document.createElement(event.track.kind)
		el.srcObject = event.streams[0]
		el.autoplay = true
		el.controls = true

		$("#media").append(el);
	}

	pc.onnegotiationneeded = e =>
		pc.createOffer().then(d => { console.log('set local desc'); pc.setLocalDescription(d) }).catch(log)

	startSession = (sd) => {
		console.log('start session')
		try {
			pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
		} catch (e) {
			alert(e)
		}
	}

});
