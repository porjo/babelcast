
$(function(){

	$("#reload").click(() => window.location.reload(false) );

	$("#channels").on('click', '.channel', function() {
		$("#output").show();
		$("#channels").hide();
		var params = {};
		params.Channel = $(this).text();
		var val = {Key: 'connect_subscriber', Value: params};
		wsSend(val);
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
				case 'channels':
					$("#channels ul").html();
					$("#nochannels").hide();
					var channels = wsMsg.Value
					if(channels.length == 0) {
						$("#nochannels").show();
					} else {
						channels.forEach((e) => {
							var $c = $("<li/>").addClass('channel').text(e)
							$("#channels ul").append($c);
						});
					}
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

});

//
// -------- WebRTC ------------
//

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
