//// Translation
var translation_de = {
  title: "Live-Audio-Stream",
  // subtitle: "Live-Audio-Stream",
  unsupported_browser:
    "Ihr Webbrowser unterstützt leider kein WebRTC, das für diese Anwendung erforderlich ist. Bitte versuchen Sie einen anderen Webbrowser.",
  channels_header: "Kanäle",
  channels_description:
    "Klicken Sie auf einen Kanalnamen unten, um eine Verbindung herzustellen.",
  no_channels: "Keine Kanäle gefunden",
  switch_channel: "Kanal wechseln",
  span_reload: "Neu laden",
  btn_original: "Direkte Übertragung",
  btn_translation: "Übersetzung",
  powered_by: "Ermöglicht durch",
};

var translation_es = {
  title: "Transmisión de Audio en Vivo",
  // subtitle: "Transmisión de Audio en Vivo",
  unsupported_browser:
    "Desafortunadamente, su navegador web no parece admitir WebRTC, que es necesario para esta aplicación. Por favor, pruebe con otro navegador web.",
  channels_header: "Canales",
  channels_description:
    "Haga clic en un nombre de canal a continuación para conectarse.",
  no_channels: "No se encontraron canales",
  switch_channel: "Cambiar canal",
  span_reload: "recargar",
  btn_translation: "traducción",
  btn_original: "original",
  powered_by: "Proporcionado por",
};

// Function to set translations based on language
function setTranslations(translations) {
  document.getElementById("title").textContent = translations["title"];
  // document.getElementById("subtitle").textContent = translations["subtitle"];
  document.getElementById("unsupported_browser").textContent =
    translations["unsupported_browser"];
  document.getElementById("channels_description").textContent =
    translations["channels_description"];
  document.getElementById("nochannels").textContent =
    translations["no_channels"];
  document.getElementById("powered_by").textContent =
    translations["powered_by"];
  // Check if the original button is available
  const originalButton = document.getElementById("btn_original");
  if (originalButton) {
    originalButton.textContent = translations["btn_original"];
  }

  // Check if the translation button is available
  const translationButton = document.getElementById("btn_translation");
  if (translationButton) {
    translationButton.textContent = translations["btn_translation"];
  }

  const reloadButton = document.getElementById("span_reload");
  if (reloadButton) {
    reloadButton.textContent = translations["span_reload"];
  }
}
function translate_text() {
  // Check if the language starts with "de" (for German)
  if (navigator.language.startsWith("de")) {
    setTranslations(translation_de);
  }
  // // Check if the language starts with "es" (for Spanish)
  // else if (navigator.language.startsWith("es")) {
  //   setTranslations(translation_es);
  // }
}

translate_text();

// Theme Switcher

const themeButton = document.getElementById("themeButton");
const themes = ["white", "default", "dark", "blue", "green"];

// Load saved theme on page load
let currentTheme = localStorage.getItem("lab_page_theme") || 0;
document.body.className = themes[currentTheme];

// Save theme on change
themeButton.addEventListener("click", () => {
  currentTheme = (currentTheme + 1) % themes.length;
  document.body.className = themes[currentTheme];
  localStorage.setItem("lab_page_theme", currentTheme); // Save the selected theme
});

function detectConnectionLoss(callback, checkInterval = 1000) {
  let wasOnline = true;
  let stopped = false;

  function checkConnection() {
    if (stopped) return;
    // Use a small image or endpoint that should always be available
    fetch("/favicon.ico", {
      mode: "no-cors",
      cache: "no-store",
    })
      .then(() => {
        wasOnline = true;
      })
      .catch(() => {
        if (wasOnline) {
          wasOnline = false;
          callback();
        }
      })
      .finally(() => {
        if (!stopped) setTimeout(checkConnection, checkInterval);
      });
  }

  // Start checking
  checkConnection();
  return function stop() { stopped = true; };
}
