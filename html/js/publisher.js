
bc = new BabelCast();

var audioTrack;

document.getElementById('reload').addEventListener('click', function() {
	window.location.reload(false);
});

document.getElementById('microphone').addEventListener('click', function() {
	toggleMic()
});

var toggleMic = function() {
	let micEle = document.getElementById('microphone');
	micEle.classList.toggle('icon-mute');
	micEle.classList.toggle('icon-mic');
	micEle.classList.toggle('on');
	audioTrack.enabled = micEle.classList.contains('icon-mic');
}

document.getElementById('input-form').addEventListener('submit', function(e) {
	e.preventDefault();

	document.getElementById('output').classList.remove('hidden');
	document.getElementById('input-form').classList.add('hidden');
	let params = {};

	params.Channel = document.getElementById('channel').value;
	params.Password = document.getElementById('password').value;
	let val = {Key: 'connect_publisher', Value: params};
	bc.wsSend(val);
});

bc.ws.onmessage = (e) => {
	let wsMsg = JSON.parse(e.data);
	if( 'Key' in wsMsg ) {
		switch (wsMsg.Key) {
			case 'info':
				bc.debug("server info", wsMsg.Value);
				break;
			case 'error':
				bc.error("server error", wsMsg.Value);
				document.getElementById('output').classList.add('hidden');
				document.getElementById('input-form').classList.add('hidden');
				break;
			case 'sd_answer':
				bc.startSession(wsMsg.Value);
				break;
			case 'ice_candidate':
				bc.pc.addIceCandidate(wsMsg.Value)
				break;
		}
	}
};

bc.ws.onclose = () => {
	bc.debug("WS connection closed");
	if (audioTrack) {
		audioTrack.stop()
	}
	bc.pc.close()
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

navigator.mediaDevices.getUserMedia(constraints).then(stream => {
	audioTrack = stream.getAudioTracks()[0];
	stream.getTracks().forEach(track => bc.pc.addTrack(track, stream))
	// mute until we're ready
	audioTrack.enabled = false;

	const soundMeter = new SoundMeter(window.audioContext);
	soundMeter.connectToSource(stream, function(e) {
		if (e) {
			alert(e);
			return;
		}

		// make the meter value relative to a sliding max
		let max = 0.0;
		setInterval(() => {
			let val = soundMeter.instant.toFixed(2);
			if( val > max ) { max = val }
			if( max > 0) { val = (val / max) }
			signalMeter.value = val;
		}, 50);
	});

	bc.pc.createOffer().then(d => {
		let val = {Key: 'session_publisher', Value: d};
		bc.wsSend(val);
		// initiate sending ICE candidates
		bc.pc.setLocalDescription(d);
	}).catch(bc.debug)
}).catch(bc.debug)


bc.pc.onicecandidate = e => {
	if (e.candidate && e.candidate.candidate !== "") {
		let val = {Key: 'ice_candidate', Value: e.candidate};
		bc.wsSend(val);
	}
}
