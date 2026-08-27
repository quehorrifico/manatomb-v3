(function () {
  "use strict";

  function setupDeckWorkspaceViews() {
    var tabs = Array.prototype.slice.call(document.querySelectorAll("[data-deck-workspace-tab]"));
    var panels = Array.prototype.slice.call(document.querySelectorAll("[data-deck-workspace-panel]"));
    if (!tabs.length || !panels.length) return;

    function normalizedView(value) {
      switch (String(value || "").toLowerCase()) {
        case "details":
        case "analysis":
          return "analysis";
        default:
          return "decklist";
      }
    }

    function selectView(value, options) {
      var next = normalizedView(value);
      var focusTab = !!(options && options.focusTab);

      if (next !== "decklist" && typeof window.mtDeckSetFocusMode === "function") {
        window.mtDeckSetFocusMode(false);
      }

      tabs.forEach(function (tab) {
        var selected = tab.getAttribute("data-deck-workspace-tab") === next;
        tab.setAttribute("aria-selected", selected ? "true" : "false");
        tab.setAttribute("tabindex", selected ? "0" : "-1");
        if (selected && focusTab) tab.focus();
      });

      panels.forEach(function (panel) {
        var selected = panel.getAttribute("data-deck-workspace-panel") === next;
        panel.classList.toggle("hidden", !selected);
        panel.hidden = !selected;
      });

      if (next === "decklist") {
        window.requestAnimationFrame(function () {
          if (typeof window.mtDeckRefreshCardLayout === "function") {
            window.mtDeckRefreshCardLayout();
          }
        });
      }

      if (next === "analysis" && typeof window.mtDeckSyncManaCurveBars === "function") {
        window.mtDeckSyncManaCurveBars();
      }

      return next;
    }

    function requestedView() {
      try {
        return new URLSearchParams(window.location.search).get("view");
      } catch (error) {
        return "";
      }
    }

    tabs.forEach(function (tab, index) {
      tab.addEventListener("click", function () {
        selectView(tab.getAttribute("data-deck-workspace-tab"));
      });

      tab.addEventListener("keydown", function (event) {
        var nextIndex = index;
        if (event.key === "ArrowRight") nextIndex = (index + 1) % tabs.length;
        else if (event.key === "ArrowLeft") nextIndex = (index - 1 + tabs.length) % tabs.length;
        else if (event.key === "Home") nextIndex = 0;
        else if (event.key === "End") nextIndex = tabs.length - 1;
        else return;

        event.preventDefault();
        selectView(tabs[nextIndex].getAttribute("data-deck-workspace-tab"), { focusTab: true });
      });
    });

    selectView(requestedView());
    window.mtDeckOpenWorkspaceView = selectView;
  }

  function setupDeckImportWarningToast() {
    var toast = document.getElementById("deck-import-warning-toast");
    var message = document.getElementById("deck-import-warning-message");
    var dismiss = document.getElementById("deck-import-warning-dismiss");
    if (!toast || !message || !dismiss) return;

    var warningKey = "manatomb.deckImportWarning";
    var warnings = [];
    var hasQueryMarker = false;
    try {
      var query = new URLSearchParams(window.location.search);
      hasQueryMarker = query.get("import_warning") === "1";
      if (hasQueryMarker) {
        query.delete("import_warning");
        var cleanURL = window.location.pathname;
        var cleanQuery = query.toString();
        if (cleanQuery) cleanURL += "?" + cleanQuery;
        if (window.location.hash) cleanURL += window.location.hash;
        window.history.replaceState({}, "", cleanURL);
      }
    } catch (queryError) {
      hasQueryMarker = false;
    }

    try {
      var storedWarnings = sessionStorage.getItem(warningKey);
      sessionStorage.removeItem(warningKey);
      if (storedWarnings) {
        var parsedWarnings = JSON.parse(storedWarnings);
        if (Array.isArray(parsedWarnings)) {
          warnings = parsedWarnings.filter(function (warning) {
            return String(warning || "").trim() !== "";
          });
        }
      }
    } catch (storageError) {
      warnings = [];
    }

    if (!hasQueryMarker && warnings.length === 0) return;

    message.textContent = "Some cards couldn't be matched and were skipped. See Notes below.";
    toast.classList.remove("hidden");
    if (typeof window.mtDeckOpenWorkspaceView === "function") {
      window.mtDeckOpenWorkspaceView("analysis");
    }
    dismiss.addEventListener("click", function () {
      toast.classList.add("hidden");
    });
  }

  function setupDeckFocusMode() {
    var editor = document.querySelector(".mt-deck-editor");
    var toggle = document.getElementById("deck-focus-mode-toggle");
    var label = document.getElementById("deck-focus-mode-label");
    if (!editor || !toggle) return;
    toggle.hidden = false;

    function nestedEscapeTargetIsOpen(event) {
      var modal = document.getElementById("card-detail-modal");
      if (modal && !modal.classList.contains("hidden")) return true;

      var deckMenu = document.getElementById("deck-menu-overlay");
      if (deckMenu && deckMenu.getAttribute("aria-hidden") === "false") return true;

      if (document.querySelector('details[data-card-action-menu][open]')) return true;

      var addInput = document.getElementById("guest-card-name");
      if (addInput && addInput.getAttribute("aria-expanded") === "true") return true;

      var siteMenu = document.querySelector("[data-site-menu-toggle]");
      if (siteMenu && siteMenu.getAttribute("aria-expanded") === "true") return true;

      return !!(event.target && event.target.matches("select"));
    }

    function syncFocusedLayout() {
      window.requestAnimationFrame(function () {
        window.dispatchEvent(new Event("resize"));
      });
    }

    function setFocusMode(active) {
      var enabled = !!active;
      if (enabled && typeof window.mtDeckOpenWorkspaceView === "function") {
        window.mtDeckOpenWorkspaceView("decklist");
      }
      if (enabled && typeof window.mtDeckCloseActionMenus === "function") {
        window.mtDeckCloseActionMenus();
      }
      document.body.classList.toggle("mt-deck-focus-mode", enabled);
      editor.classList.toggle("mt-deck-editor--focus-mode", enabled);
      toggle.setAttribute("aria-pressed", enabled ? "true" : "false");
      toggle.setAttribute(
        "aria-label",
        enabled ? "Exit full screen editing mode" : "Enter full screen editing mode"
      );
      if (label) label.textContent = enabled ? "Exit full screen" : "Full screen";
      syncFocusedLayout();
    }

    toggle.addEventListener("click", function () {
      setFocusMode(!editor.classList.contains("mt-deck-editor--focus-mode"));
    });

    document.addEventListener("keydown", function (event) {
      if (event.key !== "Escape" || !editor.classList.contains("mt-deck-editor--focus-mode")) return;
      if (nestedEscapeTargetIsOpen(event)) return;
      event.preventDefault();
      setFocusMode(false);
    }, true);

    window.mtDeckSetFocusMode = setFocusMode;
  }

  function setupDeckBoardKeyboard() {
    var tabs = Array.prototype.slice.call(document.querySelectorAll("[data-deck-board-tab]"));
    if (!tabs.length) return;

    tabs.forEach(function (tab, index) {
      tab.addEventListener("keydown", function (event) {
        var nextIndex = index;
        if (event.key === "ArrowRight" || event.key === "ArrowDown") {
          nextIndex = (index + 1) % tabs.length;
        } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
          nextIndex = (index - 1 + tabs.length) % tabs.length;
        } else if (event.key === "Home") {
          nextIndex = 0;
        } else if (event.key === "End") {
          nextIndex = tabs.length - 1;
        } else {
          return;
        }

        event.preventDefault();
        tabs[nextIndex].focus();
        tabs[nextIndex].click();
      });
    });
  }

  function setupAddCardShortcut() {
    var input = document.getElementById("guest-card-name");
    if (!input) return;

    document.addEventListener("keydown", function (event) {
      if (event.key !== "/" || event.metaKey || event.ctrlKey || event.altKey) return;
      var target = event.target;
      if (target && (target.matches("input, textarea, select") || target.isContentEditable)) return;

      event.preventDefault();
      if (typeof window.mtDeckOpenWorkspaceView === "function") {
        window.mtDeckOpenWorkspaceView("decklist");
      }
      input.focus();
    });
  }

  function setupDeckEditor() {
    setupDeckWorkspaceViews();
    setupDeckImportWarningToast();
    setupDeckFocusMode();
    setupDeckBoardKeyboard();
    setupAddCardShortcut();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", setupDeckEditor);
  } else {
    setupDeckEditor();
  }
})();
