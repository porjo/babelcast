
var localStream;
var audioTrack;

$(function(){

	$("#reload").click(() => window.location.reload(false) );

	$("#microphone").click(function() {
		toggleMic()
	});

	var toggleMic = function() {
		$el = $("#microphone")
		$el.toggleClass("icon-mute icon-mic on")
		audioTrack.enabled = $el.hasClass("icon-mic")
	}

	$("#input-form").submit(e => {
		e.preventDefault();

		$("#output").show();
		$("#input-form").hide();
		var params = {};
		params.Channel = $("#channel").val();
		params.Password = $("#password").val();
		var val = {Key: 'connect_publisher', Value: params};
		wsSend(val)
	});

	ws.onmessage = function (e)	{
		var wsMsg = JSON.parse(e.data);
		if( 'Key' in wsMsg ) {
			switch (wsMsg.Key) {
				case 'info':
					debug("server info", wsMsg.Value);
					break;
				case 'error':
					error("server error", wsMsg.Value);
					$("#output").hide();
					$("#input-form").hide();
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

});

//
// -------- WebRTC ------------
//

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

const signalMeter = document.querySelector('#microphone-meter meter');

navigator.mediaDevices.getUserMedia(constraints)
	.then(stream => {
		localStream = stream

		audioTrack = stream.getAudioTracks()[0];
		pc.addStream(stream)
		// mute until we're ready
		audioTrack.enabled = false;

		const soundMeter = new SoundMeter(window.audioContext);
		soundMeter.connectToSource(stream, function(e) {
			if (e) {
				alert(e);
				return;
			}

			// make the meter value relative to a sliding max
			var max = 0.0
			setInterval(() => {
				var val = soundMeter.instant.toFixed(2)
				if( val > max ) { max = val }
				if( max > 0) { val = (val / max) }
				signalMeter.value = val
			}, 50);
		});
	})
	.catch(log)

pc.onnegotiationneeded = e =>
	pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

