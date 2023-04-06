
bc = new BabelCast();

var getChannelsId = setInterval(function() {
  let val = {Key: 'get_channels'}
  bc.wsSend(val);
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
  bc.wsSend(val);
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

bc.ws.onmessage = function (e)	{
  let wsMsg = JSON.parse(e.data);
  if( 'Key' in wsMsg ) {
    switch (wsMsg.Key) {
      case 'info':
	bc.debug("server info:", wsMsg.Value);
	break;
      case 'error':
	bc.error("server error:", wsMsg.Value);
	document.getElementById('output').classList.add('hidden');
	document.getElementById('channels').classList.add('hidden');
	break;
      case 'sd_answer':
	bc.startSession(wsMsg.Value);
	break;
      case 'channels':
	updateChannels(wsMsg.Value);
	break;
      case 'ice_candidate':
	bc.pc.addIceCandidate(wsMsg.Value)
	break;
    }
  }
};

bc.ws.onclose = function()	{
  bc.debug("WS connection closed");
  bc.pc.close()
  document.getElementById('media').classList.add('hidden');
};

//
// -------- WebRTC ------------
//

bc.pc.ontrack = function (event) {
  //console.log("Ontrack", event);
  let el = document.createElement(event.track.kind);
  el.srcObject = event.streams[0];
  el.autoplay = true;
  el.controls = true;

  document.getElementById('media').appendChild(el);
}

bc.pc.addTransceiver('audio')

bc.pc.createOffer().then(d => {
  let val = {Key: 'session_subscriber', Value: d};
  bc.wsSend(val);
  // initiate sending ICE candidates
  bc.pc.setLocalDescription(d)
}).catch(bc.debug);

// ----------------------------------------------------------------
