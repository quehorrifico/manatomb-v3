(function () {
  "use strict";

  var shareButton = document.getElementById("profile-share");
  var actionStatus = document.getElementById("profile-action-status");
  var customizeModal = document.getElementById("profile-customize-modal");
  var profileArtSearchForm = document.querySelector("[data-profile-art-search]");
  var statusTimer = null;
  var restoreFocus = null;
  var previousBodyOverflow = "";

  function setStatus(message, isError) {
    if (!actionStatus) return;
    window.clearTimeout(statusTimer);
    actionStatus.textContent = message || "";
    actionStatus.classList.toggle("mt-profile-action-status--error", !!isError);
    if (message) {
      statusTimer = window.setTimeout(function () {
        actionStatus.textContent = "";
      }, 5000);
    }
  }

  function copyText(value) {
    if (navigator.clipboard && window.isSecureContext) {
      return navigator.clipboard.writeText(value);
    }
    return new Promise(function (resolve, reject) {
      var field = document.createElement("textarea");
      field.value = value;
      field.setAttribute("readonly", "");
      field.style.position = "fixed";
      field.style.opacity = "0";
      document.body.appendChild(field);
      field.select();
      try {
        if (!document.execCommand("copy")) throw new Error("Copy failed");
        resolve();
      } catch (error) {
        reject(error);
      } finally {
        field.remove();
      }
    });
  }

  function shareProfile() {
    var url = window.location.origin + window.location.pathname;
    var title = document.querySelector(".mt-profile-identity h1");
    var data = {
      title: title ? title.textContent.trim() + " on ManaTomb" : "ManaTomb profile",
      text: "View this ManaTomb profile",
      url: url
    };
    var result = navigator.share ? navigator.share(data) : copyText(url);
    Promise.resolve(result).then(function () {
      setStatus(navigator.share ? "Profile shared." : "Profile link copied.");
    }).catch(function (error) {
      if (error && error.name === "AbortError") return;
      setStatus("Could not share this profile.", true);
    });
  }

  function customizeModalIsOpen() {
    return !!customizeModal && !customizeModal.hidden && !customizeModal.classList.contains("hidden");
  }

  function customizeModalFocusables() {
    if (!customizeModal) return [];
    return Array.prototype.slice.call(customizeModal.querySelectorAll([
      "a[href]",
      "button:not([disabled])",
      "input:not([disabled])",
      "select:not([disabled])",
      "textarea:not([disabled])",
      "[tabindex]:not([tabindex='-1'])"
    ].join(","))).filter(function (element) {
      return !element.hidden && !element.closest("[hidden]") && element.getAttribute("aria-hidden") !== "true";
    });
  }

  function focusCustomizeModalStart() {
    if (!customizeModal) return;
    var selected = customizeModal.querySelector('[aria-pressed="true"]');
    var closeButton = customizeModal.querySelector("[data-profile-customize-close]");
    var panel = customizeModal.querySelector(".mt-profile-customize-panel");
    (selected || closeButton || panel || customizeModal).focus({ preventScroll: true });
  }

  function openCustomizeModal(trigger) {
    if (!customizeModal) return;
	// Keep the fixed modal at the viewport root. Leaving it inside the page
	// shell lets transformed or scrolling ancestors offset it in some browsers.
	if (customizeModal.parentElement !== document.body) {
	  document.body.appendChild(customizeModal);
	}
    restoreFocus = trigger || document.activeElement;
    previousBodyOverflow = document.body.style.overflow;
    customizeModal.hidden = false;
    customizeModal.classList.remove("hidden");
    customizeModal.setAttribute("aria-hidden", "false");
    document.body.style.overflow = "hidden";
    focusCustomizeModalStart();
  }

  function closeCustomizeModal() {
    if (!customizeModalIsOpen()) return;
    customizeModal.hidden = true;
    customizeModal.classList.add("hidden");
    customizeModal.setAttribute("aria-hidden", "true");
    document.body.style.overflow = previousBodyOverflow;
    if (restoreFocus && document.contains(restoreFocus)) restoreFocus.focus({ preventScroll: true });
    restoreFocus = null;
  }

  if (shareButton) shareButton.addEventListener("click", shareProfile);

  var profileArtTarget = "picture";
  var profileArtBusy = false;
  var profileArtStatus = document.querySelector("[data-profile-avatar-search-status]");
  var profileArtPrintings = document.querySelector("[data-profile-art-printings]");
  var profileArtRandom = document.querySelector("[data-profile-art-random]");
  var profileArtTargets = Array.prototype.slice.call(document.querySelectorAll("[data-profile-art-target]"));

  function setProfileArtStatus(message, isError) {
    if (!profileArtStatus) return;
    profileArtStatus.textContent = message || "";
    profileArtStatus.classList.toggle("mt-profile-avatar-search__status--error", !!isError);
  }

  function selectProfileArtTarget(target) {
    if (target !== "picture" && target !== "background") return;
    profileArtTarget = target;
    profileArtTargets.forEach(function (button) {
      button.setAttribute("aria-pressed", button.dataset.profileArtTarget === target ? "true" : "false");
    });
    setProfileArtStatus("Choose a card suggestion, then choose its printing.", false);
  }

  function profileArtImage(payload) {
    return String(payload && payload.image_uri || "").trim();
  }

  function replacePreviewImage(container, imageURI, alt) {
    if (!container || !imageURI) return;
    var image = container.querySelector("img");
    if (!image) {
      container.textContent = "";
      image = document.createElement("img");
      container.appendChild(image);
    }
    image.src = imageURI;
    image.alt = alt || "";
  }

  function applyProfileArt(payload) {
    var target = String(payload && payload.target || profileArtTarget);
    var imageURI = profileArtImage(payload);
    var name = String(payload && payload.name || "Card art");
    if (!imageURI) return;

    replacePreviewImage(document.querySelector('[data-profile-art-target-preview="' + target + '"]'), imageURI, "");
    if (target === "picture") {
      replacePreviewImage(document.querySelector(".mt-profile-avatar"), imageURI, name);
      var picture = document.querySelector(".mt-profile-avatar img");
      if (picture) picture.setAttribute("data-profile-picture-image", "");
    } else {
      var hero = document.querySelector(".mt-profile-hero");
      var background = document.querySelector("[data-profile-background-image]");
      if (!background && hero) {
        background = document.createElement("img");
        background.className = "mt-profile-hero__art";
        background.setAttribute("data-profile-background-image", "");
        background.setAttribute("aria-hidden", "true");
        background.alt = "";
        hero.insertBefore(background, hero.firstChild);
      }
      if (background) background.src = imageURI;
    }
    setProfileArtStatus(String(payload && payload.message || "Profile art updated."), false);
  }

  function submitProfileArt(values) {
    if (profileArtBusy) return Promise.reject(new Error("Profile art is already updating."));
    profileArtBusy = true;
    values.set("target", profileArtTarget);
    return window.fetch("/profile/art", {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Accept": "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8"
      },
      body: values.toString()
    }).then(function (response) {
      return response.json().catch(function () { return {}; }).then(function (payload) {
        if (!response.ok) {
          var error = new Error(String(payload && payload.error || "Could not update profile art."));
          error.redirectURL = String(payload && payload.redirect_url || "");
          throw error;
        }
        return payload;
      });
    }).then(function (payload) {
      applyProfileArt(payload);
      return payload;
    }).catch(function (error) {
      if (error && error.redirectURL) {
        window.location.assign(error.redirectURL);
        return;
      }
      setProfileArtStatus(String(error && error.message || "Could not update profile art."), true);
      throw error;
    }).finally(function () {
      profileArtBusy = false;
    });
  }

  function printingImage(version) {
    return String(version && (version.art_crop_uri || version.image_uri) || "").trim();
  }

  function renderProfileArtPrintings(versions) {
    if (!profileArtPrintings) return;
    profileArtPrintings.textContent = "";
    var available = Array.isArray(versions) ? versions.filter(function (version) {
      return String(version && version.scryfall_id || "").trim() && printingImage(version);
    }) : [];
    if (!available.length) {
      profileArtPrintings.hidden = true;
      setProfileArtStatus("No printings with artwork were found.", true);
      return;
    }

    available.forEach(function (version) {
      var button = document.createElement("button");
      var image = document.createElement("img");
      var copy = document.createElement("span");
      var name = document.createElement("strong");
      var meta = document.createElement("small");
      button.type = "button";
      button.className = "mt-profile-art-printing";
      image.src = printingImage(version);
      image.alt = "";
      name.textContent = String(version.name || "Card printing");
      meta.textContent = [
        String(version.set_name || version.set_code || "Unknown set"),
        version.collector_number ? "#" + String(version.collector_number) : ""
      ].filter(Boolean).join(" · ");
      copy.appendChild(name);
      copy.appendChild(meta);
      button.appendChild(image);
      button.appendChild(copy);
      button.addEventListener("click", function () {
        setProfileArtStatus("Applying " + name.textContent + "…", false);
        var values = new URLSearchParams();
        values.set("scryfall_id", String(version.scryfall_id));
        submitProfileArt(values).catch(function () {});
      });
      profileArtPrintings.appendChild(button);
    });
    profileArtPrintings.hidden = false;
    setProfileArtStatus("Choose one of " + String(available.length) + " printings.", false);
  }

  profileArtTargets.forEach(function (button) {
    button.addEventListener("click", function () {
      selectProfileArtTarget(String(button.dataset.profileArtTarget || ""));
    });
  });

  if (profileArtSearchForm) {
    var profileArtSearchInput = profileArtSearchForm.querySelector("[data-card-autocomplete-input]");
    profileArtSearchForm.addEventListener("submit", function (event) {
      event.preventDefault();
      setProfileArtStatus("Choose a card from the suggestions first.", true);
      if (profileArtSearchInput) profileArtSearchInput.focus();
    });
    profileArtSearchForm.addEventListener("card-autocomplete-selected", function (event) {
      var result = event.detail || {};
      var oracleID = String(result.oracle_id || "").trim();
      if (!oracleID) {
        setProfileArtStatus("Choose a valid card suggestion.", true);
        return;
      }
      setProfileArtStatus("Loading printings…", false);
      window.fetch("/cards/versions?oracle_id=" + encodeURIComponent(oracleID), {
        credentials: "same-origin",
        headers: { "Accept": "application/json" }
      }).then(function (response) {
        if (!response.ok) throw new Error("Could not load printings.");
        return response.json();
      }).then(function (payload) {
        renderProfileArtPrintings(payload && payload.versions);
      }).catch(function (error) {
        setProfileArtStatus(String(error && error.message || "Could not load printings."), true);
      });
    });
    if (profileArtSearchInput) {
      profileArtSearchInput.addEventListener("input", function () {
        if (profileArtPrintings) {
          profileArtPrintings.hidden = true;
          profileArtPrintings.textContent = "";
        }
        setProfileArtStatus("Choose a card suggestion, then choose its printing.", false);
      });
    }
  }

  if (profileArtRandom) {
    profileArtRandom.addEventListener("click", function () {
      setProfileArtStatus("Choosing random card art…", false);
      var values = new URLSearchParams();
      values.set("action", "random");
      submitProfileArt(values).catch(function () {});
    });
  }

  Array.prototype.slice.call(document.querySelectorAll("[data-profile-customize-open]")).forEach(function (button) {
    button.addEventListener("click", function () { openCustomizeModal(button); });
  });

  if (customizeModal) {
    customizeModal.querySelectorAll("[data-profile-customize-close]").forEach(function (button) {
      button.addEventListener("click", closeCustomizeModal);
    });
    customizeModal.addEventListener("click", function (event) {
      if (event.target === customizeModal) closeCustomizeModal();
    });
    document.addEventListener("keydown", function (event) {
      if (!customizeModalIsOpen()) return;
      if (event.key === "Escape") {
        event.preventDefault();
        closeCustomizeModal();
        return;
      }
      if (event.key !== "Tab") return;

      var focusables = customizeModalFocusables();
      if (!focusables.length) {
        event.preventDefault();
        focusCustomizeModalStart();
        return;
      }

      var first = focusables[0];
      var last = focusables[focusables.length - 1];
      if (event.shiftKey && (document.activeElement === first || !customizeModal.contains(document.activeElement))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
    document.addEventListener("focusin", function (event) {
      if (!customizeModalIsOpen() || customizeModal.contains(event.target)) return;
      focusCustomizeModalStart();
    });
  }

  var profileTabs = Array.prototype.slice.call(document.querySelectorAll("[data-profile-tab]"));
  var profilePanels = Array.prototype.slice.call(document.querySelectorAll("[data-profile-panel]"));

  function profileTabFromURL() {
    var value = new URL(window.location.href).searchParams.get("tab");
    return value === "favorites" || value === "achievements" ? value : "decks";
  }

  function showProfileTab(tab, nextURL) {
    if (tab !== "favorites" && tab !== "achievements") tab = "decks";
    var scrollTop = window.scrollY;
    profileTabs.forEach(function (link) {
      var active = link.dataset.profileTab === tab;
      link.classList.toggle("mt-profile-tab--active", active);
      if (active) {
        link.setAttribute("aria-current", "page");
      } else {
        link.removeAttribute("aria-current");
      }
    });
    profilePanels.forEach(function (panel) {
      panel.hidden = panel.dataset.profilePanel !== tab;
    });
    if (nextURL) window.history.pushState({ profileTab: tab }, "", nextURL);
    window.requestAnimationFrame(function () {
      var maximum = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
      window.scrollTo(0, Math.min(scrollTop, maximum));
    });
  }

  profileTabs.forEach(function (link) {
    link.addEventListener("click", function (event) {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      var next = new URL(link.href, window.location.href);
      showProfileTab(String(link.dataset.profileTab || "decks"), next.pathname + next.search);
    });
  });

  window.addEventListener("popstate", function () {
    showProfileTab(profileTabFromURL(), "");
  });

  var deckList = document.getElementById("profile-deck-list");
  var deckFilter = document.getElementById("profile-deck-filter");
  var deckSort = document.getElementById("profile-deck-sort");
  var deckSummary = document.getElementById("profile-deck-summary");
  var deckEmpty = document.getElementById("profile-deck-empty");
  var deckRows = deckList ? Array.prototype.slice.call(deckList.querySelectorAll("[data-profile-deck]")) : [];

  function deckPower(row) {
    var match = String(row.dataset.power || "").match(/^\s*(\d+)/);
    return match ? Number(match[1]) : -1;
  }

  function compareDecks(left, right, mode) {
    if (mode === "name") return left.dataset.name.localeCompare(right.dataset.name);
    if (mode === "format") {
      var formatResult = left.dataset.format.localeCompare(right.dataset.format);
      return formatResult || left.dataset.name.localeCompare(right.dataset.name);
    }
    if (mode === "power") {
      var powerResult = deckPower(right) - deckPower(left);
      return powerResult || left.dataset.name.localeCompare(right.dataset.name);
    }
    return Number(left.dataset.order || 0) - Number(right.dataset.order || 0);
  }

  function renderDecks() {
    if (!deckList) return;
    var query = deckFilter ? deckFilter.value.trim().toLowerCase() : "";
    var terms = query.split(/\s+/).filter(Boolean);
    var mode = deckSort ? deckSort.value : "recent";
    var visible = deckRows.filter(function (row) {
      var search = String(row.dataset.search || "").toLowerCase();
      return terms.every(function (term) { return search.indexOf(term) >= 0; });
    }).sort(function (left, right) { return compareDecks(left, right, mode); });

    deckRows.forEach(function (row) { row.hidden = true; });
    visible.forEach(function (row) {
      row.hidden = false;
      deckList.appendChild(row);
    });
    if (deckSummary) {
      deckSummary.textContent = String(visible.length) + (visible.length === 1 ? " deck" : " decks");
    }
    if (deckEmpty) deckEmpty.classList.toggle("hidden", visible.length !== 0);
  }

  if (deckFilter) deckFilter.addEventListener("input", renderDecks);
  if (deckSort) deckSort.addEventListener("change", renderDecks);
})();
