
var loc = window.location, ws_uri;
if (loc.protocol === "https:") {
	ws_uri = "wss:";
} else {
	ws_uri = "ws:";
}
ws_uri += "//" + loc.host;
var path = loc.pathname.replace(/\/$/, '');
ws_uri += "/ws/producer";

var ws = new WebSocket(ws_uri);

var pc;
var sd_uri = loc.protocol + "//" + loc.host + path + "/sdp";

$(function(){

	var log = m => {
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
			goWebrtc()
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
	};



	//
	// -------- WebRTC ------------
	//

	goWebrtc = () => {
		let pc = new RTCPeerConnection({
			iceServers: [
				{
					urls: 'stun:stun.l.google.com:19302'
				}
			]
		})

		navigator.mediaDevices.getUserMedia({video: false, audio: true})
			.then(stream => pc.addStream(stream))
			.catch(log)

		pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
		pc.onicecandidate = event => {
			if (event.candidate === null) {
				console.log("ice candidate", pc.localDescription.sdp)
				var params = {};
				params.Username = $("#username").val();
				params.Channel = $("#channel").val();
				params.SessionDescription = pc.localDescription.sdp
				var val = {Key: 'connect', Value: params};
				ws.send(JSON.stringify(val));
			}
		}

		pc.onnegotiationneeded = e =>
			pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

		startSession = (sd) => {
			try {
				pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
			} catch (e) {
				alert(e)
			}
		}
	}


});
