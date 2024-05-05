
var getChannelsId = setInterval(function() {
	debug("get_channels");
	let val = {Key: 'get_channels'}
	wsSend(val);
}, 1000);

document.getElementById('reload').addEventListener('click', function() {
	window.location.reload(false);
});

function channelClick(e) {
	document.getElementById('output').classList.remove('hidden');
	document.getElementById('channels').classList.add('hidden');
	let params = {};
	params.Channel = e.target.innerText;
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
};

ws.onmessage = function (e)	{
	let wsMsg = JSON.parse(e.data);
	if( 'Key' in wsMsg ) {
		switch (wsMsg.Key) {
			case 'info':
				debug("server info: " + wsMsg.Value);
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
			case "session_received": // wait for the message that session_subscriber was received
				document.getElementById("channels").classList.remove("hidden");
				document.getElementById('reload').classList.remove('hidden');
				document.getElementById("spinner").classList.add("hidden");
				break;
			case 'ice_candidate':
				pc.addIceCandidate(wsMsg.Value)
				break;
			case 'channel_closed':
				error("channel '" + wsMsg.Value + "' closed by server")
				break;
		}
	}
};

ws.onclose = function()	{
	debug("ws: connection closed")
	pc.close()
	document.getElementById('media').classList.add('hidden')
	clearInterval(getChannelsId);
};

//
// -------- WebRTC ------------
//

pc.ontrack = function (event) {
	debug("webrtc: ontrack");
	let el = document.createElement(event.track.kind);
	el.srcObject = event.streams[0];
	el.autoplay = true;
	el.controls = true;

	document.getElementById('media').appendChild(el);
}

pc.addTransceiver('audio')

let f = () => {
	debug("webrtc: create offer")
	pc.createOffer().then(d => {
		debug("webrtc: set local description")
		pc.setLocalDescription(d);
		let val = { Key: 'session_subscriber', Value: d };
		wsSend(val);
	}).catch(debug)
}
// create offer if WS is ready, otherwise queue 
ws.readyState == WebSocket.OPEN ? f() : onWSReady.push(f)

// ----------------------------------------------------------------
