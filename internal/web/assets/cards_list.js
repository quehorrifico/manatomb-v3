(function () {
  "use strict";

  function clean(value) {
    return String(value || "").trim();
  }

  function faceValue(face, key) {
    if (!face) return "";
    var aliases = {
      image_uri: ["image_uri", "ImageURI", "ImageUri"],
      name: ["name", "Name"]
    };
    var keys = aliases[key] || [key];
    for (var index = 0; index < keys.length; index += 1) {
      var value = clean(face[keys[index]]);
      if (value) return value;
    }
    return "";
  }

  function setTileImage(tile, imageURI, name) {
    var image = tile.querySelector("[data-card-result-image]");
    var fallback = tile.querySelector("[data-card-result-fallback]");
    imageURI = clean(imageURI);
    name = clean(name) || clean(tile.getAttribute("data-card-result-front-name")) || "Card";

    if (image && imageURI) {
      image.src = imageURI;
      image.alt = name;
      image.classList.remove("hidden");
      if (fallback) fallback.classList.add("hidden");
      return;
    }

    if (image) {
      image.removeAttribute("src");
      image.alt = "";
      image.classList.add("hidden");
    }
    if (fallback) {
      fallback.textContent = name;
      fallback.classList.remove("hidden");
    }
  }

  document.querySelectorAll("[data-card-result-tile]").forEach(function (tile) {
    var rotateButton = tile.querySelector("[data-card-result-rotate]");
    if (rotateButton) {
      rotateButton.addEventListener("click", function () {
        var image = tile.querySelector("[data-card-result-image]");
        var rotated = !tile.classList.contains("is-rotated");
        tile.classList.toggle("is-rotated", rotated);
        if (image) image.classList.toggle("is-rotated", rotated);
        rotateButton.setAttribute("aria-pressed", rotated ? "true" : "false");
        rotateButton.setAttribute("aria-label", rotated ? "Reset card orientation" : "Rotate card");
      });
      return;
    }

    var turnButton = tile.querySelector("[data-card-result-turn-over]");
    if (!turnButton) return;

    var faces = [];
    try {
      var parsed = JSON.parse(tile.getAttribute("data-card-result-faces") || "[]");
      if (Array.isArray(parsed)) faces = parsed;
    } catch (error) {
      faces = [];
    }
    if (faces.length < 2) {
      turnButton.hidden = true;
      return;
    }

    var faceIndex = 0;
    turnButton.addEventListener("click", function () {
      faceIndex = faceIndex === 0 ? 1 : 0;
      var face = faces[faceIndex] || {};
      var imageURI = faceValue(face, "image_uri");
      if (!imageURI && faceIndex === 0) {
        imageURI = tile.getAttribute("data-card-result-front-image");
      }
      var name = faceValue(face, "name") || tile.getAttribute("data-card-result-front-name");
      setTileImage(tile, imageURI, name);
      turnButton.setAttribute("aria-label", faceIndex === 0 ? "Show back face" : "Show front face");
    });
  });
})();
