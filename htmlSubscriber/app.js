
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

	$("#channels").on('click', '.channel', function() {
		if (ws.readyState === 1) {
			$("#output").show();
			$("#channels").hide();
			var params = {};
			params.Channel = $(this).text();
			console.log(params);
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
				case 'channels':
					$("#channels ul").html();
					var channels = wsMsg.Value
					channels.forEach((e) => {
						var $c = $("<li/>").addClass('channel').text(e)
						$("#channels ul").append($c);
					});
					$("#channels").show();
					break;
			}
		}
	};

	ws.onclose = function()	{
		log("WS connection closed");
		pc.close()
		$("#media").hide()
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

	// FIXME:
	// the first createOffer works for publisher but not subscriber
	// the second createOffer works for subscriber but not for publisher
	// ... why??
	// ----------------------------------------------------------------
	/*
	pc.onnegotiationneeded = e =>
		pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
		*/

	pc.createOffer({
		offerToReceiveVideo: false, 
		offerToReceiveAudio: true
	}).then(d => pc.setLocalDescription(d)).catch(log)
	// ----------------------------------------------------------------

	startSession = (sd) => {
		try {
			pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
		} catch (e) {
			alert(e)
		}
	}

});
