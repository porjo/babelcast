/*
 *  Copyright (c) 2015 The WebRTC project authors. All Rights Reserved.
 *
 *  Use of this source code is governed by a BSD-style license
 *  that can be found in the LICENSE file in the root of the source
 *  tree.
 */

class SoundMeter extends AudioWorkletProcessor {
  _instant
  _slow
  _clip
  _updateIntervalInMS
  _nextUpdateFrame

  constructor() {
    super();

    // Logs the current sample-frame and time at the moment of instantiation.
    // They are accessible from the AudioWorkletGlobalScope.
    console.log("currentFrame", currentFrame);
    console.log("currentTime", currentTime);
    this._instant=0.0;
    this._slow=0.0;
    this._clip=0.0;
    this._updateIntervalInMS = 25;
    this._nextUpdateFrame = this._updateIntervalInMS;
    this.port.onmessage = event => {
      if (event.data.updateIntervalInMS)
        this._updateIntervalInMS = event.data.updateIntervalInMS;
    }
  }


  process(inputs, outputs, parameters) {
    const input = inputs[0];
    if (input.length > 0) {
      let i;
      let sum = 0.0;
      let clipcount = 0;
      for (i = 0; i < input.length; ++i) {
	sum += input[i] * input[i];
	if (Math.abs(input[i]) > 0.99) {
	  clipcount += 1;
	}
      }
      this._instant = Math.sqrt(sum / input.length);
      this._slow = 0.95 * this._slow + 0.05 * this._instant;
      this._clip = clipcount / input.length;

      this._nextUpdateFrame -= input.length;
      if (this._nextUpdateFrame < 0) {
	this._nextUpdateFrame += this._updateIntervalInMS / 1000 * sampleRate;
	console.log("postmessage", this._instant);
	this.port.postMessage({instant: this._instant});
      }
    }
    return true;
  }
}

registerProcessor("soundmeter", SoundMeter);

/*
'use strict';

// Meter class that generates a number correlated to audio volume.
// The meter class itself displays nothing, but it makes the
// instantaneous and time-decaying volumes available for inspection.
// It also reports on the fraction of samples that were at or near
// the top of the measurement range.
function SoundMeter(context) {
	this.context = context;
	this.instant = 0.0;
	this.slow = 0.0;
	this.clip = 0.0;
	this.script = context.createScriptProcessor(2048, 1, 1);
	const that = this;
	this.script.onaudioprocess = function(event) {
		const input = event.inputBuffer.getChannelData(0);
		let i;
		let sum = 0.0;
		let clipcount = 0;
		for (i = 0; i < input.length; ++i) {
			sum += input[i] * input[i];
			if (Math.abs(input[i]) > 0.99) {
				clipcount += 1;
			}
		}
		that.instant = Math.sqrt(sum / input.length);
		that.slow = 0.95 * that.slow + 0.05 * that.instant;
		that.clip = clipcount / input.length;
	};
}

SoundMeter.prototype.connectToSource = function(stream, callback) {
	console.log('SoundMeter connecting');
	try {
		this.mic = this.context.createMediaStreamSource(stream);
		this.mic.connect(this.script);
		// necessary to make sample run, but should not be.
		this.script.connect(this.context.destination);
		if (typeof callback !== 'undefined') {
			callback(null);
		}
	} catch (e) {
		console.error(e);
		if (typeof callback !== 'undefined') {
			callback(e);
		}
	}
};

SoundMeter.prototype.stop = function() {
	this.mic.disconnect();
	this.script.disconnect();
};
*/
