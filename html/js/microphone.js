/**
 * microphone.js - Handles microphone detection and selection
 */

// Store available audio devices
let audioDevices = [];
let microphoneSelect = null;

// Function to get available audio input devices
async function getAvailableAudioDevices() {
  try {
    // Get all media devices
    const devices = await navigator.mediaDevices.enumerateDevices();

    // Filter out audio input devices
    audioDevices = devices.filter((device) => device.kind === "audioinput");

    // Populate the dropdown menu if it exists
    populateMicrophoneDropdown();

    return audioDevices;
  } catch (error) {
    console.error("Error fetching audio devices:", error);
    return [];
  }
}

// Function to populate the microphone dropdown
function populateMicrophoneDropdown() {
  microphoneSelect = document.getElementById("microphone-select");

  if (!microphoneSelect) return;

  // Clear existing options
  microphoneSelect.innerHTML = "";

  // Add default option
  const defaultOption = document.createElement("option");
  defaultOption.value = "default";
  defaultOption.textContent = "Default Microphone";
  microphoneSelect.appendChild(defaultOption);

  // Add detected microphones
  audioDevices.forEach((device) => {
    const option = document.createElement("option");
    option.value = device.deviceId;
    option.textContent =
      device.label || `Microphone ${microphoneSelect.options.length}`;
    microphoneSelect.appendChild(option);
  });

  // Restore selected microphone: prefer sessionStorage (per-tab) over localStorage
  const savedMicrophoneId = sessionStorage.getItem("babelcast_microphone_id") || localStorage.getItem("babelcast_microphone_id");
  if (savedMicrophoneId) {
    // Check if the saved device is in the list
    const deviceExists = Array.from(microphoneSelect.options).some(
      (option) => option.value === savedMicrophoneId
    );

    if (deviceExists) {
      microphoneSelect.value = savedMicrophoneId;
    }
  }

  // Add change event listener
  microphoneSelect.addEventListener("change", saveMicrophoneSelection);
}

// Function to save the selected microphone
function saveMicrophoneSelection() {
  const selectedMicrophoneId = microphoneSelect.value;
  sessionStorage.setItem("babelcast_microphone_id", selectedMicrophoneId);
  localStorage.setItem("babelcast_microphone_id", selectedMicrophoneId);

  // Trigger a reload to apply the new microphone
  // Using a small delay to ensure the selection is saved first
  if (window.audioTrack) {
    // Show a notification to the user
    const notification = document.createElement("div");
    notification.style.position = "fixed";
    notification.style.top = "10px";
    notification.style.left = "50%";
    notification.style.transform = "translateX(-50%)";
    notification.style.padding = "10px";
    notification.style.backgroundColor = "#4CAF50";
    notification.style.color = "white";
    notification.style.borderRadius = "5px";
    notification.style.zIndex = "1000";
    notification.textContent = "Microphone changed! Restarting audio...";
    document.body.appendChild(notification);

    setTimeout(() => {
      document.body.removeChild(notification);
      window.location.reload();
    }, 1500);
  }
}

// Function to get constraints with the selected microphone
function getAudioConstraints() {
  const savedMicrophoneId = sessionStorage.getItem("babelcast_microphone_id") || localStorage.getItem("babelcast_microphone_id");

  const constraints = {
    audio: {
      channels: 1,
      autoGainControl: true,
      echoCancellation: false,
      noiseSuppression: false,
    },
    video: false,
  };

  // If a specific microphone is selected (not default), add it to the constraints
  if (savedMicrophoneId && savedMicrophoneId !== "default") {
    constraints.audio.deviceId = { exact: savedMicrophoneId };
  }

  return constraints;
}

// Initialize on page load
document.addEventListener("DOMContentLoaded", function () {
  // Check if MediaDevices API is supported
  if (navigator.mediaDevices && navigator.mediaDevices.enumerateDevices) {
    // Request permission to access media devices
    navigator.mediaDevices
      .getUserMedia({ audio: true })
      .then((stream) => {
        // Release the stream immediately, we just needed permission
        stream.getTracks().forEach((track) => track.stop());

        // Now that we have permission, enumerate devices
        getAvailableAudioDevices();

        // Set up device change listener
        navigator.mediaDevices.addEventListener(
          "devicechange",
          getAvailableAudioDevices
        );
      })
      .catch((error) => {
        console.error("Error accessing media devices:", error);
      });
  } else {
    console.error("MediaDevices API not supported in this browser");
  }
});

// Export functions to be used in other scripts
window.getAudioConstraints = getAudioConstraints;
window.getAvailableAudioDevices = getAvailableAudioDevices;
