var getChannelsId = setInterval(function () {
  console.log("get_channels");
  let val = { Key: "get_channels" };
  wsSend(val);
}, 1000);

// Module-level: survive across initializeSubscriber() reinvocations on WS reconnect
var _visibilityHandler = null;
var _stopConnectionLoss = null;
var _loadingTimer = null;
var _reloadPillTimer = null;

function initializeSubscriber() {
  // --- Wake Lock (prevents Android from stopping audio via energy saving) ---
  var wakeLock = null;

  async function requestWakeLock() {
    if ("wakeLock" in navigator) {
      try {
        wakeLock = await navigator.wakeLock.request("screen");
        // Track system-initiated releases (battery saver, lock screen) so re-acquire works
        wakeLock.addEventListener("release", function () {
          wakeLock = null;
        });
      } catch (err) {
        console.log("Wake lock error:", err.message);
      }
    }
  }

  function releaseWakeLock() {
    if (wakeLock) {
      wakeLock.release().then(() => { wakeLock = null; });
    }
  }

  // Remove previous handler before re-registering (prevents listener accumulation on WS reconnect)
  if (_visibilityHandler) {
    document.removeEventListener("visibilitychange", _visibilityHandler);
  }
  _visibilityHandler = function () {
    if (document.visibilityState === "visible") {
      // Re-acquire lock (system releases it on screen-off / battery saver)
      if (wakeLock === null) {
        const audio = document.getElementById("audio");
        if (audio && !audio.paused) requestWakeLock();
      }
      // Kick the audio pipeline after the tab returns to foreground
      const audio = document.getElementById("audio");
      if (audio && !audio.paused) audio.play().catch(() => {});
    }
  };
  document.addEventListener("visibilitychange", _visibilityHandler);

  // --- Loading timeout (auto-reload if spinner stays visible too long) ---
  // NOTE: _loadingTimer / _reloadPillTimer are module-level so they survive
  // ws.onclose → initializeSubscriber() reinvocations and can always be canceled.

  function showReloadPill() {
    document.getElementById("reload-pill").classList.remove("hidden");
  }

  function hideReloadPill() {
    document.getElementById("reload-pill").classList.add("hidden");
  }

  function startLoadingTimeout() {
    clearLoadingTimeout();
    _reloadPillTimer = setTimeout(showReloadPill, 3000);
    _loadingTimer = setTimeout(function () {
      console.log("Loading timeout reached, reloading...");
      window.location.reload();
    }, 4000);
  }

  function clearLoadingTimeout() {
    if (_loadingTimer) {
      clearTimeout(_loadingTimer);
      _loadingTimer = null;
    }
    if (_reloadPillTimer) {
      clearTimeout(_reloadPillTimer);
      _reloadPillTimer = null;
    }
    hideReloadPill();
  }

  function updateChannels(channels) {
    let channelsEle = document.querySelector("#channels div");
    channelsEle.innerHTML = "";

    if (channels.length > 0) {
      clearInterval(getChannelsId);
      document.getElementById("nochannels").classList.add("hidden");
      // Helper function to assign a rank to each channel
      function getChannelRank(channel) {
        if (/live|original/i.test(channel)) return 1;
        if (/translation|uebersetzung/i.test(channel)) return 2;
        if (/english|englisch/i.test(channel)) return 3;
        return 4;
      }
      // Sort channels by rank
      channels.sort((a, b) => {
        const rankA = getChannelRank(a);
        const rankB = getChannelRank(b);
        return rankA - rankB;
      });

      channels.forEach((channel) => {
        let channelButton = document.createElement("a");
        channelButton.classList.add(
          "button",
          "mdl-button",
          "mdl-button--raised",
          "mdl-js-button",
          "mdl-js-ripple-effect"
        );
        channelButton.id = channel;

        // Set the innerHTML and icon based on the channel name
        if (/live|original/i.test(channel)) {
          channelButton.innerHTML =
            '<i class="material-icons">record_voice_over</i> <i id=btn_original>Original</i>';
        } else if (/translation|uebersetzung/i.test(channel)) {
          channelButton.innerHTML =
            '<i class="material-icons">translate</i> <i id=btn_translation>Translation</i>';
        } else {
          channelButton.innerHTML =
            '<i class="material-icons">language</i> ' + channel;
        }

        // Add event listener
        channelButton.addEventListener("click", function () {
          channelClick(this);
        });

        // Append the button to the channel list
        channelsEle.appendChild(channelButton);
      });
    }

    translate_text();
  }

  function channelClick(buttonElement) {
    if (buttonElement.classList.contains("playing")) {
      const audio = document.getElementById("audio");
      if (audio) {
        if (audio.paused) {
          audio.play();
        } else {
          audio.pause();
        }
      }
    } else {
      document.getElementById("play").classList.add("hidden");
      document.getElementById("spinner").classList.remove("hidden");
      startLoadingTimeout();

      // Remove 'playing' class from all channels
      document.querySelectorAll(".playing").forEach((el) => {
        el.classList.remove("playing");
      });

      // Add 'playing' class to this channel
      buttonElement.classList.add("playing");

      // Connect to the selected channel
      let params = {};
      params.Channel = buttonElement.id;

      // Store the channel selection
      localStorage.setItem("lab_channel", params.Channel);

      // Clean up existing connections properly, this will start a new connection
      triggerNewConnection();
    }
  }

  function triggerNewConnection() {
    // Close any existing audio tracks
    const audio = document.getElementById("audio");
    if (audio) {
      audio.pause();
      audio.srcObject = null;
    }
    closeWS();
  }

  function closeWS() {
    if (ws) {
      ws.close();
    }
  }

  function hardReload() {
    // Close the existing connections
    closeWS();
    window.location.reload(true);
  }

  ws.onmessage = function (e) {
    let wsMsg = JSON.parse(e.data);
    if ("Key" in wsMsg) {
      switch (wsMsg.Key) {
        case "info":
          debug("server info: " + wsMsg.Value);
          break;
        case "error":
          error("server error:", wsMsg.Value);
          document.getElementById("channels").classList.add("hidden");
          localStorage.removeItem("lab_channel");
          hardReload();
          break;
        case "sd_answer":
          startSession(wsMsg.Value);
          break;
        case "channels":
          updateChannels(wsMsg.Value);
          break;
        case "session_received": // Wait for confirmation from server
          document.getElementById("channels").classList.remove("hidden");
          document
            .getElementById("block_buttons_layer")
            .classList.add("hidden");
          document.getElementById("spinner").classList.add("hidden");
          clearLoadingTimeout();
          // Auto-connect to the previously selected channel if available
          if (localStorage.getItem("lab_channel")) {
            var params = {
              Channel: localStorage.getItem("lab_channel"),
            };
            // Wait for the channel element to be available before adding the class and connecting
            var waitForElement = setInterval(() => {
              const channelElement = document.getElementById(params.Channel);
              if (channelElement) {
                channelElement.classList.add("playing");
                document.getElementById("spinner").classList.remove("hidden");
                startLoadingTimeout();
                wsSend({
                  Key: "connect_subscriber",
                  Value: params,
                });
                clearInterval(waitForElement);
              }
            }, 100);
            // Stop waiting after 5s if the channel button never appears
            setTimeout(() => clearInterval(waitForElement), 5000);
          }
          break;
        case "ice_candidate":
          pc.addIceCandidate(wsMsg.Value);
          break;
        case "channel_closed":
          error("channel '" + wsMsg.Value + "' closed by server");
          break;
      }
    }
  };

  ws.onclose = function () {
    console.log("websocket connection closed, restarting...");
    if (_stopConnectionLoss) { _stopConnectionLoss(); _stopConnectionLoss = null; }
    pc.close();
    document.getElementById("media").classList.add("hidden");
    clearInterval(getChannelsId);
    initializeCommon();
    initializeSubscriber();
  };

  //
  // -------- WebRTC ------------
  //

  pc.ontrack = function (event) {
    debug("webrtc: ontrack");

    let audio;
    const mediaContainer = document.getElementById("media");

    // Clear the media container if needed
    if (mediaContainer.innerHTML.trim() !== "") {
      mediaContainer.innerHTML = "";
    }

    // Create a new audio element
    audio = document.createElement("audio");
    audio.id = "audio";
    audio.srcObject = event.streams[0];
    audio.autoplay = true;
    audio.playsInline = true;

    // Add the audio element to the page
    mediaContainer.appendChild(audio);

    // Setup audio event handlers
    setupAudioHandlers(audio);

    // Show the media container and play button
    mediaContainer.classList.remove("hidden");
    document.getElementById("play").classList.remove("hidden");
    clearLoadingTimeout();
  };

  function setupAudioHandlers(audio) {
    const playButton = document.getElementById("play");

    // Use onclick to replace any previous handler (prevents stacking on each ontrack event)
    playButton.onclick = function () {
      if (audio) {
        if (audio.paused) {
          audio.play().catch(() => {});
        } else {
          audio.pause();
        }
      }
    };

    // currentTime-based stall detection: catches Android freezes that don't fire onwaiting
    var lastCurrentTime = null;
    var stallCheckInterval = setInterval(function () {
      if (!audio.paused && !audio.ended) {
        if (lastCurrentTime !== null && audio.currentTime === lastCurrentTime) {
          console.log("Audio stalled (currentTime frozen), reloading...");
          clearInterval(stallCheckInterval);
          window.location.reload();
          return;
        }
        lastCurrentTime = audio.currentTime;
      } else {
        lastCurrentTime = null;
      }
    }, 3000);
    audio.addEventListener("emptied", function () { clearInterval(stallCheckInterval); });

    let pauseTimer = null;

    // Clear pause timer helper function
    function clearPauseTimer() {
      if (pauseTimer) {
        clearTimeout(pauseTimer);
        pauseTimer = null;
      }
    }

    // Function to handle long pause
    function handleLongPause() {
      // playButton.innerHTML = '<span class="material-icons">refresh</span>';
      playButton.onclick = function () {
        window.location.reload();
      };
    }

    // Audio state handlers
    audio.onended = function () {
      console.log("stream ended");
      releaseWakeLock();
      playButton.onclick = function () {
        window.location.reload();
      };
    };

    // Stop any previous loop before starting a new one (prevents accumulation on reconnect)
    if (_stopConnectionLoss) { _stopConnectionLoss(); }
    _stopConnectionLoss = detectConnectionLoss(() => {
      console.log("Connection lost!");
      playButton.innerHTML = '<span class="material-icons">refresh</span>';
      playButton.onclick = function () {
        window.location.reload();
      };
    });

    audio.onwaiting = function () {
      console.log("waiting for audio data");
      if (!audio.paused) {
        document.getElementById("spinner").classList.remove("hidden");
        startLoadingTimeout();
      }
    };

    audio.oncanplaythrough = function () {
      document.getElementById("spinner").classList.add("hidden");
      clearLoadingTimeout();
    };

    audio.onerror = function () {
      console.log("error loading audio data");
      playButton.innerHTML = "error";
    };

    // Play/Pause button handlers
    audio.onplay = function () {
      playButton.innerHTML = '<i class="material-icons">pause</i>';
      clearPauseTimer(); // Clear timer when playing
      clearLoadingTimeout();
      requestWakeLock();
    };

    audio.onpause = function () {
      playButton.innerHTML = '<i class="material-icons">play_arrow</i>';
      document.getElementById("spinner").classList.add("hidden");
      // Start timer when paused
      clearPauseTimer(); // Clear any existing timer
      pauseTimer = setTimeout(handleLongPause, 60000); // 1 minute
      releaseWakeLock();
    };

    // Set initial button state
    playButton.innerHTML = '<i class="material-icons">play_arrow</i>';
  }

  pc.addTransceiver("audio");

  let f = () => {
    debug("webrtc: create offer");
    pc.createOffer()
      .then((d) => {
        debug("webrtc: set local description");
        pc.setLocalDescription(d);
        let val = { Key: "session_subscriber", Value: d };
        wsSend(val);
      })
      .catch(debug);
  };

  // Start loading timeout — covers both initial page load and WS reconnect.
  // The spinner is visible at this point; if we never reach pc.ontrack the pill/reload fires.
  startLoadingTimeout();

  // Create offer if WS is ready, otherwise queue
  ws.readyState == WebSocket.OPEN ? f() : onWSReady.push(f);
}

// Call the initialization function
initializeSubscriber();
