(function () {
  "use strict";

  function disableSubmitter(form, submitter) {
    var button = submitter || form.querySelector('button[type="submit"]');
    if (!button) return;
    button.disabled = true;
    button.setAttribute("aria-disabled", "true");
  }

  var guessForms = document.querySelectorAll("[data-guess-card-guess-form]");
  for (var i = 0; i < guessForms.length; i++) {
    guessForms[i].addEventListener("submit", function (event) {
      disableSubmitter(this, event.submitter);
    });
  }

  var revealForms = document.querySelectorAll("[data-guess-card-reveal-form]");
  for (var j = 0; j < revealForms.length; j++) {
    revealForms[j].addEventListener("submit", function (event) {
      if (!window.confirm("Reveal the card and end this round?")) {
        event.preventDefault();
        return;
      }
      disableSubmitter(this, event.submitter);
    });
  }

  var scrollStorageKey = "manatomb.guessCard.scroll.v1";
  var gameForms = document.querySelectorAll(".mt-guess-game form");

  if ("scrollRestoration" in window.history) {
    window.history.scrollRestoration = "manual";
  }

  function rememberGameScroll() {
    try {
      window.localStorage.setItem(scrollStorageKey, JSON.stringify({
        path: window.location.pathname,
        top: Math.max(0, window.scrollY || window.pageYOffset || 0)
      }));
    } catch (error) {
      // Preserving the viewport is a convenience; gameplay still works when
      // private browsing blocks session storage.
    }
  }

  for (var formIndex = 0; formIndex < gameForms.length; formIndex++) {
    gameForms[formIndex].addEventListener("submit", function (event) {
      if (!event.defaultPrevented) rememberGameScroll();
    });
  }

  try {
    var storedScroll = JSON.parse(window.localStorage.getItem(scrollStorageKey) || "null");
    window.localStorage.removeItem(scrollStorageKey);
    if (storedScroll && storedScroll.path === window.location.pathname && Number.isFinite(Number(storedScroll.top))) {
      window.requestAnimationFrame(function () {
        window.requestAnimationFrame(function () {
          var maximum = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
          window.scrollTo(0, Math.min(Math.max(0, Number(storedScroll.top)), maximum));
        });
      });
    }
  } catch (error) {
    try { window.localStorage.removeItem(scrollStorageKey); } catch (storageError) {}
  }

  var helpModal = document.querySelector("[data-guess-card-help-modal]");
  var helpOpeners = document.querySelectorAll("[data-guess-card-help-open]");
  var helpClose = helpModal && helpModal.querySelector("[data-guess-card-help-close]");
  var helpPanel = helpModal && helpModal.querySelector("[data-guess-card-help-panel]");
  var helpStorageKey = "manatomb.guessCard.howToPlaySeen.v1";
  var helpRestoreFocus = null;
  var previousBodyOverflow = "";
  var helpBackgroundState = [];

  function helpIsOpen() {
    return helpModal && !helpModal.hidden && !helpModal.classList.contains("hidden");
  }

  function helpFocusableElements() {
    if (!helpPanel) return [];
    return Array.prototype.slice.call(helpPanel.querySelectorAll(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter(function (element) {
      return !element.hidden && element.getAttribute("aria-hidden") !== "true";
    });
  }

  function setHelpBackgroundInert(inert) {
    if (inert) {
      helpBackgroundState = [];
      var elements = [
        document.querySelector(".mt-site-header"),
        document.querySelector(".mt-guess-game"),
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
      var state = helpBackgroundState[stateIndex];
      if (!state.inert) state.element.removeAttribute("inert");
      if (state.ariaHidden === null) state.element.removeAttribute("aria-hidden");
      else state.element.setAttribute("aria-hidden", state.ariaHidden);
    }
    helpBackgroundState = [];
  }

  function openHelpModal(trigger) {
    if (!helpModal || helpIsOpen()) return;
    helpRestoreFocus = trigger || null;
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    helpModal.hidden = false;
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
    helpModal.hidden = true;
    helpModal.classList.add("hidden");
    helpModal.setAttribute("aria-hidden", "true");
    document.body.style.overflow = previousBodyOverflow;
    setHelpBackgroundInert(false);
    rememberHelpDismissal();

    var focusTarget = helpRestoreFocus;
    if (!focusTarget || !document.contains(focusTarget)) {
      focusTarget = document.querySelector("[data-guess-question-id]") ||
        document.querySelector("[data-guess-card-help-open]");
    }
    helpRestoreFocus = null;
    if (focusTarget && typeof focusTarget.focus === "function") {
      focusTarget.focus({ preventScroll: true });
    }
  }

  for (var k = 0; k < helpOpeners.length; k++) {
    helpOpeners[k].addEventListener("click", function () {
      openHelpModal(this);
    });
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
        if (window.localStorage.getItem(helpStorageKey) !== "1") {
          openHelpModal(null);
        }
      } catch (error) {
        openHelpModal(null);
      }
    }
  }

  var questionCarousel = document.querySelector("[data-guess-question-carousel]");
  if (questionCarousel) {
    var questionViewport = questionCarousel.querySelector("[data-guess-question-viewport]");
    var questionPages = Array.prototype.slice.call(
      questionCarousel.querySelectorAll("[data-guess-question-page]")
    );
    var previousQuestions = questionCarousel.querySelector("[data-guess-question-previous]");
    var nextQuestions = questionCarousel.querySelector("[data-guess-question-next]");
    var questionPageLabel = questionCarousel.querySelector("[data-guess-question-page-label]");
    var questionPage = 0;
    var questionPageStorageKey = "manatomb.guessCard.questionPage.v1";

    function storedQuestionPage() {
      try {
        var stored = Number(window.localStorage.getItem(questionPageStorageKey));
        if (Number.isInteger(stored)) return stored;
      } catch (error) {
        // The category controls remain fully usable when storage is blocked.
      }
      return 0;
    }

    function rememberQuestionPage() {
      try {
        window.localStorage.setItem(questionPageStorageKey, String(questionPage));
      } catch (error) {
        // Remembering the selected category is a convenience, not a requirement.
      }
    }

    function updateQuestionPage(index, alignViewport) {
      if (!questionPages.length) return;
      questionPage = (index + questionPages.length) % questionPages.length;

      for (var pageIndex = 0; pageIndex < questionPages.length; pageIndex++) {
        var active = pageIndex === questionPage;
        questionPages[pageIndex].setAttribute("aria-hidden", active ? "false" : "true");
        if (active) questionPages[pageIndex].removeAttribute("inert");
        else questionPages[pageIndex].setAttribute("inert", "");
      }

      if (questionPageLabel) {
        questionPageLabel.textContent = questionPages[questionPage].getAttribute("data-page-label") || "Questions";
      }
      rememberQuestionPage();

      if (alignViewport && questionViewport) {
        questionViewport.scrollLeft = questionViewport.clientWidth * questionPage;
      }
    }

    if (previousQuestions) {
      previousQuestions.addEventListener("click", function () {
        updateQuestionPage(questionPage - 1, true);
      });
    }
    if (nextQuestions) {
      nextQuestions.addEventListener("click", function () {
        updateQuestionPage(questionPage + 1, true);
      });
    }
    if (questionViewport) {
      questionViewport.addEventListener("scroll", function () {
        if (!questionViewport.clientWidth) return;
        var nearestPage = Math.round(questionViewport.scrollLeft / questionViewport.clientWidth);
        if (nearestPage !== questionPage && nearestPage >= 0 && nearestPage < questionPages.length) {
          updateQuestionPage(nearestPage, false);
        }
      });
      window.addEventListener("resize", function () {
        questionViewport.scrollLeft = questionViewport.clientWidth * questionPage;
      });
    }

    updateQuestionPage(storedQuestionPage(), true);
  }

})();
