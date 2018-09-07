
var loc = window.location, ws_uri;
if (loc.protocol === "https:") {
	ws_uri = "wss:";
} else {
	ws_uri = "ws:";
}
ws_uri += "//" + loc.host;
var path = loc.pathname.replace(/\/$/, '');
ws_uri += path + "/websocket";

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

	$("#advanced-toggle").click(function() {
		$(this).find(".text").toggleClass('hidden');
		$("#advanced-form").slideToggle();
	});

	$("#connect-button").click(function() {
		if (ws.readyState === 1) {
			log("js: Connecting to host");
			$("#output").show();
			var params = {};
			params.Url = $("#url").val();
			params.Hostname = $("#hostname").val();
			params.Port = Number($("#port").val());
			params.Username = $("#username").val();
			params.Channel = $("#channel").val();
			params.SessionDescription = pc.localDescription.sdp;
			var val = {Key: 'connect', Value: params};
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
					connectRTC(wsMsg.Value);
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

	pc = new RTCPeerConnection({
		iceServers: [
			{
				urls: "stun:stun.l.google.com:19302"
			}
		]
	})

	pc.ontrack = function (event) {
		var el = document.createElement(event.track.kind)
		el.srcObject = event.streams[0]
		el.autoplay = true
		el.controls = true

		$("#media").append(el);
	}

	pc.oniceconnectionstatechange = e => log("js: rtc state change, " + pc.iceConnectionState)
	pc.onicecandidate = event => {
		if (event.candidate === null) {
			//document.getElementById('localSessionDescription').value = btoa(pc.localDescription.sdp)
		}
	}

	pc.createOffer({
	//	offerToReceiveVideo: true, 
		offerToReceiveAudio: true
	}).then(d => pc.setLocalDescription(d)).catch(log)

	function connectRTC(sd) {
			if (sd === '') {
				return alert('Session Description must not be empty')
			}

			try {
				pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: sd}))
			} catch (e) {
				alert(e)
			}
	}

});
