(function () {
  "use strict";

  var root = document.querySelector("[data-tombscript-root]");
  if (!root) return;

  var stateNode = root.querySelector("[data-tombscript-state]");
  var state = {};
  try {
    state = JSON.parse(stateNode ? stateNode.textContent : "{}");
  } catch (error) {
    state = {};
  }

  var pendingReveal = false;
  var pendingGuess = false;
  var nameNode = root.querySelector("[data-tombscript-name]");
  var manaCostNode = root.querySelector("[data-tombscript-mana-cost]");
  var rulesNode = root.querySelector("[data-tombscript-rules]");
  var flavorNode = root.querySelector("[data-tombscript-flavor]");
  var remainingNode = root.querySelector("[data-tombscript-remaining]");
  var announcementNode = root.querySelector("[data-tombscript-announcement]");
  var characterForm = root.querySelector("[data-tombscript-character-form]");
  var keyButtons = root.querySelectorAll("[data-tombscript-key]");
  var gameIDNode = root.querySelector("[data-tombscript-game-id]");

  function normalizeChoice(value) {
    value = String(value || "").trim();
    if (!value) return "";
    if (value.charAt(0) === "{" && value.charAt(value.length - 1) === "}") {
      var token = value.slice(1, -1).trim().toUpperCase();
      return token ? "{" + token + "}" : "";
    }
    var character = Array.from(value)[0] || "";
    try {
      return /^[\p{L}\p{N}]$/u.test(character) ? character.toLocaleLowerCase() : "";
    } catch (error) {
      return /^[a-z0-9]$/i.test(character) ? character.toLowerCase() : "";
    }
  }

  function announce(message) {
    if (!announcementNode) return;
    announcementNode.textContent = "";
    window.setTimeout(function () {
      announcementNode.textContent = message || "";
    }, 0);
  }

  function renderMaskedSymbols(node, value) {
    if (!node) return;
    node.innerHTML = "";
    var source = String(value || "");
    var tokenPattern = /\{([^}]+)\}/g;
    var index = 0;
    var match;
    while ((match = tokenPattern.exec(source)) !== null) {
      if (match.index > index) node.appendChild(document.createTextNode(source.slice(index, match.index)));
      var raw = match[1];
      if (raw === "_") {
        var hidden = document.createElement("span");
        hidden.className = "mt-tombscript__masked-symbol";
        hidden.setAttribute("aria-label", "Hidden symbol");
        node.appendChild(hidden);
      } else {
        var image = document.createElement("img");
        image.src = "https://svgs.scryfall.io/card-symbols/" + raw.toUpperCase().replace(/\//g, "") + ".svg";
        image.alt = "{" + raw + "}";
        node.appendChild(image);
      }
      index = tokenPattern.lastIndex;
    }
    if (index < source.length) node.appendChild(document.createTextNode(source.slice(index)));
  }

  function stateSymbolMap() {
    var result = {};
    var symbolKeys = Array.isArray(state.SymbolKeys) ? state.SymbolKeys : [];
    for (var index = 0; index < symbolKeys.length; index++) {
      result[normalizeChoice(symbolKeys[index].Value)] = symbolKeys[index];
    }
    return result;
  }

  function updateKeys() {
    var guessed = {};
    var guessedChars = Array.isArray(state.GuessedChars) ? state.GuessedChars : [];
    for (var index = 0; index < guessedChars.length; index++) {
      guessed[normalizeChoice(guessedChars[index])] = true;
    }
    var symbols = stateSymbolMap();
    var revealedText = [state.MaskedName, state.MaskedManaCost, state.MaskedRulesText, state.MaskedFlavorText]
      .join(" ")
      .replace(/\{[^}]*\}/g, "")
      .toLocaleLowerCase();

    for (var keyIndex = 0; keyIndex < keyButtons.length; keyIndex++) {
      var keyButton = keyButtons[keyIndex];
      var choice = normalizeChoice(keyButton.getAttribute("data-tombscript-key"));
      var symbolState = symbols[choice];
      var wasGuessed = symbolState ? Boolean(symbolState.Guessed) : Boolean(guessed[choice]);
      var wasHit = symbolState ? Boolean(symbolState.Hit) : (wasGuessed && revealedText.indexOf(choice) !== -1);
      keyButton.classList.toggle("is-hit", wasHit);
      keyButton.classList.toggle("is-miss", wasGuessed && !wasHit);
      keyButton.disabled = pendingReveal || wasGuessed || !state.CanRevealChar;
      var baseLabel = keyButton.querySelector(".sr-only");
      baseLabel = baseLabel ? baseLabel.textContent.trim() : keyButton.textContent.trim();
      keyButton.setAttribute(
        "aria-label",
        baseLabel + (wasGuessed ? (wasHit ? ", found" : ", not found") : "")
      );
    }
  }

  function updateState(nextState) {
    if (!nextState) return;
    state = nextState;
    if (nameNode) nameNode.textContent = state.MaskedName || "";
    renderMaskedSymbols(manaCostNode, state.MaskedManaCost || "");
    if (rulesNode) {
      if (state.HasRulesText) renderMaskedSymbols(rulesNode, state.MaskedRulesText || "");
      else rulesNode.textContent = "No rules text.";
    }
    if (flavorNode) {
      if (state.HasFlavorText) renderMaskedSymbols(flavorNode, state.MaskedFlavorText || "");
      else flavorNode.textContent = "No flavor text.";
    }
    if (remainingNode) remainingNode.textContent = String(state.RemainingGuesses || 0);
    updateKeys();
    renderGuessSlots();
    updateGuessModalState();
  }

  async function submitChoice(value) {
    var choice = normalizeChoice(value);
    if (!choice || !state.CanRevealChar || pendingReveal) return;

    pendingReveal = true;
    updateKeys();
    try {
      var body = new URLSearchParams();
      body.set("action", "char");
      body.set("game_id", gameIDNode ? gameIDNode.value : String(state.GameID || ""));
      body.set("char", choice);
      var response = await fetch("/games/spellify", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "fetch"
        },
        body: body.toString()
      });
      var payload = await response.json();
      if (payload && payload.Data) updateState(payload.Data);
      announce((payload && payload.Message) || "Reveal recorded.");
    } catch (error) {
      announce("That reveal could not be recorded. Try again.");
    } finally {
      pendingReveal = false;
      updateKeys();
    }
  }

  if (characterForm) {
    characterForm.addEventListener("submit", function (event) {
      event.preventDefault();
      if (!event.submitter) return;
      submitChoice(event.submitter.value);
    });
  }

  var guessModal = document.querySelector("[data-tombscript-guess-modal]");
  var guessPanel = guessModal && guessModal.querySelector("[data-tombscript-guess-panel]");
  var guessOpen = root.querySelector("[data-tombscript-guess-open]");
  var guessClose = guessModal && guessModal.querySelector("[data-tombscript-guess-close]");
  var guessCancel = guessModal && guessModal.querySelector("[data-tombscript-guess-cancel]");
  var guessForm = guessModal && guessModal.querySelector("[data-tombscript-guess-form]");
  var guessInput = guessModal && guessModal.querySelector("[data-tombscript-guess-input]");
  var guessSubmit = guessModal && guessModal.querySelector("[data-tombscript-guess-submit]");
  var guessMessage = guessModal && guessModal.querySelector("[data-tombscript-guess-message]");
  var modalGuessesLeft = guessModal && guessModal.querySelector("[data-tombscript-modal-guesses-left]");
  var modalGuessUnit = guessModal && guessModal.querySelector("[data-tombscript-modal-guess-unit]");
  var guessSlots = guessModal && guessModal.querySelector("[data-tombscript-guess-slots]");
  var guessHistory = guessModal && guessModal.querySelector("[data-tombscript-guess-history]");
  var guessHistoryList = guessModal && guessModal.querySelector("[data-tombscript-guess-history-list]");
  var guessBackgroundState = [];
  var guessPreviousOverflow = "";

  function guessIsOpen() {
    return guessModal && !guessModal.classList.contains("hidden");
  }

  function guessFocusableElements() {
    if (!guessPanel) return [];
    return Array.prototype.slice.call(guessPanel.querySelectorAll(
      'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'
    )).filter(function (element) {
      return !element.hidden && element.getAttribute("aria-hidden") !== "true";
    });
  }

  function setGuessBackgroundInert(inert) {
    if (inert) {
      guessBackgroundState = [];
      var elements = [
        document.querySelector(".mt-site-header"),
        document.querySelector(".mt-tombscript"),
        document.querySelector("#site-footer")
      ];
      for (var index = 0; index < elements.length; index++) {
        var element = elements[index];
        if (!element) continue;
        guessBackgroundState.push({
          element: element,
          inert: element.hasAttribute("inert"),
          ariaHidden: element.getAttribute("aria-hidden")
        });
        element.setAttribute("inert", "");
        element.setAttribute("aria-hidden", "true");
      }
      return;
    }
    for (var stateIndex = 0; stateIndex < guessBackgroundState.length; stateIndex++) {
      var saved = guessBackgroundState[stateIndex];
      if (!saved.inert) saved.element.removeAttribute("inert");
      if (saved.ariaHidden === null) saved.element.removeAttribute("aria-hidden");
      else saved.element.setAttribute("aria-hidden", saved.ariaHidden);
    }
    guessBackgroundState = [];
  }

  function renderGuessSlots() {
    if (!guessSlots) return;
    guessSlots.innerHTML = "";
    var maskedName = String(state.MaskedName || "");
    var words = maskedName.split(" ");
    for (var wordIndex = 0; wordIndex < words.length; wordIndex++) {
      var wordNode = document.createElement("span");
      wordNode.className = "mt-tombscript-guess-modal__word";
      var characters = Array.from(words[wordIndex]);
      for (var characterIndex = 0; characterIndex < characters.length; characterIndex++) {
        var character = characters[characterIndex];
        var slot = document.createElement("span");
        if (character === "_") {
          slot.className = "mt-tombscript-guess-modal__slot is-hidden";
          slot.textContent = "?";
        } else if (/^[\p{L}\p{N}]$/u.test(character)) {
          slot.className = "mt-tombscript-guess-modal__slot";
          slot.textContent = character;
        } else {
          slot.className = "mt-tombscript-guess-modal__punctuation";
          slot.textContent = character;
        }
        wordNode.appendChild(slot);
      }
      guessSlots.appendChild(wordNode);
    }
  }

  function updateGuessModalState() {
    var remainingCardGuesses = Number(state.RemainingCardGuesses || 0);
    if (modalGuessesLeft) modalGuessesLeft.textContent = String(remainingCardGuesses);
    if (modalGuessUnit) modalGuessUnit.textContent = remainingCardGuesses === 1 ? "guess" : "guesses";
    renderPreviousWrongGuesses();
    var enabled = Boolean(state.CanGuess) && !pendingGuess;
    if (guessInput) guessInput.disabled = !enabled;
    if (guessSubmit) guessSubmit.disabled = !enabled;
    if (guessOpen) guessOpen.disabled = !state.CanGuess;
  }

  function renderPreviousWrongGuesses() {
    if (!guessHistory || !guessHistoryList) return;
    var guesses = Array.isArray(state.PreviousWrongGuesses) ? state.PreviousWrongGuesses : [];
    guessHistoryList.innerHTML = "";
    for (var index = 0; index < guesses.length; index++) {
      var item = document.createElement("li");
      item.textContent = String(guesses[index] || "");
      guessHistoryList.appendChild(item);
    }
    guessHistory.classList.toggle("hidden", guesses.length === 0);
  }

  function showGuessMessage(message) {
    if (!guessMessage) return;
    guessMessage.textContent = message || "";
    guessMessage.classList.toggle("hidden", !message);
  }

  function openGuessModal() {
    if (!guessModal || guessIsOpen() || !state.CanGuess) return;
    guessPreviousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    guessModal.classList.remove("hidden");
    guessModal.setAttribute("aria-hidden", "false");
    setGuessBackgroundInert(true);
    showGuessMessage("");
    renderGuessSlots();
    if (guessInput) guessInput.focus({ preventScroll: true });
    else if (guessPanel) guessPanel.focus({ preventScroll: true });
  }

  function closeGuessModal() {
    if (!guessIsOpen()) return;
    guessModal.classList.add("hidden");
    guessModal.setAttribute("aria-hidden", "true");
    document.body.style.overflow = guessPreviousOverflow;
    setGuessBackgroundInert(false);
    showGuessMessage("");
    if (guessInput) guessInput.value = "";
    if (guessOpen) guessOpen.focus({ preventScroll: true });
  }

  if (guessOpen) guessOpen.addEventListener("click", openGuessModal);
  if (guessClose) guessClose.addEventListener("click", closeGuessModal);
  if (guessCancel) guessCancel.addEventListener("click", closeGuessModal);

  if (guessModal) {
    guessModal.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        closeGuessModal();
        return;
      }
      if (event.key !== "Tab") return;
      var focusable = guessFocusableElements();
      if (focusable.length === 0) {
        event.preventDefault();
        if (guessPanel) guessPanel.focus();
        return;
      }
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
  }

  if (guessForm) {
    guessForm.addEventListener("submit", async function (event) {
      event.preventDefault();
      if (pendingGuess || !state.CanGuess || !guessInput || !guessInput.value.trim()) return;
      pendingGuess = true;
      updateGuessModalState();
      try {
        var body = new URLSearchParams();
        body.set("action", "guess");
        body.set("game_id", String(state.GameID || ""));
        body.set("guess", guessInput.value.trim());
        var response = await fetch("/games/spellify", {
          method: "POST",
          credentials: "same-origin",
          headers: {
            "Accept": "application/json",
            "Content-Type": "application/x-www-form-urlencoded",
            "X-Requested-With": "fetch"
          },
          body: body.toString()
        });
        var payload = await response.json();
        if (payload && payload.Data) updateState(payload.Data);
        if (payload && payload.Redirect) {
          window.location.assign(payload.Redirect);
          return;
        }
        showGuessMessage((payload && payload.Message) || "That guess could not be submitted.");
        guessInput.value = "";
        guessInput.focus({ preventScroll: true });
      } catch (error) {
        showGuessMessage("That guess could not be submitted. Try again.");
      } finally {
        pendingGuess = false;
        updateGuessModalState();
      }
    });
  }

  var giveUpForm = root.querySelector("[data-tombscript-give-up-form]");
  if (giveUpForm) {
    giveUpForm.addEventListener("submit", function (event) {
      if (!window.confirm("Reveal the card and end this round?")) {
        event.preventDefault();
        return;
      }
      var submitter = event.submitter || this.querySelector('button[type="submit"]');
      if (submitter) submitter.disabled = true;
    });
  }

  var helpModal = document.querySelector("[data-tombscript-help-modal]");
  var helpOpeners = document.querySelectorAll("[data-tombscript-help-open]");
  var helpClose = helpModal && helpModal.querySelector("[data-tombscript-help-close]");
  var helpPanel = helpModal && helpModal.querySelector("[data-tombscript-help-panel]");
  var helpStorageKey = "manatomb.tombscript.howToPlaySeen.v2";
  var helpRestoreFocus = null;
  var previousBodyOverflow = "";
  var helpBackgroundState = [];

  function helpIsOpen() {
    return helpModal && !helpModal.classList.contains("hidden");
  }

  function helpFocusableElements() {
    if (!helpPanel) return [];
    return Array.prototype.slice.call(helpPanel.querySelectorAll(
      'button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter(function (element) {
      return !element.hidden && element.getAttribute("aria-hidden") !== "true";
    });
  }

  function setHelpBackgroundInert(inert) {
    if (inert) {
      helpBackgroundState = [];
      var elements = [
        document.querySelector(".mt-site-header"),
        document.querySelector(".mt-tombscript"),
        document.querySelector("#site-footer")
      ];
      for (var index = 0; index < elements.length; index++) {
        var element = elements[index];
        if (!element) continue;
        helpBackgroundState.push({
          element: element,
          inert: element.hasAttribute("inert"),
          ariaHidden: element.getAttribute("aria-hidden")
        });
        element.setAttribute("inert", "");
        element.setAttribute("aria-hidden", "true");
      }
      return;
    }

    for (var stateIndex = 0; stateIndex < helpBackgroundState.length; stateIndex++) {
      var saved = helpBackgroundState[stateIndex];
      if (!saved.inert) saved.element.removeAttribute("inert");
      if (saved.ariaHidden === null) saved.element.removeAttribute("aria-hidden");
      else saved.element.setAttribute("aria-hidden", saved.ariaHidden);
    }
    helpBackgroundState = [];
  }

  function openHelpModal(trigger) {
    if (!helpModal || helpIsOpen() || guessIsOpen()) return;
    helpRestoreFocus = trigger || null;
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    helpModal.classList.remove("hidden");
    helpModal.setAttribute("aria-hidden", "false");
    setHelpBackgroundInert(true);
    if (helpClose) helpClose.focus({ preventScroll: true });
    else if (helpPanel) helpPanel.focus({ preventScroll: true });
  }

  function rememberHelpDismissal() {
    try {
      window.localStorage.setItem(helpStorageKey, "1");
    } catch (error) {
      // The manual How to Play control remains available when storage is blocked.
    }
  }

  function closeHelpModal() {
    if (!helpIsOpen()) return;
    helpModal.classList.add("hidden");
    helpModal.setAttribute("aria-hidden", "true");
    document.body.style.overflow = previousBodyOverflow;
    setHelpBackgroundInert(false);
    rememberHelpDismissal();
    var focusTarget = helpRestoreFocus || root.querySelector("[data-tombscript-key]:not(:disabled)") || root.querySelector("[data-tombscript-help-open]");
    helpRestoreFocus = null;
    if (focusTarget && typeof focusTarget.focus === "function") focusTarget.focus({ preventScroll: true });
  }

  for (var openerIndex = 0; openerIndex < helpOpeners.length; openerIndex++) {
    helpOpeners[openerIndex].addEventListener("click", function () { openHelpModal(this); });
  }
  if (helpClose) helpClose.addEventListener("click", closeHelpModal);

  if (helpModal) {
    helpModal.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        closeHelpModal();
        return;
      }
      if (event.key !== "Tab") return;
      var focusable = helpFocusableElements();
      if (focusable.length === 0) {
        event.preventDefault();
        if (helpPanel) helpPanel.focus();
        return;
      }
      var first = focusable[0];
      var last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
    if (helpModal.getAttribute("data-auto-open") === "true") {
      try {
        if (window.localStorage.getItem(helpStorageKey) !== "1") openHelpModal(null);
      } catch (error) {
        openHelpModal(null);
      }
    }
  }

  document.addEventListener("keydown", function (event) {
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || helpIsOpen() || guessIsOpen()) return;
    var target = event.target;
    if (target && target.closest && target.closest("input, textarea, select, a, button, [contenteditable='true']")) return;
    var choice = normalizeChoice(event.key);
    if (!choice || choice.charAt(0) === "{" || !state.CanRevealChar || pendingReveal) return;
    event.preventDefault();
    submitChoice(choice);
  });

  updateState(state);
})();
