var audioTrack;
var mediaRecorder;
var recordedChunks = [];
var silenceStart = null;
var recording = false;
var stopAfterMinutesSilence = 60;
var reconnectTimeout = null;
var connectionLostTime = null;
var channelName = "";
var channelPassword = "";
var wasRecording = false;
var micEnabled = true;
var autoStopEnabled = true;
var gainNode = null;
var amplifiedStream = null;
var volumeGain = 1.0;

// Add a debug function
var debug = function (...args) {
  console.log(...args);
};

// Function to create amplified audio stream
var createAmplifiedStream = function (originalStream) {
  if (!window.audioContext) {
    debug("AudioContext not available");
    return originalStream;
  }

  try {
    // Create audio nodes
    const source = window.audioContext.createMediaStreamSource(originalStream);
    gainNode = window.audioContext.createGain();
    const destination = window.audioContext.createMediaStreamDestination();

    // Connect the nodes
    source.connect(gainNode);
    gainNode.connect(destination);

    // Set initial gain
    gainNode.gain.value = volumeGain;

    debug("Audio amplification chain created, initial gain:", volumeGain);
    return destination.stream;
  } catch (error) {
    debug("Error creating amplified stream:", error);
    return originalStream;
  }
};

// Function to update volume gain
var updateVolumeGain = function (newGain) {
  volumeGain = Math.max(0.1, Math.min(5.0, newGain)); // Limit between 0.1x and 5x
  if (gainNode) {
    gainNode.gain.value = volumeGain;
    debug("Volume gain updated to:", volumeGain);
  }
  
  // Update UI display
  const volumeDisplay = document.getElementById("volume-display");
  if (volumeDisplay) {
    volumeDisplay.textContent = Math.round(volumeGain * 100) + "%";
  }
};

// Add event listeners for the new controls
document.addEventListener("DOMContentLoaded", function () {
  const silenceMinutesInput = document.getElementById("silence-minutes");
  const autoStopEnabledCheckbox = document.getElementById("auto-stop-enabled");
  const volumeSlider = document.getElementById("volume-slider");

  // Initialize controls with current values
  silenceMinutesInput.value = stopAfterMinutesSilence;
  autoStopEnabledCheckbox.checked = autoStopEnabled;
  if (volumeSlider) {
    volumeSlider.value = volumeGain * 100;
    updateVolumeGain(volumeGain);
  }

  // Add event listeners
  silenceMinutesInput.addEventListener("change", function () {
    const value = parseInt(this.value);
    if (value >= 1 && value <= 180) {
      stopAfterMinutesSilence = value;
      debug("Updated silence threshold to", value, "minutes");
    } else {
      this.value = stopAfterMinutesSilence;
      debug("Invalid silence threshold value");
    }
  });

  autoStopEnabledCheckbox.addEventListener("change", function () {
    autoStopEnabled = this.checked;
    silenceMinutesInput.disabled = !this.checked;
    debug("Auto-stop recording", autoStopEnabled ? "enabled" : "disabled");
  });

  // Volume slider event listener
  if (volumeSlider) {
    volumeSlider.addEventListener("input", function () {
      const newGain = parseFloat(this.value) / 100;
      updateVolumeGain(newGain);
    });
  }
});

document.getElementById("reload").addEventListener("click", function () {
  saveCurrentSettings();
  window.location.reload(false);
});

// Saves current publisher settings to localStorage so they survive a reload.
// Also registered as window.saveSettingsForReload so common.js can call it
// before an ICE-failure-triggered reload.
var saveCurrentSettings = function () {
  const currentSettings = {
    channel: channelName,
    password: channelPassword,
    micEnabled: audioTrack ? audioTrack.enabled : false,
    wasRecording: recording,
    autoStopEnabled: autoStopEnabled,
    stopAfterMinutesSilence: stopAfterMinutesSilence,
    volumeGain: volumeGain,
    microphoneId: sessionStorage.getItem("babelcast_microphone_id") || localStorage.getItem("babelcast_microphone_id"),
  };
  localStorage.setItem("babelcast_settings", JSON.stringify(currentSettings));
};
window.saveSettingsForReload = saveCurrentSettings;

document.getElementById("microphone").addEventListener("click", function () {
  toggleMic();
});

document.getElementById("record").addEventListener("click", function () {
  toggleRecording();
});

var toggleMic = function () {
  let micEle = document.getElementById("microphone");
  micEle.classList.toggle("icon-mute");
  micEle.classList.toggle("icon-mic");
  micEle.classList.toggle("on");
  
  const isEnabled = micEle.classList.contains("icon-mic");
  
  // Toggle original audio track
  if (audioTrack) {
    audioTrack.enabled = isEnabled;
  }
  
  // Toggle amplified stream tracks
  if (amplifiedStream) {
    amplifiedStream.getAudioTracks().forEach(track => {
      track.enabled = isEnabled;
    });
  }
  
  micEnabled = isEnabled;
};

var toggleRecording = function () {
  debug("toggleRecording called, current recording state:", recording);
  if (recording) {
    stopRecording();
  } else {
    startRecording();
  }
};

var startRecording = function () {
  debug("startRecording called");
  if (!mediaRecorder) {
    debug("Error: mediaRecorder not initialized");
    return;
  }

  try {
    recordedChunks = [];
    mediaRecorder.start();
    recording = true;
    document.getElementById("record").innerText = "Stop Recording";
    debug("Recording started successfully");
  } catch (error) {
    debug("Error starting recording:", error);
  }
};

var stopRecording = function () {
  debug("stopRecording called");
  if (!mediaRecorder) {
    debug("Error: mediaRecorder not initialized");
    return;
  }

  try {
    mediaRecorder.stop();
    recording = false;
    document.getElementById("record").innerText = "Record";
    debug("Recording stopped successfully");
  } catch (error) {
    debug("Error stopping recording:", error);
  }
};

var handleDataAvailable = function (event) {
  debug("handleDataAvailable called, data size:", event.data.size);
  if (event.data.size > 0) {
    recordedChunks.push(event.data);
    debug("Chunk added, total chunks:", recordedChunks.length);
  }
};

var handleStop = function () {
  debug("handleStop called");
  try {
    var blob = new Blob(recordedChunks, {
      type: "audio/webm",
    });
    debug("Blob created, size:", blob.size);
    sendBlob(blob);
    silenceStart = null;
  } catch (error) {
    debug("Error creating blob:", error);
  }
};

var sendBlob = function (blob) {
  debug("sendBlob called, blob size:", blob.size);
  let date = new Date();
  if (silenceStart) {
    // Calculate the actual end of speech time by subtracting the silence duration
    const silenceDurationMs = stopAfterMinutesSilence * 60 * 1000;
    date = new Date(Date.now() - silenceDurationMs);
    debug(
      "Adjusted timestamp for silence duration:",
      stopAfterMinutesSilence,
      "minutes"
    );
  }
  let filename = `${channelName.replace(/\s+/g, "_")}_${date.getFullYear()}${
    date.getMonth() + 1 < 10 ? "0" + (date.getMonth() + 1) : date.getMonth() + 1
  }${date.getDate() < 10 ? "0" + date.getDate() : date.getDate()}_${
    date.getHours() < 10 ? "0" + date.getHours() : date.getHours()
  }_${
    date.getMinutes() < 10 ? "0" + date.getMinutes() : date.getMinutes()
  }.webm`;
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.style.display = "none";
  document.body.appendChild(a);

  a.click();

  setTimeout(() => {
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  }, 0);
};

// Function to connect to a channel
var connectToChannel = function (channel, password) {
  if (!channel) return;

  document.getElementById("output").classList.remove("hidden");
  document.getElementById("input-form").classList.add("hidden");

  // Store these for potential reconnection
  channelName = channel;
  channelPassword = password || "";

  let params = {
    Channel: channelName,
    Password: channelPassword,
  };

  let val = { Key: "connect_publisher", Value: params };
  wsSend(val);
  document.getElementById("subtitle").innerText = params.Channel;
  console.log(`Connected to channel: ${params.Channel}`);
};

document.getElementById("input-form").addEventListener("submit", function (e) {
  e.preventDefault();

  // Get values from form
  channelName = document.getElementById("channel").value;
  channelPassword = document.getElementById("password").value;

  connectToChannel(channelName, channelPassword);
});

// Function to restore settings after page reload or reconnection
var restoreSettings = function () {
  const savedSettings = localStorage.getItem("babelcast_settings");
  if (savedSettings) {
    try {
      const settings = JSON.parse(savedSettings);

      // Pre-fill form fields if we're still on the form
      if (
        document.getElementById("input-form").classList.contains("hidden") ===
        false
      ) {
        if (settings.channel) {
          document.getElementById("channel").value = settings.channel;
        }
        if (settings.password) {
          document.getElementById("password").value = settings.password;
        }
      }

      // Remember these for when the connection is established
      channelName = settings.channel || "";
      channelPassword = settings.password || "";
      wasRecording = settings.wasRecording || false;
      micEnabled = settings.micEnabled || true;

      // Restore auto-stop settings
      if (typeof settings.autoStopEnabled !== "undefined") {
        autoStopEnabled = settings.autoStopEnabled;
        document.getElementById("auto-stop-enabled").checked = autoStopEnabled;
      }
      if (typeof settings.stopAfterMinutesSilence !== "undefined") {
        stopAfterMinutesSilence = settings.stopAfterMinutesSilence;
        document.getElementById("silence-minutes").value =
          stopAfterMinutesSilence;
      }
      
      // Restore volume settings
      if (typeof settings.volumeGain !== "undefined") {
        volumeGain = settings.volumeGain;
        const volumeSlider = document.getElementById("volume-slider");
        if (volumeSlider) {
          volumeSlider.value = volumeGain * 100;
          updateVolumeGain(volumeGain);
        }
      }

      // Restore microphone selection into sessionStorage (per-tab, survives reload)
      if (settings.microphoneId) {
        sessionStorage.setItem("babelcast_microphone_id", settings.microphoneId);
        localStorage.setItem("babelcast_microphone_id", settings.microphoneId);
      }

      // Clear the settings to prevent unexpected auto-connections on manual page refreshes
      localStorage.removeItem("babelcast_settings");

      return settings;
    } catch (error) {
      debug("Error restoring settings:", error);
    }
  }
  return null;
};

ws.onmessage = function (e) {
  let wsMsg = JSON.parse(e.data);
  if ("Key" in wsMsg) {
    switch (wsMsg.Key) {
      case "info":
        debug("server info", wsMsg.Value);
        // Reset reconnection timer if we get any info message
        clearReconnectionTimer();
        break;
      case "error":
        error("server error", wsMsg.Value);
        document.getElementById("output").classList.add("hidden");
        document.getElementById("input-form").classList.remove("hidden");
        break;
      case "sd_answer":
        startSession(wsMsg.Value);
        break;
      case "ice_candidate":
        pc.addIceCandidate(wsMsg.Value);
        break;
    }
  }
};

ws.onclose = function () {
  debug("WS connection closed");
  // Start reconnection timeout if connection closes unexpectedly
  startReconnectionTimer();
  if (audioTrack) {
    audioTrack.stop();
  }
  pc.close();
};

// Function to handle reconnection timer
function startReconnectionTimer() {
  // Don't start a new timer if one is already running
  if (reconnectTimeout) return;

  connectionLostTime = Date.now();
  wasRecording = recording;

  // If there was an active recording, stop it
  if (recording) {
    stopRecording();
  }

  // Attempt to reconnect after a delay
  debug("Starting reconnection timer");
  reconnectTimeout = setTimeout(attemptReconnect, 5000);
}

// Function to clear the reconnection timer
function clearReconnectionTimer() {
  if (reconnectTimeout) {
    clearTimeout(reconnectTimeout);
    reconnectTimeout = null;
    connectionLostTime = null;
    debug("Cleared reconnection timer");
  }
}

// Function to attempt reconnection
function attemptReconnect() {
  debug("Attempting to reconnect...");

  try {
    // Create a new WebSocket
    ws = new WebSocket(window.ws_uri);

    ws.onopen = function () {
      debug("Reconnection successful");

      // Initialize new peer connection
      pc = new RTCPeerConnection({
        iceServers: [
          {
            urls: "stun:stun.l.google.com:19302",
          },
        ],
      });

      // Set up event handlers
      var iceReconnectTimer = null;
      pc.oniceconnectionstatechange = (e) => {
        debug("ICE state:", pc.iceConnectionState);
        switch (pc.iceConnectionState) {
          case "connected":
          case "completed":
            if (iceReconnectTimer) { clearTimeout(iceReconnectTimer); iceReconnectTimer = null; }
            document.getElementById("spinner").classList.add("hidden");
            let cb = document.getElementById("connect-button");
            if (cb) {
              cb.classList.remove("hidden");
            }

            // If we have channel info, connect again
            if (channelName) {
              connectToChannel(channelName, channelPassword);
            }

            // If we were recording before, restart recording
            if (wasRecording && mediaRecorder && !recording) {
              setTimeout(startRecording, 1000);
            }

            // Restore microphone state
            if (audioTrack && micEnabled) {
              audioTrack.enabled = true;
              let micEle = document.getElementById("microphone");
              micEle.classList.remove("icon-mute");
              micEle.classList.add("icon-mic");
              micEle.classList.add("on");
            }
            break;
          case "disconnected":
            iceReconnectTimer = setTimeout(function () {
              if (pc.iceConnectionState === "disconnected" || pc.iceConnectionState === "failed") {
                saveCurrentSettings();
                window.location.reload();
              }
            }, 5000);
            break;
          case "failed":
            if (iceReconnectTimer) { clearTimeout(iceReconnectTimer); iceReconnectTimer = null; }
            saveCurrentSettings();
            window.location.reload();
            break;
          case "new":
          case "checking":
          case "closed":
            break;
          default:
            debug("ice state unknown", e);
            break;
        }
      };

      pc.onicecandidate = (e) => {
        if (e.candidate && e.candidate.candidate !== "") {
          let val = { Key: "ice_candidate", Value: e.candidate };
          wsSend(val);
        }
      };

      // Re-establish the WebRTC connection using the selected microphone
      const audioConstraints = window.getAudioConstraints
        ? window.getAudioConstraints()
        : {
            audio: {
              channels: 1,
              autoGainControl: true,
              echoCancellation: false,
              noiseSuppression: false,
            },
            video: false,
          };

      navigator.mediaDevices
        .getUserMedia(audioConstraints)
        .then((stream) => {
          audioTrack = stream.getAudioTracks()[0];
          
          // Create amplified stream for WebRTC and recording
          amplifiedStream = createAmplifiedStream(stream);
          
          // Use amplified stream for WebRTC
          amplifiedStream.getTracks().forEach((track) => pc.addTrack(track, amplifiedStream));

          // Set mic state based on previous state
          audioTrack.enabled = micEnabled;

          const soundMeter = new SoundMeter(window.audioContext);
          soundMeter.connectToSource(stream, function (e) {
            if (e) {
              alert(e);
              return;
            }

            try {
              // Use amplified stream for recording
              mediaRecorder = new MediaRecorder(amplifiedStream);
              debug(
                "MediaRecorder initialized with mimeType:",
                mediaRecorder.mimeType
              );

              mediaRecorder.ondataavailable = handleDataAvailable;
              mediaRecorder.onstop = handleStop;
              mediaRecorder.onerror = (event) =>
                debug("MediaRecorder error:", event);

              debug("MediaRecorder event handlers set up");
            } catch (error) {
              debug("Error setting up MediaRecorder:", error);
            }

            // make the meter value relative to a sliding max
            let max = 0.0;
            setInterval(() => {
              let val = soundMeter.instant.toFixed(2);
              if (val > max) {
                max = val;
              }
              if (max > 0) {
                val = val / max;
              }
              signalMeter.value = val;

              // Updated silence check logic
              if (val < 0.03 && recording && autoStopEnabled) {
                if (!silenceStart) {
                  silenceStart = Date.now();
                  debug("Silence detected, starting timer");
                } else if (
                  Date.now() - silenceStart >
                  stopAfterMinutesSilence * 60 * 1000
                ) {
                  debug(
                    "Silence threshold reached, stopping and restarting recording"
                  );
                  stopRecording();
                  startRecording();
                }
              } else {
                if (silenceStart) {
                  debug("Silence ended");
                }
                silenceStart = null;
              }
            }, 50);
          });

          pc.createOffer()
            .then((d) => {
              pc.setLocalDescription(d);
              let val = { Key: "session_publisher", Value: d };
              wsSend(val);
            })
            .catch(debug);
        })
        .catch((error) => debug("getUserMedia error:", error));

      // Update message handlers
      ws.onmessage = function (e) {
        let wsMsg = JSON.parse(e.data);
        if ("Key" in wsMsg) {
          switch (wsMsg.Key) {
            case "info":
              debug("server info", wsMsg.Value);
              // Reset reconnection timer if we get any info message
              clearReconnectionTimer();
              break;
            case "error":
              error("server error", wsMsg.Value);
              document.getElementById("output").classList.add("hidden");
              document.getElementById("input-form").classList.remove("hidden");
              break;
            case "sd_answer":
              startSession(wsMsg.Value);
              break;
            case "ice_candidate":
              pc.addIceCandidate(wsMsg.Value);
              break;
          }
        }
      };

      ws.onclose = function () {
        debug("WS connection closed");
        // Start reconnection timeout if connection closes unexpectedly
        startReconnectionTimer();
        if (audioTrack) {
          audioTrack.stop();
        }
        pc.close();
      };
    };

    ws.onerror = function (error) {
      debug("Reconnection error:", error);
      // Try again after a delay
      reconnectTimeout = setTimeout(attemptReconnect, 5000);
    };
  } catch (error) {
    debug("Error during reconnection attempt:", error);
    // Try again after a delay
    reconnectTimeout = setTimeout(attemptReconnect, 5000);
  }
}

//
// -------- WebRTC ------------
//

// Use the audio constraints from microphone.js if available, otherwise use defaults
const constraints = (window.constraints = window.getAudioConstraints
  ? window.getAudioConstraints()
  : {
      audio: {
        channels: 1,
        autoGainControl: true,
        echoCancellation: false,
        noiseSuppression: false,
      },
      video: false,
    });

try {
  window.AudioContext = window.AudioContext || window.webkitAudioContext;
  window.audioContext = new AudioContext();
} catch (e) {
  alert("Web Audio API not supported.");
}

const signalMeter = document.querySelector("#microphone-meter meter");

navigator.mediaDevices
  .getUserMedia(constraints)
  .then((stream) => {
    audioTrack = stream.getAudioTracks()[0];
    
    // Create amplified stream for WebRTC and recording
    amplifiedStream = createAmplifiedStream(stream);
    
    // Use amplified stream for WebRTC
    amplifiedStream.getTracks().forEach((track) => pc.addTrack(track, amplifiedStream));

    // Restore settings if available
    const settings = restoreSettings();

    // Set initial mic state
    if (settings && typeof settings.micEnabled !== "undefined") {
      audioTrack.enabled = settings.micEnabled;
      if (settings.micEnabled) {
        let micEle = document.getElementById("microphone");
        micEle.classList.remove("icon-mute");
        micEle.classList.add("icon-mic");
        micEle.classList.add("on");
      }
    } else {
      // Default is muted
      audioTrack.enabled = true;
    }

    micEnabled = audioTrack.enabled;

    const soundMeter = new SoundMeter(window.audioContext);
    soundMeter.connectToSource(stream, function (e) {
      if (e) {
        alert(e);
        return;
      }

      try {
        // Use amplified stream for recording
        mediaRecorder = new MediaRecorder(amplifiedStream);
        debug(
          "MediaRecorder initialized with mimeType:",
          mediaRecorder.mimeType
        );

        mediaRecorder.ondataavailable = handleDataAvailable;
        mediaRecorder.onstop = handleStop;
        mediaRecorder.onerror = (event) => debug("MediaRecorder error:", event);

        debug("MediaRecorder event handlers set up");

        // If we have restored settings and we should be recording, start recording
        if (settings && settings.wasRecording) {
          // Delay starting recording to ensure everything is ready
          setTimeout(startRecording, 1000);
        }
      } catch (error) {
        debug("Error setting up MediaRecorder:", error);
      }

      // make the meter value relative to a sliding max
      let max = 0.0;
      setInterval(() => {
        let val = soundMeter.instant.toFixed(2);
        if (val > max) {
          max = val;
        }
        if (max > 0) {
          val = val / max;
        }
        signalMeter.value = val;

        // Updated silence check logic
        if (val < 0.03 && recording && autoStopEnabled) {
          if (!silenceStart) {
            silenceStart = Date.now();
            debug("Silence detected, starting timer");
          } else if (
            Date.now() - silenceStart >
            stopAfterMinutesSilence * 60 * 1000
          ) {
            debug(
              "Silence threshold reached, stopping and restarting recording"
            );
            stopRecording();
            startRecording();
          }
        } else {
          if (silenceStart) {
            debug("Silence ended");
          }
          silenceStart = null;
        }
      }, 50);
    });

    pc.createOffer()
      .then((d) => {
        pc.setLocalDescription(d);
        let val = { Key: "session_publisher", Value: d };
        wsSend(val);
      })
      .catch(debug);

    // Auto-connect to channel if we have settings
    if (settings && settings.channel) {
      // First wait for connection to be established
      const checkConnectionState = setInterval(() => {
        if (
          pc.iceConnectionState === "connected" ||
          pc.iceConnectionState === "completed"
        ) {
          clearInterval(checkConnectionState);

          // Auto-fill form and submit if needed
          if (
            document
              .getElementById("input-form")
              .classList.contains("hidden") === false
          ) {
            document.getElementById("channel").value = settings.channel;
            if (settings.password) {
              document.getElementById("password").value = settings.password;
            }

            // Connect to the channel
            connectToChannel(settings.channel, settings.password);
          }
        }
      }, 500);

      // Set a timeout to clear the interval after 10 seconds if connection isn't established
      setTimeout(() => {
        clearInterval(checkConnectionState);
      }, 10000);
    }
  })
  .catch((error) => debug("getUserMedia error:", error));

pc.onicecandidate = (e) => {
  if (e.candidate && e.candidate.candidate !== "") {
    let val = { Key: "ice_candidate", Value: e.candidate };
    wsSend(val);
  }
};
// ICE connection state changes are handled by the handler set in common.js,
// which triggers saveSettingsForReload() + window.location.reload() on failure.
