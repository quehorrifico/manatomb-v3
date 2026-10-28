(function (global) {
  'use strict';

  function exportRows(deck) {
    var rows = [];
    var commander = String(deck.commanderName || '').trim();
    function metaFor(board, name) {
      return (deck.boardCardMeta && deck.boardCardMeta[board] && deck.boardCardMeta[board][name]) ||
        (deck.cardMeta && deck.cardMeta[name]) || {};
    }
    if (commander) rows.push({ section: 'Commander', name: commander, qty: 1, meta: (deck.cardMeta && deck.cardMeta[commander]) || metaFor('main', commander) });
    [['main', 'cards', 'Mainboard'], ['side', 'sideboardCards', 'Sideboard'], ['maybe', 'maybeCards', 'Maybeboard']].forEach(function (board) {
      Object.keys(deck[board[1]] || {}).sort(function (a, b) { return a.localeCompare(b); }).forEach(function (name) {
        var qty = Number(deck[board[1]][name]) || 0;
        if (board[0] === 'main' && name.toLowerCase() === commander.toLowerCase()) qty--;
        if (qty > 0) rows.push({ section: board[2], name: name, qty: qty, meta: metaFor(board[0], name) });
      });
    });
    return rows;
  }

  function csvCell(value) {
    var text = String(value == null ? '' : value);
    return /[",\r\n]/.test(text) ? '"' + text.replace(/"/g, '""') + '"' : text;
  }

  // Preview and full export use the same formatter; the limit counts card rows.
  function formatExport(deck, type, options, limit) {
    var rows = exportRows(deck);
    if (limit != null) rows = rows.slice(0, limit);
    if (!rows.length) return '';
    if (type === 'csv') {
      var columns = [];
      if (options.sections) columns.push(['Section', function (row) { return row.section; }]);
      if (options.quantity) columns.push(['Quantity', function (row) { return row.qty; }]);
      columns.push(['Name', function (row) { return row.name; }]);
      if (options.set) columns.push(['Set', function (row) { return String(row.meta.setCode || '').toUpperCase(); }]);
      if (options.collector) columns.push(['Collector Number', function (row) { return row.meta.collectorNumber || ''; }]);
      if (options.uuid) columns.push(['Scryfall UUID', function (row) { return row.meta.printID || row.meta.preferredPrintID || ''; }]);
      return [columns.map(function (col) { return csvCell(col[0]); }).join(',')].concat(rows.map(function (row) {
        return columns.map(function (col) { return csvCell(col[1](row)); }).join(',');
      })).join('\n') + '\n';
    }
    var lines = [], section = '';
    rows.forEach(function (row) {
      if (options.sections && section !== row.section) {
        if (lines.length) lines.push('');
        lines.push(row.section + ':');
        section = row.section;
      }
      var line = (options.quantity ? row.qty + ' ' : '') + row.name;
      if (options.set && row.meta.setCode) line += ' (' + String(row.meta.setCode).toUpperCase() + ')';
      if (options.collector && row.meta.collectorNumber) line += ' ' + row.meta.collectorNumber;
      var printID = row.meta.printID || row.meta.preferredPrintID;
      if (options.uuid && printID) line += ' {scryfall:' + printID + '}';
      lines.push(line);
    });
    return lines.join('\n') + '\n';
  }

  function applyCheapestPrintings(input, printings) {
    var deck = JSON.parse(JSON.stringify(input));
    deck.cardMeta = deck.cardMeta || {};
    deck.boardCardMeta = deck.boardCardMeta || {};
    var commander = deck.commanderName;
    var commanderMeta = commander && deck.cardMeta[commander];
    function replacement(name, current) {
      var next = printings[name.toLowerCase()];
      if (!next || (current.cardID && next.cardID && current.cardID !== next.cardID)) return null;
      var before = parseFloat(current.priceUSD || current.price || '');
      var after = parseFloat(next.priceUSD || next.price || '');
      if (!Number.isFinite(after) || after < 0 || (Number.isFinite(before) && before <= after)) return null;
      return Object.assign({}, current, next);
    }
    [['main', 'cards'], ['side', 'sideboardCards'], ['maybe', 'maybeCards']].forEach(function (entry) {
      var board = entry[0];
      deck.boardCardMeta[board] = deck.boardCardMeta[board] || {};
      Object.keys(deck[entry[1]] || {}).forEach(function (name) {
        var current = Object.assign({}, deck.cardMeta[name] || {}, deck.boardCardMeta[board][name] || {});
        var next = replacement(name, current);
        if (!next) return;
        deck.boardCardMeta[board][name] = next;
        if (board === 'main' || !deck.cardMeta[name]) deck.cardMeta[name] = next;
      });
    });
    if (commander) {
      var nextCommander = replacement(commander, commanderMeta || {});
      if (nextCommander) {
        deck.cardMeta[commander] = nextCommander;
        deck.commanderPrintID = nextCommander.printID;
      } else if (commanderMeta) deck.cardMeta[commander] = commanderMeta;
    }
    return deck;
  }

  function deckCost(deck) {
    var total = 0, unpriced = 0;
    exportRows(deck).filter(function (row) { return row.section !== 'Maybeboard'; }).forEach(function (row) {
      var price = parseFloat(row.meta.priceUSD || row.meta.price || '');
      if (Number.isFinite(price) && price >= 0) total += Math.round(price * 100) * row.qty;
      else unpriced += row.qty;
    });
    return { display: '$' + (total / 100).toFixed(2), unpriced: unpriced };
  }

  function fromBoards(boards, metadata) {
    var deck = { name: metadata.name, commanderName: metadata.commanderName, cards: {}, sideboardCards: {}, maybeCards: {}, cardMeta: {}, boardCardMeta: { main: {}, side: {}, maybe: {} } };
    [['main', 'cards'], ['side', 'sideboardCards'], ['maybe', 'maybeCards']].forEach(function (entry) {
      (boards[entry[0]] || []).forEach(function (card) {
        deck[entry[1]][card.name] = card.quantity;
        deck.boardCardMeta[entry[0]][card.name] = card;
        if (!deck.cardMeta[card.name]) deck.cardMeta[card.name] = card;
      });
    });
    if (metadata.commanderName && metadata.commander) deck.cardMeta[metadata.commanderName] = metadata.commander;
    return deck;
  }

  function init(config) {
    var doc = global.document;
    var exportDialog = doc.getElementById('deck-export-dialog');
    var bulkDialog = doc.getElementById('deck-bulk-dialog');
    if (!exportDialog || exportDialog.dataset.ready) return;
    exportDialog.dataset.ready = 'true';
    var type = doc.getElementById('deck-export-type');
    var preview = doc.getElementById('deck-export-preview');
    var exportStatus = doc.getElementById('deck-export-status');
    var exportSubmit = doc.getElementById('deck-export-submit');
    var bulkSubmit = doc.getElementById('deck-bulk-submit');
    var bulkText = doc.getElementById('deck-bulk-text');
    var bulkStatus = doc.getElementById('deck-bulk-status');
    function options() {
      var result = {};
      exportDialog.querySelectorAll('[data-export-option]').forEach(function (input) { result[input.dataset.exportOption] = input.checked; });
      return result;
    }
    function updatePreview() {
      var content = formatExport(config.getDeck(), type.value, options(), 3);
      preview.textContent = content || 'Add cards to your deck to preview an export.';
      exportSubmit.disabled = !content;
      exportSubmit.textContent = type.value === 'clipboard' ? 'Copy to clipboard' : type.value === 'csv' ? 'Download CSV' : 'Download text';
      exportStatus.textContent = '';
    }
    [exportDialog, bulkDialog].filter(Boolean).forEach(function (dialog) {
      dialog.addEventListener('click', function (event) { if (event.target === dialog && (!bulkSubmit || !bulkSubmit.disabled)) dialog.close(); });
    });
    doc.getElementById('deck-export-open').addEventListener('click', function () { updatePreview(); exportDialog.showModal(); });
    exportDialog.addEventListener('change', updatePreview);
    exportDialog.querySelectorAll('[data-export-select-all]').forEach(function (button) {
      button.addEventListener('click', function () {
        var checked = button.dataset.exportSelectAll === 'true';
        exportDialog.querySelectorAll('[data-export-option]').forEach(function (input) { input.checked = checked; });
        updatePreview();
      });
    });
    exportSubmit.addEventListener('click', async function () {
      var deck = config.getDeck();
      var content = formatExport(deck, type.value, options());
      exportSubmit.disabled = true;
      try {
        if (type.value === 'clipboard') {
          await global.navigator.clipboard.writeText(content);
          exportStatus.textContent = 'Deck copied to clipboard.';
        } else {
          var csv = type.value === 'csv';
          var url = URL.createObjectURL(new Blob([content], { type: csv ? 'text/csv;charset=utf-8' : 'text/plain;charset=utf-8' }));
          var link = doc.createElement('a');
          link.href = url;
          link.download = (String(deck.name || 'deck').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'deck') + (csv ? '.csv' : '.txt');
          doc.body.appendChild(link);
          link.click();
          link.remove();
          global.setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
          exportStatus.textContent = 'Deck exported.';
        }
      } catch (error) { exportStatus.textContent = 'Could not export: ' + error.message; }
      finally { exportSubmit.disabled = false; }
    });
    if (!bulkDialog) return;
    doc.getElementById('deck-bulk-open').addEventListener('click', function () {
      doc.getElementById('deck-bulk-board').value = config.getBoard();
      bulkDialog.showModal();
      bulkText.focus();
    });
    doc.getElementById('deck-bulk-file').addEventListener('change', async function (event) {
      var file = event.target.files[0];
      if (!file) return;
      if (file.size > 1024 * 1024) { bulkStatus.textContent = 'Choose a file smaller than 1 MB.'; return; }
      try { bulkText.value = await file.text(); bulkStatus.textContent = 'Loaded ' + file.name + '.'; }
      catch (error) { bulkStatus.textContent = 'Could not read that file.'; }
    });
    bulkSubmit.addEventListener('click', async function () {
      if (!bulkText.value.trim()) { bulkStatus.textContent = 'Paste a card list first.'; bulkText.focus(); return; }
      bulkSubmit.disabled = true;
      bulkDialog.querySelector('[aria-label="Close bulk add"]').disabled = true;
      bulkDialog.addEventListener('cancel', preventCancel);
      bulkStatus.textContent = 'Resolving and adding cards…';
      try {
        await config.importCards(bulkText.value, doc.getElementById('deck-bulk-board').value);
        bulkText.value = '';
        bulkStatus.textContent = 'Cards added. Your existing printing selections were kept.';
      } catch (error) { bulkStatus.textContent = error.message || 'Could not import cards. Please try again.'; }
      finally {
        bulkSubmit.disabled = false;
        bulkDialog.querySelector('[aria-label="Close bulk add"]').disabled = false;
        bulkDialog.removeEventListener('cancel', preventCancel);
      }
    });
    function preventCancel(event) { event.preventDefault(); }
  }

  global.ManatombDeckTransfer = { init: init, applyCheapestPrintings: applyCheapestPrintings, deckCost: deckCost, fromBoards: fromBoards, formatExport: formatExport, exportRows: exportRows };
  if (typeof module !== 'undefined' && module.exports) module.exports = global.ManatombDeckTransfer;
})(typeof window !== 'undefined' ? window : globalThis);
