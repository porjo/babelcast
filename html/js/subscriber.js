
var allCandidatesGathered = false;
var getChannelsId = null;

document.getElementById('reload').addEventListener('click', function() {
	window.location.reload(false);
});

function channelClick(e) {
	document.getElementById('output').classList.remove('hidden');
	document.getElementById('channels').classList.add('hidden');
	let params = {};
	params.Channel = e.target.innerText;
	if( allCandidatesGathered ) {
		params.SessionDescription = pc.localDescription.sdp
	}
	let val = {Key: 'connect_subscriber', Value: params};
	wsSend(val);
};

function updateChannels(channels) {
	let channelsEle = document.querySelector('#channels ul');
	channelsEle.innerHTML = '';
	if(channels.length > 0) {
		clearInterval(getChannelsId);
		document.getElementById('nochannels').classList.add('hidden');
		channels.forEach((e) => {
			let c = document.createElement("li");
			c.classList.add('channel');
			c.innerText = e;
			c.addEventListener("click", channelClick);
			channelsEle.appendChild(c);
		});
	}
	document.getElementById('channels').classList.remove('hidden');
	document.getElementById('reload').classList.remove('hidden');
};

ws.onmessage = function (e)	{
	let wsMsg = JSON.parse(e.data);
	if( 'Key' in wsMsg ) {
		switch (wsMsg.Key) {
			case 'info':
				debug("server info:", wsMsg.Value);
				break;
			case 'error':
				error("server error:", wsMsg.Value);
				document.getElementById('output').classList.add('hidden');
				document.getElementById('channels').classList.add('hidden');
				break;
			case 'sd_answer':
				startSession(wsMsg.Value);
				break;
			case 'channels':
				updateChannels(wsMsg.Value);
				break;
		}
	}
};

ws.onclose = function()	{
	debug("WS connection closed");
	pc.close()
	document.getElementById('media').classList.add('hidden');
};

//
// -------- WebRTC ------------
//

pc.ontrack = function (event) {
	//console.log("Ontrack", event);
	let el = document.createElement(event.track.kind);
	el.srcObject = event.streams[0];
	el.autoplay = true;
	el.controls = true;

	document.getElementById('media').appendChild(el);
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

//pc.addTransceiver('audio', {'direction': 'sendrecv'})
pc.addTransceiver('audio')
/*pc.createOffer({
	offerToReceiveVideo: false, 
	offerToReceiveAudio: true
	*/
pc.createOffer().then(d => pc.setLocalDescription(d)).catch(debug)

pc.onicecandidate = event => {
	document.getElementById('spinner').classList.remove('hidden');

	// Instead of trickle ICE (where each candidate gets send individually) we wait
	// until the end and send them all at once in the sdp
	if (event.candidate === null) {

		allCandidatesGathered = true;
		getChannelsId = setInterval(function() {
			console.log("get_channels");
			let val = {Key: 'get_channels'}
			wsSend(val);
		}, 1000);

		/*
		let params = {};
		params.SessionDescription = pc.localDescription.sdp;
		params.IsSubscriber = true;
		let val = {Key: 'session', Value: params};
		wsSend(val);
		*/
	}
}
// ----------------------------------------------------------------
