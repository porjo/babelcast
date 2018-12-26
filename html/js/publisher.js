
var localStream;
var audioTrack;

document.getElementById('reload').addEventListener('click', function() {
	window.location.reload(false);
});

document.getElementById('microphone').addEventListener('click', function() {
	toggleMic()
});

var toggleMic = function() {
	var micEle = document.getElementById('microphone');
	micEle.classList.toggle('icon-mute');
	micEle.classList.toggle('icon-mic');
	micEle.classList.toggle('on');
	audioTrack.enabled = micEle.classList.contains('icon-mic');
}

document.getElementById('input-form').addEventListener('submit', function(e) {
	e.preventDefault();

	document.getElementById('output').classList.remove('hidden');
	document.getElementById('input-form').classList.add('hidden');
	var params = {};

	params.Channel = document.getElementById('channel').value;
	params.Password = document.getElementById('password').value;
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
				document.getElementById('output').classList.add('hidden');
				document.getElementById('input-form').classList.add('hidden');
				break;
			case 'sd_answer':
				startSession(wsMsg.Value);
				break;
		}
	}
};

ws.onclose = function()	{
	debug("WS connection closed");
	if (audioTrack) {
		audioTrack.stop()
	}
	pc.close()
};

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
	.catch(debug)

pc.onnegotiationneeded = e =>
	pc.createOffer().then(d => pc.setLocalDescription(d)).catch(debug)

