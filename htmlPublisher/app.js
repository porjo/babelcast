
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

var localStream;
var audioTrack;

$(function(){

	var debug = (...m) => {
		console.log(...m)
	}
	var log = (...m) => {
		console.log(...m)
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

	$("#reload").click(() => window.location.reload(false) );

	$("#input-form").submit(e => {
		e.preventDefault();
		if (ws.readyState === 1) {
			$("#output").show();
			$("#input-form").hide();
			var params = {};
			params.Channel = $("#channel").val();
			params.Password = $("#password").val();
			var val = {Key: 'connect_publisher', Value: params};
			ws.send(JSON.stringify(val));
			audioTrack.enabled = true;
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
		if (audioTrack) {
			audioTrack.stop()
		}
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

	const constraints = window.constraints = {
		audio: true,
		video: false
	};

	try {
		window.AudioContext = window.AudioContext || window.webkitAudioContext;
		window.audioContext = new AudioContext();
	} catch (e) {
		alert('Web Audio API not supported.');
	}

	const signalMeter = document.querySelector('#signal-meter meter');

	navigator.mediaDevices.getUserMedia(constraints)
		.then(stream => {
			localStream = stream

			audioTrack = stream.getAudioTracks()[0];
			pc.addStream(stream)
			// mute until we're ready
			audioTrack.enabled = false;

			const soundMeter = window.soundMeter = new SoundMeter(window.audioContext);
			soundMeter.connectToSource(stream, function(e) {
				if (e) {
					alert(e);
					return;
				}
				setInterval(() => {
					signalMeter.value = soundMeter.instant.toFixed(2);
				}, 50);
			});

		})
		.catch(log)

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
				$("#spinner").hide()
				$("#connect-button").show()
				break
			default:
				debug("ice state unknown", e)
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

	pc.onnegotiationneeded = e =>
		pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

	startSession = (sd) => {
		try {
			pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
		} catch (e) {
			alert(e)
		}
	}

});
