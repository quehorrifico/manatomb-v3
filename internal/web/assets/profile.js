(function () {
  "use strict";

  var shareButton = document.getElementById("profile-share");
  var actionStatus = document.getElementById("profile-action-status");
  var customizeModal = document.getElementById("profile-customize-modal");
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
      title: title ? title.textContent.trim() + " on Mana Tomb" : "Mana Tomb profile",
      text: "View this Mana Tomb profile",
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

  function openCustomizeModal(trigger) {
    if (!customizeModal) return;
    restoreFocus = trigger || document.activeElement;
    previousBodyOverflow = document.body.style.overflow;
    customizeModal.classList.remove("hidden");
    document.body.style.overflow = "hidden";
    var selected = customizeModal.querySelector(".mt-profile-avatar-choice--selected");
    var closeButton = customizeModal.querySelector("[data-profile-customize-close]");
    (selected || closeButton || customizeModal).focus({ preventScroll: true });
  }

  function closeCustomizeModal() {
    if (!customizeModal || customizeModal.classList.contains("hidden")) return;
    customizeModal.classList.add("hidden");
    document.body.style.overflow = previousBodyOverflow;
    if (restoreFocus && document.contains(restoreFocus)) restoreFocus.focus({ preventScroll: true });
    restoreFocus = null;
  }

  if (shareButton) shareButton.addEventListener("click", shareProfile);

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
      if (event.key === "Escape") closeCustomizeModal();
    });
  }

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
