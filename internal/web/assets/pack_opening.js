(function () {
  "use strict";

  var root = document.querySelector("[data-pack-root]");
  if (!root) return;

  function readJSON(selector, fallback) {
    var node = root.parentElement && root.parentElement.querySelector(selector);
    if (!node) node = document.querySelector(selector);
    if (!node) return fallback;
    try {
      return JSON.parse(node.textContent || "null") || fallback;
    } catch (_) {
      return fallback;
    }
  }

  var sets = readJSON("[data-pack-sets]", []);
  var optionsBySet = readJSON("[data-pack-type-options]", {});
  var rail = root.querySelector("[data-pack-set-rail]");
  var typeSection = root.querySelector("[data-pack-type-section]");
  var typeList = root.querySelector("[data-pack-type-list]");
  var selectedSetLabel = root.querySelector("[data-pack-selected-set]");
  var accuracy = root.querySelector("[data-pack-accuracy]");
  var simulationButton = root.querySelector("[data-pack-simulation-open]");
  var simulationDialog = root.querySelector("[data-pack-simulation-dialog]");
  var simulationClose = root.querySelector("[data-pack-simulation-close]");
  var simulationConfidence = root.querySelector("[data-pack-simulation-confidence]");
  var simulationSummary = root.querySelector("[data-pack-simulation-summary]");
  var simulationRecipeSection = root.querySelector("[data-pack-simulation-recipe-section]");
  var simulationRecipe = root.querySelector("[data-pack-simulation-recipe]");
  var simulationLimitationsSection = root.querySelector("[data-pack-simulation-limitations-section]");
  var simulationLimitations = root.querySelector("[data-pack-simulation-limitations]");
  var simulationSource = root.querySelector("[data-pack-simulation-source]");
  var opening = root.querySelector("[data-pack-opening]");
  var stageTitle = root.querySelector("[data-pack-stage-title]");
  var stageInstruction = root.querySelector("[data-pack-stage-instruction]");
  var stage = root.querySelector("[data-pack-stage]");
  var revealAllButton = root.querySelector("[data-pack-reveal-all]");
  var resetButton = root.querySelector("[data-pack-reset]");
  var loader = root.querySelector("[data-pack-loader]");
  var scene = root.querySelector("[data-pack-scene]");
  var wrapper = root.querySelector("[data-pack-wrapper]");
  var wrapperSymbolImage = root.querySelector("[data-pack-wrapper-symbol-image]");
  var wrapperSymbolFallback = root.querySelector("[data-pack-wrapper-symbol-fallback]");
  var wrapperSet = root.querySelector("[data-pack-wrapper-set]");
  var wrapperType = root.querySelector("[data-pack-wrapper-type]");
  var openSlider = root.querySelector("[data-pack-open-slider]");
  var openSliderFill = root.querySelector(".mt-pack-open-slider__fill");
  var openSliderHandle = root.querySelector("[data-pack-open-slider-handle]");
  var stack = root.querySelector("[data-pack-stack]");
  var current = root.querySelector("[data-pack-current]");
  var currentName = root.querySelector("[data-pack-current-name]");
  var currentPrice = root.querySelector("[data-pack-current-price]");
  var currentPulls = root.querySelector("[data-pack-current-pulls]");
  var currentPullsGrid = root.querySelector("[data-pack-current-pulls-grid]");
  var pulls = root.querySelector("[data-pack-pulls]");
  var pullCount = root.querySelector("[data-pack-pull-count]");
  var totalValue = root.querySelector("[data-pack-total-value]");
  var pullsGrid = root.querySelector("[data-pack-pulls-grid]");
  var live = root.querySelector("[data-pack-live]");
  var reducedMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  var state = {
    phase: "idle",
    set: null,
    packType: null,
    cards: [],
    currentIndex: 0,
    packPullCount: 0,
    packPullValueCents: 0,
    packCommitted: false,
    sessionPullCount: 0,
    sessionPullValueCents: 0,
    requestToken: 0,
    sequenceToken: 0,
    animations: [],
    sliding: null,
    sliderProgress: 0,
    swiping: null,
    suppressClickNode: null,
    suppressClickTimer: 0,
    simulationTrigger: null
  };

  function field(value, name) {
    if (!value || typeof value !== "object") return "";
    var lower = name.toLowerCase();
    var snake = name.replace(/([a-z0-9])([A-Z])/g, "$1_$2").toLowerCase();
    if (value[name] !== undefined) return value[name];
    if (value[snake] !== undefined) return value[snake];
    return value[lower];
  }

  function text(value) {
    return String(value === undefined || value === null ? "" : value).trim();
  }

  function textList(value, name) {
    var items = field(value, name);
    if (!Array.isArray(items)) return [];
    return items.map(text).filter(Boolean);
  }

  function safeHTTPSURL(value) {
    var candidate = text(value);
    if (!/^https:\/\//i.test(candidate)) return "";
    try {
      return new URL(candidate).href;
    } catch (_) {
      return "";
    }
  }

  function setPhase(phase) {
    state.phase = phase;
    root.dataset.state = phase;
  }

  function announce(message) {
    if (!live) return;
    live.textContent = "";
    window.requestAnimationFrame(function () {
      live.textContent = message;
    });
  }

  function setName(item) {
    return text(field(item, "Name")) || text(field(item, "Label")) || text(field(item, "Code")).toUpperCase();
  }

  function setCode(item) {
    return text(field(item, "Code")).toLowerCase();
  }

  function setYear(item) {
    var released = text(field(item, "ReleasedAt"));
    return /^\d{4}/.test(released) ? released.slice(0, 4) : "";
  }

  function setIconURL(item) {
    var candidate = text(field(item, "IconSVGURI")) || text(item && item.icon_svg_uri) || text(item && item.iconSvgUri);
    if (!candidate) return "";
    try {
      var parsed = new URL(candidate);
      return parsed.protocol === "https:" ? parsed.href : "";
    } catch (_) {
      return "";
    }
  }

  function packTypesFor(item) {
    var options = optionsBySet[setCode(item)];
    return Array.isArray(options) ? options : [];
  }

  function packTypeID(item) {
    return text(field(item, "ID")).toLowerCase();
  }

  function packTypeName(item) {
    return text(field(item, "Name")) || "Booster";
  }

  function packTypeCount(item) {
    var count = Number(field(item, "CardCount"));
    return Number.isFinite(count) && count > 0 ? count : 0;
  }

  function priceNumber(value) {
    var number = Number.parseFloat(text(value));
    return Number.isFinite(number) && number > 0 ? number : 0;
  }

  function priceText(value) {
    var number = priceNumber(value);
    return number > 0 ? "$" + number.toFixed(2) : "Price unavailable";
  }

  function priceCents(value) {
    return Math.round(priceNumber(value) * 100);
  }

  function finishName(card) {
    var finish = text(card && card.finish).toLowerCase();
    if (finish === "etched") return "Etched";
    if (finish === "foil") return "Foil";
    return "";
  }

  function dollarsFromCents(cents) {
    return "$" + (cents / 100).toFixed(2);
  }

  function play(node, frames, options) {
    function applyLastFrame() {
      if (!node || !frames.length) return;
      var last = frames[frames.length - 1];
      Object.keys(last).forEach(function (property) {
        if (property !== "offset" && property !== "easing" && property !== "composite") {
          node.style[property] = String(last[property]);
        }
      });
    }
    if (!node || reducedMotion || typeof node.animate !== "function") {
      applyLastFrame();
      return Promise.resolve();
    }
    var animation = node.animate(frames, options);
    state.animations.push(animation);
    function untrack() {
      var index = state.animations.indexOf(animation);
      if (index >= 0) state.animations.splice(index, 1);
    }
    return animation.finished.then(function () {
      untrack();
      applyLastFrame();
      animation.cancel();
    }).catch(function () {
      untrack();
    });
  }

  function cancelChoreography() {
    state.sequenceToken += 1;
    state.animations.slice().forEach(function (animation) {
      try { animation.cancel(); } catch (_) {}
    });
    state.animations = [];
  }

  function preloadImages(imageURLs) {
    var seen = {};
    var urls = (imageURLs || []).map(text).filter(function (url) {
      if (!url || seen[url]) return false;
      seen[url] = true;
      return true;
    });
    return Promise.all(urls.map(function (url) {
      return new Promise(function (resolve) {
        var settled = false;
        var image = new Image();
        var timer = window.setTimeout(done, 6000);
        function done() {
          if (settled) return;
          settled = true;
          window.clearTimeout(timer);
          resolve();
        }
        image.onload = function () {
          if (typeof image.decode === "function") {
            image.decode().catch(function () {}).then(done);
          } else {
            done();
          }
        };
        image.onerror = done;
        image.src = url;
      });
    }));
  }

  function preloadCardImages(cards) {
    return preloadImages((cards || []).map(function (card) {
      return text(card && card.image_uri);
    }));
  }

  function sortSetsChronologically(items) {
    return items.slice().sort(function (a, b) {
      var left = text(field(a, "ReleasedAt"));
      var right = text(field(b, "ReleasedAt"));
      if (left === right) return setName(a).localeCompare(setName(b));
      if (!left) return -1;
      if (!right) return 1;
      return left.localeCompare(right);
    });
  }

  function renderSets() {
    rail.innerHTML = "";
    var ordered = sortSetsChronologically(sets);
    ordered.forEach(function (item) {
      var button = document.createElement("button");
      button.type = "button";
      button.className = "mt-pack-set";
      button.setAttribute("aria-pressed", "false");
      button.dataset.setCode = setCode(item);

      var identity = document.createElement("span");
      identity.className = "mt-pack-set__identity";

      var symbol = document.createElement("span");
      symbol.className = "mt-pack-set__symbol";
      symbol.setAttribute("aria-hidden", "true");

      var fallback = document.createElement("span");
      fallback.className = "mt-pack-set__symbol-fallback";
      fallback.textContent = setCode(item).slice(0, 2).toUpperCase();
      symbol.appendChild(fallback);

      var iconURL = setIconURL(item);
      if (iconURL) {
        var icon = document.createElement("img");
        icon.className = "mt-pack-set__symbol-image";
        icon.src = iconURL;
        icon.alt = "";
        icon.loading = "lazy";
        icon.decoding = "async";
        icon.addEventListener("error", function () {
          icon.hidden = true;
          symbol.classList.add("is-fallback");
        });
        symbol.appendChild(icon);
      } else {
        symbol.classList.add("is-fallback");
      }

      var name = document.createElement("span");
      name.className = "mt-pack-set__name";
      name.textContent = setName(item);
      identity.appendChild(name);
      identity.appendChild(symbol);

      var meta = document.createElement("span");
      meta.className = "mt-pack-set__meta";
      var code = document.createElement("span");
      code.textContent = setCode(item).toUpperCase();
      var year = document.createElement("span");
      year.textContent = setYear(item);
      meta.appendChild(code);
      meta.appendChild(year);

      button.appendChild(identity);
      button.appendChild(meta);
      button.addEventListener("click", function () {
        selectSet(item, button);
      });
      rail.appendChild(button);
    });

    window.requestAnimationFrame(function () {
      rail.scrollLeft = rail.scrollWidth;
    });
  }

  function selectSet(item, button) {
    resetOpening(false);
    state.set = item;
    state.packType = null;
    setPhase("idle");
    rail.querySelectorAll(".mt-pack-set").forEach(function (node) {
      node.setAttribute("aria-pressed", node === button ? "true" : "false");
    });
    button.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "nearest", inline: "center" });
    selectedSetLabel.textContent = setName(item);
    renderPackTypes(item);
    typeSection.hidden = false;
    accuracy.textContent = "";
    announce(setName(item) + " selected. Choose a booster type.");
  }

  function renderPackTypes(item) {
    typeList.innerHTML = "";
    packTypesFor(item).forEach(function (packType) {
      var button = document.createElement("button");
      button.type = "button";
      button.className = "mt-pack-type";
      button.setAttribute("aria-pressed", "false");
      button.dataset.packType = packTypeID(packType);

      var name = document.createElement("span");
      name.className = "mt-pack-type__name";
      name.textContent = packTypeName(packType);
      var meta = document.createElement("span");
      meta.className = "mt-pack-type__meta";
      var count = packTypeCount(packType);
      meta.textContent = count ? count + " cards" : "Randomized booster";
      button.appendChild(name);
      button.appendChild(meta);
      button.addEventListener("click", function () {
        selectPackType(packType, button);
      });
      typeList.appendChild(button);
    });
  }

  function updateAccuracy(packType) {
    if (!accuracy) return;
    var label = text(field(packType, "AccuracyLabel")) || text(field(packType, "Accuracy"));
    accuracy.textContent = "";
    if (label) {
      accuracy.textContent = label;
    } else {
      var exact = Boolean(field(packType, "ExactOdds"));
      accuracy.textContent = exact
        ? "Published booster collation is modeled for this product."
        : "This newly synced product uses an approximate rarity model.";
    }
    updateSimulationDetails(packType);
  }

  function renderDetailList(node, items) {
    node.textContent = "";
    items.forEach(function (item) {
      var entry = document.createElement("li");
      entry.textContent = item;
      node.appendChild(entry);
    });
  }

  function updateSimulationDetails(packType) {
    if (!simulationButton || !simulationDialog) return;
    var label = text(field(packType, "AccuracyLabel")) || text(field(packType, "Accuracy"));
    var summary = text(field(packType, "AccuracySummary"));
    var recipe = textList(packType, "SlotRecipe");
    var limitations = textList(packType, "Limitations");
    var sourceURL = safeHTTPSURL(field(packType, "SourceURL"));
    var hasDetails = Boolean(summary || recipe.length || limitations.length || sourceURL);

    simulationButton.hidden = !hasDetails;
    simulationButton.setAttribute("aria-label", "Simulation details for " + packTypeName(packType));
    simulationConfidence.textContent = label || "Simulation model";
    simulationSummary.textContent = summary || "See how this booster product is modeled.";

    renderDetailList(simulationRecipe, recipe);
    simulationRecipeSection.hidden = recipe.length === 0;
    renderDetailList(simulationLimitations, limitations);
    simulationLimitationsSection.hidden = limitations.length === 0;

    simulationSource.hidden = !sourceURL;
    if (sourceURL) {
      simulationSource.href = sourceURL;
      simulationSource.target = "_blank";
      simulationSource.rel = "noopener noreferrer";
      simulationSource.setAttribute("aria-label", "View official source for " + packTypeName(packType) + " (opens in a new tab)");
    } else {
      simulationSource.removeAttribute("href");
      simulationSource.removeAttribute("target");
      simulationSource.removeAttribute("rel");
      simulationSource.removeAttribute("aria-label");
    }
  }

  function clearSimulationDetails() {
    if (!simulationButton) return;
    simulationButton.hidden = true;
    simulationButton.removeAttribute("aria-label");
    simulationConfidence.textContent = "";
    simulationSummary.textContent = "";
    simulationRecipe.textContent = "";
    simulationRecipeSection.hidden = true;
    simulationLimitations.textContent = "";
    simulationLimitationsSection.hidden = true;
    simulationSource.hidden = true;
    simulationSource.removeAttribute("href");
    simulationSource.removeAttribute("target");
    simulationSource.removeAttribute("rel");
  }

  function openSimulationDetails() {
    if (!simulationDialog || simulationButton.hidden) return;
    state.simulationTrigger = document.activeElement;
    if (typeof simulationDialog.showModal === "function") {
      simulationDialog.showModal();
      simulationClose.focus({ preventScroll: true });
    }
  }

  function closeSimulationDetails() {
    if (simulationDialog && simulationDialog.open) simulationDialog.close();
  }

  function attachSimulationDetails() {
    if (!simulationButton || !simulationDialog || !simulationClose) return;
    simulationButton.addEventListener("click", openSimulationDetails);
    simulationClose.addEventListener("click", closeSimulationDetails);
    simulationDialog.addEventListener("click", function (event) {
      if (event.target === simulationDialog) closeSimulationDetails();
    });
    simulationDialog.addEventListener("close", function () {
      var trigger = state.simulationTrigger;
      state.simulationTrigger = null;
      if (trigger && trigger.isConnected && typeof trigger.focus === "function") {
        trigger.focus({ preventScroll: true });
      }
    });
  }

  function selectPackType(packType, button) {
    if (!state.set) return;
    resetOpening(false);
    state.packType = packType;
    typeList.querySelectorAll(".mt-pack-type").forEach(function (node) {
      node.setAttribute("aria-pressed", node === button ? "true" : "false");
    });
    updateAccuracy(packType);
    loadPack(button, { autoOpen: false, scroll: true });
  }

  async function loadPack(typeButton, options) {
    options = options || {};
    var token = ++state.requestToken;
    setPhase("loading");
    opening.hidden = false;
    stage.hidden = false;
    loader.hidden = false;
    scene.hidden = true;
    current.hidden = true;
    pulls.hidden = state.sessionPullCount === 0;
    resetButton.hidden = true;
    stageTitle.textContent = setName(state.set) + " " + packTypeName(state.packType);
    stageInstruction.textContent = "Preparing your pack…";
    if (typeButton) typeButton.setAttribute("aria-busy", "true");
    if (options.scroll !== false) {
      opening.scrollIntoView({ behavior: reducedMotion ? "auto" : "smooth", block: "start" });
    }

    try {
      var body = new URLSearchParams();
      body.set("set_code", setCode(state.set));
      body.set("pack_type", packTypeID(state.packType));
      var response = await fetch("/games/pack-opening", {
        method: "POST",
        credentials: "same-origin",
        headers: {
          "Accept": "application/json",
          "Content-Type": "application/x-www-form-urlencoded"
        },
        body: body.toString()
      });
      if (token !== state.requestToken) return;
      var payload = await response.json();
      if (token !== state.requestToken) return;
      if (!response.ok || !payload || !payload.ok) {
        throw new Error(payload && payload.message ? payload.message : "This pack could not be prepared.");
      }
      if (token !== state.requestToken) return;
      state.set = payload.set || state.set;
      state.packType = payload.pack_type || state.packType;
      state.cards = Array.isArray(payload.cards) ? payload.cards : [];
      await preloadCardImages(state.cards.slice(0, 3));
      if (token !== state.requestToken) return;
      preloadCardImages(state.cards.slice(3));
      prepareSealedPack(Boolean(options.autoOpen));
    } catch (error) {
      if (token !== state.requestToken) return;
      setPhase("idle");
      loader.hidden = true;
      scene.hidden = true;
      stageInstruction.textContent = error && error.message ? error.message : "Try a different booster.";
      resetButton.hidden = true;
      announce(stageInstruction.textContent);
    } finally {
      if (typeButton) typeButton.removeAttribute("aria-busy");
    }
  }

  function prepareSealedPack(autoOpen) {
    state.currentIndex = 0;
    state.packPullCount = 0;
    state.packPullValueCents = 0;
    state.packCommitted = false;
    currentPullsGrid.innerHTML = "";
    currentPulls.hidden = true;
    currentPulls.classList.remove("is-complete");
    updatePullSummary();
    wrapperSet.textContent = setName(state.set);
    wrapperType.textContent = packTypeName(state.packType);
    applyWrapperIdentity();
    renderStack();
    wrapper.classList.remove("is-opening", "is-unsealed");
    wrapper.style.removeProperty("opacity");
    wrapper.style.removeProperty("transform");
    wrapper.style.removeProperty("visibility");
    stack.style.removeProperty("opacity");
    stack.style.removeProperty("transform");
    stack.style.removeProperty("z-index");
    openSlider.style.removeProperty("opacity");
    openSlider.style.removeProperty("transform");
    openSliderFill.style.removeProperty("width");
    openSliderHandle.style.removeProperty("transform");
    updateOpenSlider(0);
    loader.hidden = true;
    stage.hidden = false;
    scene.hidden = false;
    current.hidden = true;
    pulls.hidden = state.sessionPullCount === 0;
    revealAllButton.hidden = true;
    revealAllButton.disabled = false;
    resetButton.hidden = true;
    stageTitle.textContent = setName(state.set) + " " + packTypeName(state.packType);
    stageInstruction.textContent = autoOpen
      ? "Opening another " + packTypeName(state.packType).toLowerCase() + "…"
      : "Drag the slider across the top of the pack to open it.";
    setPhase("sealed");
    if (autoOpen) {
      announce("Another " + packTypeName(state.packType) + " is ready and opening automatically.");
      window.requestAnimationFrame(function () {
        autoOpenPack();
      });
    } else {
      announce("Pack ready. Slide right across the top of the pack to open it.");
      openSlider.focus({ preventScroll: true });
    }
  }

  function applyWrapperIdentity() {
    var code = setCode(state.set).toUpperCase();
    var iconURL = setIconURL(state.set);
    wrapper.dataset.setCode = setCode(state.set);
    wrapper.dataset.packType = packTypeID(state.packType);
    wrapper.setAttribute("aria-label", setName(state.set) + " " + packTypeName(state.packType));
    wrapperSymbolFallback.textContent = code || "MTG";
    wrapperSymbolFallback.hidden = false;
    wrapperSymbolImage.hidden = true;
    wrapperSymbolImage.removeAttribute("src");
    if (iconURL) {
      wrapperSymbolImage.src = iconURL;
      wrapperSymbolImage.hidden = false;
      wrapperSymbolFallback.hidden = true;
    }
  }

  function renderStack() {
    stack.innerHTML = "";
    state.cards.forEach(function (card, index) {
      if (card && typeof card === "object") delete card.__packRecorded;
      var cardButton = document.createElement("button");
      cardButton.type = "button";
      cardButton.className = "mt-pack-card";
      cardButton.dataset.cardIndex = String(index);
      cardButton.dataset.finish = text(card.finish).toLowerCase();
      cardButton.setAttribute("aria-label", "Reveal the next card");
      cardButton.tabIndex = -1;

      var inner = document.createElement("span");
      inner.className = "mt-pack-card__inner";
      var back = document.createElement("span");
      back.className = "mt-pack-card__face mt-pack-card__back";
      back.setAttribute("aria-hidden", "true");
      var front = document.createElement("span");
      front.className = "mt-pack-card__face mt-pack-card__front";
      if (text(card.image_uri)) {
        var image = document.createElement("img");
        image.src = text(card.image_uri);
        image.alt = text(card.name) || "Pulled card";
        image.draggable = false;
        front.appendChild(image);
      } else {
        var fallback = document.createElement("span");
        fallback.className = "mt-pack-card__fallback";
        fallback.textContent = text(card.name) || "Card image unavailable";
        front.appendChild(fallback);
      }
      inner.appendChild(back);
      inner.appendChild(front);
      cardButton.appendChild(inner);
      cardButton.addEventListener("click", function (event) {
        if (state.suppressClickNode === cardButton) {
          event.preventDefault();
          state.suppressClickNode = null;
          return;
        }
        if (state.phase === "revealing" && index === state.currentIndex) revealCurrentCard(1);
      });
      attachCardSwipe(cardButton, index);
      stack.appendChild(cardButton);
    });
  }

  function cardNodes() {
    return Array.prototype.slice.call(stack.querySelectorAll(".mt-pack-card"));
  }

  async function autoOpenPack() {
    if (state.phase !== "sealed") return;
    var sequence = state.sequenceToken;
    var travel = sliderTravel();
    stageInstruction.textContent = "Opening…";
    await Promise.all([
      play(openSliderFill, [
        { width: "0%" },
        { width: "100%" }
      ], { duration: 260, easing: "cubic-bezier(.2,.75,.2,1)", fill: "forwards" }),
      play(openSliderHandle, [
        { transform: "translateX(0)" },
        { transform: "translateX(" + travel + "px)" }
      ], { duration: 260, easing: "cubic-bezier(.2,.75,.2,1)", fill: "forwards" })
    ]);
    if (sequence !== state.sequenceToken || state.phase !== "sealed") return;
    updateOpenSlider(100);
    openPack();
  }

  async function openPack() {
    if (state.phase !== "sealed") return;
    var sequence = ++state.sequenceToken;
    var wrapperCentered = "translate(-50%, -50%)";
    var stackSettled = "translate(-50%, -58%)";
    setPhase("opening");
    wrapper.classList.add("is-opening");
    stageInstruction.textContent = "Opening…";
    announce("The wrapper opens.");

    await play(openSlider, [
      { transform: "translateY(0)", opacity: 1 },
      { transform: "translateY(-0.75rem)", opacity: 0 }
    ], { duration: 190, easing: "cubic-bezier(.35,0,.2,1)", fill: "forwards" });
    if (sequence !== state.sequenceToken) return;
    openSlider.style.opacity = "0";
    wrapper.classList.add("is-unsealed");

    await play(wrapper, [
      { transform: wrapperCentered + " scale(1)", opacity: 1 },
      { transform: wrapperCentered + " scale(.985)", opacity: 1 },
      { transform: wrapperCentered + " scale(1)", opacity: 1 }
    ], { duration: 180, easing: "cubic-bezier(.2,.75,.2,1)", fill: "forwards" });
    if (sequence !== state.sequenceToken) return;

    var nodes = cardNodes();
    nodes.forEach(function (node, index) {
      node.style.opacity = "1";
      node.style.zIndex = String(nodes.length - index);
      node.style.transform = "translateY(" + Math.min(index * 1.5, 16) + "px)";
    });
    stack.style.zIndex = "12";

    await Promise.all([
      play(stack, [
        { transform: "translate(-50%, 5%) scale(.78)", opacity: 0.2 },
        { transform: "translate(-50%, -92%) scale(.97)", opacity: 1, offset: 0.58 },
        { transform: stackSettled + " scale(1)", opacity: 1 }
      ], { duration: 500, easing: "cubic-bezier(.18,.8,.2,1)", fill: "forwards" }),
      play(wrapper, [
        { transform: wrapperCentered + " scale(1)", opacity: 1 },
        { transform: "translate(-50%, -43%) scale(.96)", opacity: 0 }
      ], { duration: 210, delay: 265, easing: "cubic-bezier(.4,0,1,1)", fill: "forwards" })
    ]);
    if (sequence !== state.sequenceToken) return;

    stack.style.zIndex = "24";
    wrapper.style.visibility = "hidden";

    var flipped = await flipStackFaceUp(nodes, sequence);
    if (!flipped || sequence !== state.sequenceToken) return;
    var first = nodes[0];
    if (!first) {
      completePack();
      return;
    }
    first.classList.add("is-current");
    first.tabIndex = 0;
    setPhase("revealing");
    revealAllButton.hidden = false;
    showCurrentCard();
    first.focus({ preventScroll: true });
  }

  async function flipStackFaceUp(nodes, sequence) {
    var stackSettled = "translate(-50%, -58%)";
    await play(stack, [
      { transform: stackSettled + " scaleX(1)" },
      { transform: stackSettled + " scaleX(.035)" }
    ], { duration: 125, easing: "cubic-bezier(.4,0,1,1)", fill: "forwards" });
    if (sequence !== state.sequenceToken) return false;
    nodes.forEach(function (node) {
      node.classList.add("is-face-up");
    });
    await play(stack, [
      { transform: stackSettled + " scaleX(.035)" },
      { transform: stackSettled + " scaleX(1)" }
    ], { duration: 135, easing: "cubic-bezier(0,.72,.2,1)", fill: "forwards" });
    return sequence === state.sequenceToken;
  }

  function showCurrentCard() {
    var card = state.cards[state.currentIndex];
    if (!card) return;
    stageInstruction.textContent = "Reveal each pull one card at a time.";
    current.hidden = false;
    currentName.textContent = text(card.name) || "Unknown card";
    currentPrice.textContent = priceText(card.price_usd) + (finishName(card) ? " · " + finishName(card) : "");
    announce((text(card.name) || "Card") + ", " + priceText(card.price_usd) + ". Pull " + (state.currentIndex + 1) + " of " + state.cards.length + ".");
  }

  async function revealCurrentCard(direction, fromTransform) {
    if (state.phase !== "revealing") return;
    var sequence = state.sequenceToken;
    var index = state.currentIndex;
    var card = state.cards[index];
    var node = stack.querySelector('[data-card-index="' + index + '"]');
    if (!card || !node) return;
    setPhase("transitioning");
    node.classList.remove("is-current");
    node.tabIndex = -1;
    var x = direction < 0 ? "-132%" : "132%";
    await play(node, [
      { transform: fromTransform || "translate(0,0) rotate(0deg)", opacity: 1 },
      { transform: "translate(" + x + ",-3%) rotate(" + (direction < 0 ? -11 : 11) + "deg)", opacity: 0 }
    ], { duration: 185, easing: "cubic-bezier(.4,0,1,1)", fill: "forwards" });
    if (sequence !== state.sequenceToken) return;

    recordPull(index);
    node.hidden = true;
    state.currentIndex += 1;
    if (state.currentIndex >= state.cards.length) {
      completePack();
      return;
    }

    var next = stack.querySelector('[data-card-index="' + state.currentIndex + '"]');
    next.classList.add("is-current");
    next.tabIndex = 0;
    setPhase("revealing");
    showCurrentCard();
    next.focus({ preventScroll: true });
  }

  async function revealAllCards() {
    if (state.phase !== "revealing") return;
    var sequence = state.sequenceToken;
    var startIndex = state.currentIndex;
    var remaining = [];
    for (var index = startIndex; index < state.cards.length; index += 1) {
      var node = stack.querySelector('[data-card-index="' + index + '"]');
      if (node) remaining.push({ index: index, node: node });
    }
    if (!remaining.length) return;

    setPhase("fast-forwarding");
    revealAllButton.disabled = true;
    current.hidden = true;
    stageInstruction.textContent = "Revealing the rest of this pack…";
    announce("Revealing all remaining cards.");

    await Promise.all(remaining.map(function (item, offset) {
      item.node.classList.remove("is-current");
      item.node.tabIndex = -1;
      var direction = offset % 2 === 0 ? 1 : -1;
      var fromTransform = item.node.style.transform || "translate(0,0) rotate(0deg)";
      return play(item.node, [
        { transform: fromTransform, opacity: 1 },
        { transform: "translate(" + (direction * 118) + "%,-3%) rotate(" + (direction * 9) + "deg)", opacity: 0 }
      ], {
        duration: reducedMotion ? 0 : 150,
        delay: reducedMotion ? 0 : Math.min(offset * 22, 240),
        easing: "cubic-bezier(.4,0,1,1)",
        fill: "forwards"
      });
    }));
    if (sequence !== state.sequenceToken) return;

    remaining.forEach(function (item) {
      item.node.hidden = true;
      recordPull(item.index);
    });
    state.currentIndex = state.cards.length;
    completePack();
  }

  function recordPull(index) {
    var card = state.cards[index];
    if (!card || card.__packRecorded) return;
    card.__packRecorded = true;
    var cents = priceCents(card.price_usd);
    state.packPullCount += 1;
    state.packPullValueCents += cents;
    addPull(card, currentPullsGrid);
    currentPulls.hidden = false;
  }

  function addPull(card, destination) {
    var link = document.createElement("a");
    link.className = "mt-pack-pull mt-pack-pull--" + text(card.rarity).toLowerCase();
    link.dataset.finish = text(card.finish).toLowerCase();
    link.href = text(card.detail_path) || "/cards";

    var imageShell = document.createElement("span");
    imageShell.className = "mt-pack-pull__image";
    if (text(card.image_uri)) {
      var image = document.createElement("img");
      image.src = text(card.image_uri);
      image.alt = text(card.name) || "Pulled card";
      image.loading = "lazy";
      imageShell.appendChild(image);
    } else {
      var fallback = document.createElement("span");
      fallback.className = "mt-pack-card__fallback";
      fallback.textContent = text(card.name) || "Card image unavailable";
      imageShell.appendChild(fallback);
    }
    var name = document.createElement("p");
    name.className = "mt-pack-pull__name";
    name.textContent = text(card.name) || "Unknown card";
    var price = document.createElement("p");
    price.className = "mt-pack-pull__price";
    price.textContent = priceText(card.price_usd) + (finishName(card) ? " · " + finishName(card) : "");
    link.appendChild(imageShell);
    link.appendChild(name);
    link.appendChild(price);
    destination.appendChild(link);
  }

  function commitCurrentPack() {
    if (state.packCommitted || state.phase !== "complete" || state.packPullCount === 0) return false;
    state.packCommitted = true;
    state.sessionPullCount += state.packPullCount;
    state.sessionPullValueCents += state.packPullValueCents;
    while (currentPullsGrid.firstChild) {
      pullsGrid.appendChild(currentPullsGrid.firstChild);
    }
    updatePullSummary();
    return true;
  }

  function settleOpenedPack() {
    if (["revealing", "transitioning", "fast-forwarding", "complete"].indexOf(state.phase) < 0) return;
    for (var index = 0; index < state.cards.length; index += 1) {
      recordPull(index);
    }
    state.currentIndex = state.cards.length;
    if (state.packPullCount > 0) {
      setPhase("complete");
      commitCurrentPack();
    }
  }

  function updatePullSummary() {
    pullCount.textContent = String(state.sessionPullCount);
    totalValue.textContent = dollarsFromCents(state.sessionPullValueCents);
    pulls.hidden = state.sessionPullCount === 0;
  }

  function completePack() {
    current.hidden = true;
    var allCardsDismissed = state.cards.length > 0 && state.currentIndex >= state.cards.length;
    revealAllButton.hidden = true;
    revealAllButton.disabled = false;
    resetButton.hidden = !allCardsDismissed;
    if (!allCardsDismissed) {
      setPhase("idle");
      stageInstruction.textContent = "This pack could not be displayed.";
      announce(stageInstruction.textContent);
      return;
    }
    setPhase("complete");
    stage.hidden = true;
    currentPulls.hidden = false;
    currentPulls.classList.add("is-complete");
    stageInstruction.textContent = "Pack complete. Every card is collected below.";
    announce("Pack complete. " + state.cards.length + " cards pulled, totaling " + dollarsFromCents(state.packPullValueCents) + ".");
    currentPulls.focus({ preventScroll: true });
  }

  function resetOpening(clearSelection) {
    cancelChoreography();
    settleOpenedPack();
    closeSimulationDetails();
    clearSimulationDetails();
    state.requestToken += 1;
    state.cards = [];
    state.currentIndex = 0;
    state.packPullCount = 0;
    state.packPullValueCents = 0;
    state.packCommitted = false;
    state.sliding = null;
    state.sliderProgress = 0;
    state.swiping = null;
    state.suppressClickNode = null;
    window.clearTimeout(state.suppressClickTimer);
    state.suppressClickTimer = 0;
    stack.innerHTML = "";
    currentPullsGrid.innerHTML = "";
    opening.hidden = true;
    stage.hidden = false;
    scene.hidden = true;
    current.hidden = true;
    currentPulls.hidden = true;
    currentPulls.classList.remove("is-complete");
    updatePullSummary();
    revealAllButton.hidden = true;
    revealAllButton.disabled = false;
    resetButton.hidden = true;
    resetButton.disabled = false;
    loader.hidden = false;
    wrapper.classList.remove("is-opening", "is-unsealed");
    wrapper.style.removeProperty("opacity");
    wrapper.style.removeProperty("transform");
    wrapper.style.removeProperty("visibility");
    stack.style.removeProperty("opacity");
    stack.style.removeProperty("transform");
    stack.style.removeProperty("transition");
    stack.style.removeProperty("z-index");
    openSlider.style.removeProperty("opacity");
    openSlider.style.removeProperty("transform");
    openSliderFill.style.removeProperty("width");
    openSliderHandle.style.removeProperty("transform");
    updateOpenSlider(0);
    setPhase("idle");
    if (clearSelection) {
      state.set = null;
      state.packType = null;
      typeSection.hidden = true;
      accuracy.textContent = "";
      rail.querySelectorAll(".mt-pack-set").forEach(function (node) {
        node.setAttribute("aria-pressed", "false");
      });
    }
  }

  function resetForAnother() {
    var currentSet = state.set;
    var currentPackType = state.packType;
    if (!currentSet || !currentPackType || state.phase !== "complete") return;
    resetButton.disabled = true;
    commitCurrentPack();
    var typeButton = typeList.querySelector('[data-pack-type="' + packTypeID(currentPackType) + '"]');
    resetOpening(false);
    state.set = currentSet;
    state.packType = currentPackType;
    typeList.querySelectorAll(".mt-pack-type").forEach(function (node) {
      node.setAttribute("aria-pressed", node === typeButton ? "true" : "false");
      node.disabled = false;
    });
    updateAccuracy(currentPackType);
    announce("Preparing another " + packTypeName(currentPackType) + ".");
    loadPack(typeButton, { autoOpen: true, scroll: true });
  }

  function sliderTravel() {
    if (!openSlider || !openSliderHandle) return 1;
    return Math.max(1, openSlider.clientWidth - openSliderHandle.offsetWidth - 6);
  }

  function updateOpenSlider(progress) {
    var value = Math.max(0, Math.min(100, Number(progress) || 0));
    state.sliderProgress = value;
    openSlider.style.setProperty("--mt-pack-open-progress", value + "%");
    openSliderHandle.style.transform = "translateX(" + (sliderTravel() * value / 100) + "px)";
    openSlider.setAttribute("aria-valuenow", String(Math.round(value)));
    openSlider.setAttribute("aria-valuetext", value >= 96 ? "Opening" : Math.round(value) + " percent open");
  }

  function attachOpenSlider() {
    openSlider.addEventListener("pointerdown", function (event) {
      if (state.phase !== "sealed") return;
      event.preventDefault();
      state.sliding = { id: event.pointerId, startX: event.clientX, startProgress: state.sliderProgress };
      try { openSlider.setPointerCapture(event.pointerId); } catch (_) {}
    });
    openSlider.addEventListener("pointermove", function (event) {
      if (!state.sliding || state.sliding.id !== event.pointerId || state.phase !== "sealed") return;
      var progress = state.sliding.startProgress + ((event.clientX - state.sliding.startX) / sliderTravel()) * 100;
      updateOpenSlider(progress);
      if (state.sliderProgress >= 96) {
        state.sliding = null;
        updateOpenSlider(100);
        openPack();
      }
    });
    function endSlide(event) {
      if (!state.sliding || state.sliding.id !== event.pointerId) return;
      state.sliding = null;
      if (state.phase === "sealed") updateOpenSlider(0);
    }
    openSlider.addEventListener("pointerup", endSlide);
    openSlider.addEventListener("pointercancel", endSlide);
    openSlider.addEventListener("keydown", function (event) {
      if (state.phase !== "sealed") return;
      var next = state.sliderProgress;
      if (event.key === "ArrowRight" || event.key === "ArrowUp") next += 10;
      else if (event.key === "ArrowLeft" || event.key === "ArrowDown") next -= 10;
      else if (event.key === "Home") next = 0;
      else if (event.key === "End" || event.key === "Enter" || event.key === " ") next = 100;
      else return;
      event.preventDefault();
      updateOpenSlider(next);
      if (state.sliderProgress >= 96) openPack();
    });
  }

  function attachCardSwipe(node, index) {
    function suppressGestureClick() {
      state.suppressClickNode = node;
      window.clearTimeout(state.suppressClickTimer);
      state.suppressClickTimer = window.setTimeout(function () {
        if (state.suppressClickNode === node) state.suppressClickNode = null;
        state.suppressClickTimer = 0;
      }, 450);
    }

    function returnToStack(fromTransform) {
      play(node, [
        { transform: fromTransform },
        { transform: "translate(0,0) rotate(0deg)" }
      ], { duration: 95, easing: "cubic-bezier(.2,.75,.2,1)", fill: "forwards" });
    }

    node.addEventListener("pointerdown", function (event) {
      if (state.phase !== "revealing" || index !== state.currentIndex) return;
      state.swiping = { id: event.pointerId, node: node, startX: event.clientX, startY: event.clientY, moved: false };
      try { node.setPointerCapture(event.pointerId); } catch (_) {}
    });
    node.addEventListener("pointermove", function (event) {
      if (!state.swiping || state.swiping.id !== event.pointerId || state.swiping.node !== node) return;
      var dx = event.clientX - state.swiping.startX;
      var dy = event.clientY - state.swiping.startY;
      if (Math.abs(dx) + Math.abs(dy) > 5) state.swiping.moved = true;
      node.style.transform = "translate(" + dx + "px," + dy + "px) rotate(" + (dx / 20) + "deg)";
    });
    function endSwipe(event) {
      if (!state.swiping || state.swiping.id !== event.pointerId || state.swiping.node !== node) return;
      var dx = event.clientX - state.swiping.startX;
      var dy = event.clientY - state.swiping.startY;
      var moved = state.swiping.moved;
      var dragTransform = node.style.transform || "translate(0,0) rotate(0deg)";
      state.swiping = null;
      if (moved) {
        event.preventDefault();
        suppressGestureClick();
      }
      if (Math.abs(dx) >= 62 || Math.abs(dy) >= 78) {
        revealCurrentCard(dx < 0 ? -1 : 1, dragTransform);
      } else if (moved) {
        returnToStack(dragTransform);
      }
    }
    node.addEventListener("pointerup", endSwipe);
    node.addEventListener("pointercancel", function (event) {
      if (!state.swiping || state.swiping.id !== event.pointerId || state.swiping.node !== node) return;
      var moved = state.swiping.moved;
      var dragTransform = node.style.transform || "translate(0,0) rotate(0deg)";
      state.swiping = null;
      if (moved) suppressGestureClick();
      returnToStack(dragTransform);
    });
  }

  wrapperSymbolImage.addEventListener("error", function () {
    wrapperSymbolImage.hidden = true;
    wrapperSymbolFallback.hidden = false;
  });
  revealAllButton.addEventListener("click", revealAllCards);
  resetButton.addEventListener("click", resetForAnother);
  attachSimulationDetails();
  attachOpenSlider();
  renderSets();
})();
