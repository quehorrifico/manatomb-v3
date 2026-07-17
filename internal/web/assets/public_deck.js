(function () {
  "use strict";

  var config = window.ManatombPublicDeckConfig || {};
  var core = window.ManatombDeckBrowser;
  var ui = window.ManatombCardUI;
  var output = document.getElementById("public-deck-output");
  if (!core || !ui || !output) return;

  var summary = document.getElementById("public-deck-summary");
  var filterField = document.getElementById("public-deck-filter");
  var groupField = document.getElementById("public-deck-group");
  var sortField = document.getElementById("public-deck-sort");
  var viewField = document.getElementById("public-deck-view");
  var actionMenu = document.getElementById("public-deck-action-menu");
  var actionStatus = document.getElementById("public-deck-action-status");
  var boardButtons = Array.prototype.slice.call(document.querySelectorAll("[data-board]"));
  var params = new URLSearchParams(window.location.search);
  var pointerStartX = null;
  var statusTimer = null;
  var rawBoards = config.boards || {};
  var boards = {
    main: (rawBoards.main || []).map(core.adaptPublicCard),
    side: (rawBoards.side || []).map(core.adaptPublicCard),
    maybe: (rawBoards.maybe || []).map(core.adaptPublicCard)
  };
  var state = {
    board: core.normalizeBoard(params.get("board")),
    view: core.normalizeView(params.get("view")),
    group: core.normalizeGroup(params.get("group"), "category"),
    sort: core.normalizeSort(params.get("sort")),
    filter: core.trim(params.get("q")),
    singleIndex: 0
  };

  function groupsForState() {
    return core.groupCards(boards[state.board], {
      filter: state.filter,
      group: state.group,
      sort: state.sort
    });
  }

  function filteredCards() {
    return core.filterCards(boards[state.board], state.filter);
  }

  function orderedCards() {
    return core.orderedCards(boards[state.board], {
      filter: state.filter,
      group: state.group,
      sort: state.sort
    });
  }

  function imageMarkup(card, eager) {
    if (!card.image) return '<span class="mt-art-card__fallback">' + ui.escapeHTML(card.name) + "</span>";
    return '<img class="mt-deck-art-card__img" src="' + ui.escapeHTML(card.image) + '" alt="' + ui.escapeHTML(card.name) +
      '" loading="' + (eager ? "eager" : "lazy") + '" decoding="async" draggable="false">';
  }

  function sectionHead(group) {
    if (!group.name) return "";
    return '<div class="mt-deck-view-section__head"><p class="mt-deck-view-section__title">' +
      ui.escapeHTML(group.name) + '</p><p class="mt-deck-view-section__count">' +
      String(core.quantityTotal(group.cards)) + "</p></div>";
  }

  function renderText(groups) {
    return groups.map(function (group) {
      var rows = group.cards.map(function (card) {
        return '<article class="mt-list-row"><div class="min-w-0 flex-1"><button type="button" data-card-id="' +
          ui.escapeHTML(card.id) + '" class="mt-public-card-button mt-list-row__title text-left">' +
          String(card.quantity) + "x " + ui.escapeHTML(card.name) + '</button></div><div class="mt-list-row__actions">' +
          '<span class="inline-flex shrink-0 items-center">' + ui.renderManaSymbols(card.manaCost) + "</span>" +
          '<span class="text-xs text-slate-500">' + ui.escapeHTML(core.displayPrice(card)) + "</span></div></article>";
      }).join("");
      return '<section class="mt-deck-view-section">' + sectionHead(group) +
        '<div class="mt-deck-view-section__body mt-deck-view-section__body--text">' + rows + "</div></section>";
    }).join("");
  }

  function artCardMarkup(card, view, stackIndex) {
    var stackAttrs = view === "stacks" ? ' data-stack-index="' + String(stackIndex) + '" style="z-index:' + String(stackIndex + 1) + '"' : "";
    return '<article class="mt-art-card mt-deck-art-card ' + (view === "grid" ? "mt-art-card--grid" : "mt-art-card--stack") + '"' + stackAttrs + ">" +
      '<button type="button" data-card-id="' + ui.escapeHTML(card.id) + '" class="mt-public-card-button mt-deck-art-card__media block w-full border-0 bg-transparent p-0 text-left">' +
        imageMarkup(card, false) + "</button>" +
      '<span class="mt-art-card__badge">' + String(card.quantity) + "x</span>" +
      '<div class="mt-deck-art-card__body"><button type="button" data-card-id="' + ui.escapeHTML(card.id) +
        '" class="mt-public-card-button mt-art-card__title block w-full text-left">' + ui.escapeHTML(card.name) + "</button>" +
        '<p class="mt-deck-art-card__meta">' + ui.escapeHTML(core.displayManaSummary(card)) + " &middot; " + ui.escapeHTML(core.displayPrice(card)) + "</p></div>" +
      "</article>";
  }

  function renderArt(groups, view) {
    return groups.map(function (group) {
      var cards = group.cards.map(function (card, index) { return artCardMarkup(card, view, index); }).join("");
      return '<section class="mt-deck-view-section">' + sectionHead(group) +
        '<div class="mt-deck-view-section__body mt-deck-view-section__body--' + view + '">' + cards + "</div></section>";
    }).join("");
  }

  function renderTable(groups) {
    var grouped = state.group !== "none";
    var rows = groups.map(function (group) {
      var groupRow = grouped ? '<tr class="mt-deck-table__group-row"><td colspan="6">' +
        ui.escapeHTML(group.name) + " &middot; " + String(core.quantityTotal(group.cards)) + "</td></tr>" : "";
      return groupRow + group.cards.map(function (card) {
        return '<tr><td class="mt-deck-table__qty">' + String(card.quantity) + '</td><td><button type="button" data-card-id="' +
          ui.escapeHTML(card.id) + '" class="mt-public-card-button mt-deck-table__name text-left">' + ui.escapeHTML(card.name) + "</button></td>" +
          '<td><span class="inline-flex items-center">' + ui.renderManaSymbols(card.manaCost) + "</span></td>" +
          '<td class="text-slate-400">' + ui.escapeHTML(core.displayManaValue(card)) + "</td>" +
          '<td class="text-slate-400">' + ui.escapeHTML(card.typeLine || "Unknown") + "</td>" +
          '<td class="text-slate-400">' + ui.escapeHTML(core.displayPrice(card)) + "</td></tr>";
      }).join("");
    }).join("");
    return '<section class="mt-deck-view-section mt-deck-view-section--table"><div class="mt-deck-view-section__body mt-deck-view-section__body--table">' +
      '<table class="mt-deck-table"><thead><tr><th>Qty</th><th>Card</th><th>Cost</th><th>MV</th><th>Type</th><th>Price</th></tr></thead><tbody>' +
      rows + "</tbody></table></div></section>";
  }

  function singlePeekMarkup(card, direction) {
    var attribute = direction === "previous" ? "data-single-prev" : "data-single-next";
    if (!card) return '<button type="button" class="mt-public-single-peek" ' + attribute + ' disabled aria-hidden="true"></button>';
    var media = card.image
      ? '<img src="' + ui.escapeHTML(card.image) + '" alt="" loading="lazy" decoding="async" draggable="false">'
      : '<span class="mt-public-single-fallback">' + ui.escapeHTML(card.name) + "</span>";
    return '<button type="button" class="mt-public-single-peek" ' + attribute + ' aria-label="Show ' +
      ui.escapeHTML(direction) + " card, " + ui.escapeHTML(card.name) + '">' + media + "</button>";
  }

  function renderSingle(cards) {
    if (state.singleIndex >= cards.length) state.singleIndex = Math.max(0, cards.length - 1);
    var card = cards[state.singleIndex];
    if (!card) return '<div class="mt-public-empty">No cards match these controls.</div>';
    var previousCard = state.singleIndex > 0 ? cards[state.singleIndex - 1] : null;
    var nextCard = state.singleIndex < cards.length - 1 ? cards[state.singleIndex + 1] : null;
    var media = card.image
      ? '<img src="' + ui.escapeHTML(card.image) + '" alt="' + ui.escapeHTML(card.name) + '" loading="eager" decoding="async" draggable="false">'
      : '<span class="mt-public-single-fallback">' + ui.escapeHTML(card.name) + "</span>";
    return '<div class="mt-public-single" data-single-surface><div class="mt-public-single-stage">' +
      singlePeekMarkup(previousCard, "previous") + '<div class="mt-public-single-current"><div class="mt-public-single-image">' + media + "</div>" +
      '<div class="mt-public-single-copy"><h3 class="mt-public-single-title">' + ui.escapeHTML(card.name) + "</h3>" +
      '<div class="mt-public-single-meta"><span>' + ui.escapeHTML(core.setLabel(card)) + '<\/span><span class="mt-public-single-price">' +
      ui.escapeHTML(core.displayPrice(card)) + "</span></div></div></div>" + singlePeekMarkup(nextCard, "next") + "</div>" +
      '<div class="mt-public-single-controls"><button type="button" class="mt-public-single-nav" data-single-prev ' +
      (state.singleIndex === 0 ? "disabled" : "") + '>Previous</button><p class="mt-public-single-status" aria-live="polite">' +
      String(state.singleIndex + 1) + " of " + String(cards.length) + '</p><button type="button" class="mt-public-single-nav" data-single-next ' +
      (state.singleIndex === cards.length - 1 ? "disabled" : "") + ">Next</button></div></div>";
  }

  function layoutStack(container, expandedIndex) {
    var cards = Array.prototype.slice.call(container.querySelectorAll(".mt-deck-art-card"));
    if (!cards.length) return;
    var active = Number(expandedIndex);
    if (!Number.isFinite(active) || active < 0 || active >= cards.length) active = -1;
    var step = 40;
    var cardHeight = cards[0].offsetHeight || 274;
    var reveal = Math.min(Math.max(cardHeight + 8, 260), 322);
    var extra = active >= 0 ? Math.max(reveal - step, 0) : 0;
    cards.forEach(function (card, index) {
      card.style.top = String((index * step) + (active >= 0 && index > active ? extra : 0)) + "px";
      card.style.zIndex = String(index === active ? cards.length + 5 : index + 1);
      card.classList.toggle("mt-deck-art-card--stack-active", index === active);
    });
    container.style.height = String(((cards.length - 1) * step) + cardHeight + extra + 2) + "px";
  }

  function bindStacks() {
    Array.prototype.slice.call(output.querySelectorAll(".mt-deck-view-section__body--stacks")).forEach(function (container) {
      var cards = Array.prototype.slice.call(container.querySelectorAll(".mt-deck-art-card"));
      cards.forEach(function (card, index) {
        card.addEventListener("mouseenter", function () { layoutStack(container, index); });
        card.addEventListener("focusin", function () { layoutStack(container, index); });
      });
      container.addEventListener("mouseleave", function () { layoutStack(container, -1); });
      container.addEventListener("focusout", function (event) {
        if (!container.contains(event.relatedTarget)) layoutStack(container, -1);
      });
      layoutStack(container, -1);
    });
  }

  function updateURL() {
    var url = new URL(window.location.href);
    url.searchParams.set("board", state.board);
    url.searchParams.set("view", state.view);
    url.searchParams.set("group", state.group);
    url.searchParams.set("sort", state.sort);
    if (state.filter) url.searchParams.set("q", state.filter);
    else url.searchParams.delete("q");
    window.history.replaceState({}, "", url);
  }

  function syncControls() {
    boardButtons.forEach(function (button) {
      var board = button.dataset.board;
      var selected = board === state.board;
      button.classList.toggle("mt-board-tab--active", selected);
      button.setAttribute("aria-selected", selected ? "true" : "false");
      var count = button.querySelector("[data-board-count]");
      if (count) count.textContent = String(core.quantityTotal(boards[board]));
    });
    filterField.value = state.filter;
    groupField.value = state.group;
    sortField.value = state.sort;
    viewField.value = state.view;
  }

  function render() {
    var allCards = boards[state.board];
    var cards = filteredCards();
    var groups = groupsForState();
    var total = core.quantityTotal(cards);
    var allTotal = core.quantityTotal(allCards);
    summary.textContent = state.filter
      ? String(total) + " of " + String(allTotal) + " cards"
      : String(total) + (total === 1 ? " card" : " cards") + " in " + String(groups.length) + (groups.length === 1 ? " group" : " groups");

    if (!cards.length) {
      output.className = "mt-public-deck-output";
      output.innerHTML = '<div class="mt-public-empty">No cards match these controls.</div>';
    } else if (state.view === "single") {
      output.className = "mt-public-deck-output mt-public-single-root";
      output.innerHTML = renderSingle(orderedCards());
    } else {
      var grouped = state.group !== "none" && state.view !== "table";
      output.className = "mt-public-deck-output mt-deck-view-root mt-deck-view-root--" + state.view + (grouped ? " mt-deck-view-root--grouped" : "");
      if (state.view === "text") output.innerHTML = renderText(groups);
      else if (state.view === "grid") output.innerHTML = renderArt(groups, "grid");
      else if (state.view === "table") output.innerHTML = renderTable(groups);
      else output.innerHTML = renderArt(groups, "stacks");
      if (state.view === "stacks") window.requestAnimationFrame(bindStacks);
    }
    syncControls();
    updateURL();
  }

  function currentCardID() {
    var cards = orderedCards();
    return cards[state.singleIndex] ? cards[state.singleIndex].id : "";
  }

  function preserveSingleIndex(cardID) {
    var cards = orderedCards();
    var next = cards.findIndex(function (card) { return card.id === cardID; });
    state.singleIndex = next >= 0 ? next : 0;
  }

  function openCardDetail(cardID) {
    var card = boards[state.board].find(function (item) { return item.id === cardID; });
    if (!card || !window.mtCardDetailModal || typeof window.mtCardDetailModal.open !== "function") return;
    window.mtCardDetailModal.open({
      oracleID: card.id,
      scryfallID: card.printID,
      printID: card.printID,
      preferredPrintID: card.preferredPrintID || card.printID,
      detailPath: "/cards/view/" + encodeURIComponent(card.id),
      name: card.name,
      manaCost: card.manaCost,
      typeLine: card.typeLine,
      oracleText: card.oracleText,
      imageURI: card.image,
      priceUSD: card.price,
      artist: card.artist,
      setCode: card.setCode,
      setName: card.setName,
      collectorNumber: card.collectorNumber,
      rarity: card.rarity,
      releasedAt: card.releasedAt
    });
  }

  function moveSingle(delta) {
    var cards = orderedCards();
    var next = state.singleIndex + delta;
    if (next < 0 || next >= cards.length) return;
    state.singleIndex = next;
    render();
  }

  function setActionStatus(message, isError) {
    if (!actionStatus) return;
    window.clearTimeout(statusTimer);
    actionStatus.textContent = message || "";
    actionStatus.classList.toggle("mt-public-action-status--error", !!isError);
    if (message) statusTimer = window.setTimeout(function () { actionStatus.textContent = ""; }, 5000);
  }

  function copyText(value) {
    if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(value);
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

  function closeActionMenu() {
    if (actionMenu) actionMenu.open = false;
  }

  function shareDeck() {
    var data = { title: config.name || "Public deck", text: "View " + (config.name || "this deck"), url: window.location.href };
    var result = navigator.share ? navigator.share(data) : copyText(data.url);
    Promise.resolve(result).then(function () {
      setActionStatus(navigator.share ? "Share options opened." : "Deck link copied.");
    }).catch(function (error) {
      if (error && error.name === "AbortError") return;
      setActionStatus("Could not share this deck.", true);
    });
    closeActionMenu();
  }

  function copyDeckList() {
    copyText(core.textExport(boards, config)).then(function () {
      setActionStatus("Deck list copied.");
    }).catch(function () {
      setActionStatus("Could not copy the deck list.", true);
    });
    closeActionMenu();
  }

  function download(content, extension, contentType) {
    var blob = new Blob([content], { type: contentType });
    var url = URL.createObjectURL(blob);
    var link = document.createElement("a");
    link.href = url;
    link.download = core.fileBaseName(config.name) + extension;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    setActionStatus(extension.toUpperCase().slice(1) + " export downloaded.");
    closeActionMenu();
  }

  boardButtons.forEach(function (button) {
    button.addEventListener("click", function () {
      state.board = core.normalizeBoard(button.dataset.board);
      state.singleIndex = 0;
      render();
    });
  });

  filterField.addEventListener("input", function () {
    var cardID = currentCardID();
    state.filter = core.trim(filterField.value);
    preserveSingleIndex(cardID);
    render();
  });

  groupField.addEventListener("change", function () {
    var cardID = currentCardID();
    state.group = core.normalizeGroup(groupField.value, "category");
    preserveSingleIndex(cardID);
    render();
  });

  sortField.addEventListener("change", function () {
    var cardID = currentCardID();
    state.sort = core.normalizeSort(sortField.value);
    preserveSingleIndex(cardID);
    render();
  });

  viewField.addEventListener("change", function () {
    state.view = core.normalizeView(viewField.value);
    render();
  });

  output.addEventListener("click", function (event) {
    var cardButton = event.target.closest("[data-card-id]");
    if (cardButton && state.view !== "single") return openCardDetail(cardButton.dataset.cardId);
    if (event.target.closest("[data-single-prev]")) moveSingle(-1);
    if (event.target.closest("[data-single-next]")) moveSingle(1);
  });

  output.addEventListener("pointerdown", function (event) {
    if (!event.target.closest("[data-single-surface]") || event.target.closest("button")) return;
    pointerStartX = event.clientX;
  });

  output.addEventListener("pointerup", function (event) {
    if (pointerStartX === null) return;
    var delta = event.clientX - pointerStartX;
    pointerStartX = null;
    if (Math.abs(delta) >= 45) moveSingle(delta < 0 ? 1 : -1);
  });

  output.addEventListener("pointercancel", function () { pointerStartX = null; });

  document.addEventListener("keydown", function (event) {
    if (state.view !== "single" || event.target.matches("input, select, textarea")) return;
    if (event.key === "ArrowLeft") moveSingle(-1);
    if (event.key === "ArrowRight") moveSingle(1);
  });

  document.getElementById("public-deck-share").addEventListener("click", shareDeck);
  document.getElementById("public-deck-copy-list").addEventListener("click", copyDeckList);
  document.getElementById("public-deck-export-txt").addEventListener("click", function () {
    download(core.textExport(boards, config), ".txt", "text/plain;charset=utf-8");
  });
  document.getElementById("public-deck-export-csv").addEventListener("click", function () {
    download(core.csvExport(boards), ".csv", "text/csv;charset=utf-8");
  });

  render();
})();
