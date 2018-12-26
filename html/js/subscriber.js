
document.getElementById('reload').addEventListener('click', function() {
	window.location.reload(false);
});

function channelClick(e) {
	document.getElementById('output').classList.remove('hidden');
	document.getElementById('channels').classList.add('hidden');
	var params = {};
	params.Channel = e.target.innerText;
	var val = {Key: 'connect_subscriber', Value: params};
	wsSend(val);
};

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
				document.getElementById('channels').classList.add('hidden');
				break;
			case 'sd_answer':
				startSession(wsMsg.Value);
				break;
			case 'channels':
				var channelsEle = document.querySelector('#channels ul');
				channelsEle.innerHTML = '';
				var channels = wsMsg.Value
				if(channels.length > 0) {
					document.getElementById('nochannels').classList.add('hidden');
					channels.forEach((e) => {
						var c = document.createElement("li");
						c.classList.add('channel');
						c.innerText = e;
						c.addEventListener("click", channelClick);
						channelsEle.appendChild(c);
					});
				}
				document.getElementById('channels').classList.remove('hidden');
				document.getElementById('reload').classList.remove('hidden');
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
	var el = document.createElement(event.track.kind)
	el.srcObject = event.streams[0]
	el.autoplay = true
	el.controls = true

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

pc.createOffer({
	offerToReceiveVideo: false, 
	offerToReceiveAudio: true
}).then(d => pc.setLocalDescription(d)).catch(debug)
// ----------------------------------------------------------------
