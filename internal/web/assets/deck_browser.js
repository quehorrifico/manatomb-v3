(function (global) {
  "use strict";

  var CATEGORY_DEFINITIONS = [
    { key: "ramp", label: "Ramp" },
    { key: "draw", label: "Draw" },
    { key: "tutor", label: "Tutors" },
    { key: "removal", label: "Removal" },
    { key: "board_wipe", label: "Board Wipes" },
    { key: "counterspell", label: "Countermagic" },
    { key: "protection", label: "Protection" },
    { key: "blink", label: "Blink" },
    { key: "recursion", label: "Recursion" },
    { key: "tokens", label: "Tokens" },
    { key: "sacrifice", label: "Sacrifice" },
    { key: "graveyard", label: "Graveyard" },
    { key: "stax", label: "Stax" },
    { key: "combo", label: "Combo" },
    { key: "threat", label: "Threats" },
    { key: "utility", label: "Utility" },
    { key: "lands", label: "Lands" }
  ];
  var CATEGORY_ORDER = CATEGORY_DEFINITIONS.map(function (item) { return item.label; });
  var CATEGORY_PRIORITY = ["lands"].concat(CATEGORY_DEFINITIONS.map(function (item) {
    return item.key;
  }).filter(function (key) {
    return key !== "lands";
  }));
  var CATEGORY_LABELS = {};
  CATEGORY_DEFINITIONS.forEach(function (item) { CATEGORY_LABELS[item.key] = item.label; });
  var TYPE_ORDER = ["Creatures", "Artifacts", "Enchantments", "Planeswalkers", "Battles", "Instants", "Sorceries", "Other", "Lands"];
  var TYPE_SORT_ORDER = TYPE_ORDER.slice();
  var MANA_VALUE_ORDER = ["0", "1", "2", "3", "4", "5", "6", "7+", "Unknown", "Lands"];
  var PRICE_ORDER = ["Free", "Under $1", "$1-$4.99", "$5-$9.99", "$10+", "Unknown"];
  var COMBO_NAMES = [
    "aetherflux reservoir", "basalt monolith", "brain freeze", "demonic consultation",
    "devoted druid", "dramatic reversal", "dualcaster mage", "food chain",
    "goblin bombardment", "heliod, sun-crowned", "isochron scepter", "karmic guide",
    "kiki-jiki, mirror breaker", "lion's eye diamond", "mikaeus, the unhallowed",
    "power artifact", "reveillark", "rings of brighthearth", "sensei's divining top",
    "splinter twin", "tainted pact", "thassa's oracle", "triskelion", "twinflame",
    "underworld breach", "vizier of remedies", "walking ballista", "zealous conscripts"
  ];

  function trim(value) {
    return String(value == null ? "" : value).replace(/\s+/g, " ").trim();
  }

  function numberValue(value, fallback) {
    var parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : (fallback == null ? 0 : fallback);
  }

  function normalizeBoard(value) {
    value = String(value || "").toLowerCase();
    return ["main", "side", "maybe"].indexOf(value) >= 0 ? value : "main";
  }

  function normalizeView(value, allowSingle) {
    value = String(value || "").toLowerCase();
    if (value === "focus") value = "single";
    var modes = ["stacks", "text", "grid", "table"];
    if (allowSingle !== false) modes.push("single");
    return modes.indexOf(value) >= 0 ? value : "stacks";
  }

  function normalizeGroup(value, fallback) {
    value = String(value || "").toLowerCase();
    if (value === "categories") value = "category";
    if (value === "mana") value = "mv";
    var modes = ["none", "category", "alphabet", "type", "mv", "price"];
    return modes.indexOf(value) >= 0 ? value : (fallback || "none");
  }

  function normalizeSort(value) {
    value = String(value || "").toLowerCase();
    if (value === "name") value = "alphabet";
    if (value === "mana-asc" || value === "mana-desc") value = "mv";
    return ["alphabet", "mv", "type", "price"].indexOf(value) >= 0 ? value : "alphabet";
  }

  function normalizeSortDirection(value) {
    return String(value || "").toLowerCase() === "desc" ? "desc" : "asc";
  }

  function normalizeCard(input, index) {
    input = input || {};
    var quantity = numberValue(
      input.quantity != null ? input.quantity : (input.qty != null ? input.qty : input.Quantity),
      1
    );
    var card = {
      id: trim(input.id || input.oracleID || input.oracle_id || input.CardID) || ("card-" + String(index || 0)),
      name: trim(input.name || input.cardName || input.CardName) || "Unknown card",
      manaCost: trim(input.manaCost || input.mana_cost || input.ManaCost),
      image: trim(input.image || input.imageURI || input.image_uri || input.ImageURI),
      typeLine: trim(input.typeLine || input.type_line || input.TypeLine),
      oracleText: trim(input.oracleText || input.oracle_text || input.OracleText),
      cmc: numberValue(input.cmc != null ? input.cmc : input.CMC, 0),
      price: trim(input.price || input.priceUSD || input.price_usd || input.PriceUSD),
      quantity: Math.max(0, quantity),
      preferredPrintID: trim(input.preferredPrintID || input.preferred_print_id || input.PreferredPrintID),
      printID: trim(input.printID || input.print_id || input.scryfallID || input.ScryfallID || input.PrintID),
      setCode: trim(input.setCode || input.set_code || input.SetCode).toUpperCase(),
      setName: trim(input.setName || input.set_name || input.SetName),
      collectorNumber: trim(input.collectorNumber || input.collector_number || input.CollectorNumber),
      rarity: trim(input.rarity || input.Rarity),
      releasedAt: trim(input.releasedAt || input.released_at || input.ReleasedAt),
      artist: trim(input.artist || input.Artist),
      source: input.source || null
    };
    if (card.quantity <= 0) card.quantity = 1;
    return card;
  }

  function adaptPublicCard(card, index) {
    return normalizeCard(card, index);
  }

  function adaptEditorRow(row, index) {
    row = row || {};
    var meta = row.cardMeta || {};
    return normalizeCard({
      id: meta.oracleID || meta.oracleId || meta.id,
      name: row.cardName || meta.name,
      manaCost: meta.manaCost,
      image: meta.imageURI,
      typeLine: meta.typeLine,
      oracleText: meta.oracleText,
      cmc: meta.cmc,
      price: meta.priceUSD,
      quantity: row.qty,
      preferredPrintID: meta.preferredPrintID,
      printID: meta.printID || meta.scryfallID,
      setCode: meta.setCode,
      setName: meta.setName,
      collectorNumber: meta.collectorNumber,
      rarity: meta.rarity,
      releasedAt: meta.releasedAt,
      artist: meta.artist,
      source: row
    }, index);
  }

  function quantityTotal(cards) {
    return (Array.isArray(cards) ? cards : []).reduce(function (total, card) {
      return total + numberValue(card && card.quantity, 0);
    }, 0);
  }

  function primaryType(typeLine) {
    var text = trim(typeLine).toLowerCase();
    if (text.indexOf("land") >= 0) return "Lands";
    if (text.indexOf("creature") >= 0) return "Creatures";
    if (text.indexOf("artifact") >= 0) return "Artifacts";
    if (text.indexOf("enchantment") >= 0) return "Enchantments";
    if (text.indexOf("planeswalker") >= 0) return "Planeswalkers";
    if (text.indexOf("battle") >= 0) return "Battles";
    if (text.indexOf("instant") >= 0) return "Instants";
    if (text.indexOf("sorcery") >= 0) return "Sorceries";
    return "Other";
  }

  function manaValueGroup(card) {
    card = card || {};
    if (primaryType(card.typeLine) === "Lands") return "Lands";
    var value = numberValue(card.cmc, NaN);
    if (!Number.isFinite(value)) return "Unknown";
    if (value >= 7) return "7+";
    if (value <= 0) return "0";
    return String(Math.floor(value));
  }

  function alphabetGroup(card) {
    var name = typeof card === "string" ? card : (card && card.name);
    var first = trim(name).charAt(0).toUpperCase();
    return /^[A-Z]$/.test(first) ? first : "#";
  }

  function priceValue(card) {
    var raw = trim(card && (card.price != null ? card.price : card.priceUSD));
    if (!raw) return null;
    var parsed = Number(raw.replace(/[^0-9.]/g, ""));
    return Number.isFinite(parsed) ? parsed : null;
  }

  function displayPrice(card) {
    var value = priceValue(card);
    return value === null ? "\u2014" : ("$" + value.toFixed(2));
  }

  function priceGroup(card) {
    var value = priceValue(card);
    if (value === null) return "Unknown";
    if (value === 0) return "Free";
    if (value < 1) return "Under $1";
    if (value < 5) return "$1-$4.99";
    if (value < 10) return "$5-$9.99";
    return "$10+";
  }

  function hasAny(text, needles) {
    text = String(text || "");
    for (var i = 0; i < needles.length; i++) {
      if (text.indexOf(needles[i]) >= 0) return true;
    }
    return false;
  }

  function comboSignal(card) {
    var name = trim(card.name).toLowerCase().replace(/\u2019/g, "'");
    var typeLine = trim(card.typeLine).toLowerCase();
    var oracle = trim(card.oracleText).toLowerCase();
    if (COMBO_NAMES.indexOf(name) >= 0) return true;
    if (oracle.indexOf("win the game") >= 0 || oracle.indexOf("infinite") >= 0) return true;
    return typeLine.indexOf("artifact") >= 0 && oracle.indexOf("untap") >= 0 && oracle.indexOf("add {") >= 0;
  }

  function categoryKeys(card) {
    card = card || {};
    var typeLine = trim(card.typeLine).toLowerCase();
    var oracle = trim(card.oracleText).toLowerCase();
    var cmc = numberValue(card.cmc, 0);
    var keys = [];
    function add(key, condition) {
      if (condition && keys.indexOf(key) < 0) keys.push(key);
    }

    var isLand = typeLine.indexOf("land") >= 0;
    add("lands", isLand);
    add("ramp", !isLand && (oracle.indexOf("add {") >= 0 || (oracle.indexOf("search your library for") >= 0 && oracle.indexOf("land") >= 0) || oracle.indexOf("treasure token") >= 0));
    add("draw", oracle.indexOf("draw ") >= 0 || hasAny(oracle, ["investigate", "connive", "impulse draw"]));
    add("tutor", oracle.indexOf("search your library for") >= 0 && oracle.indexOf("card") >= 0);
    add("counterspell", oracle.indexOf("counter target") >= 0);
    add("removal", hasAny(oracle, ["destroy target", "exile target", "return target", "target creature gets -", "target permanent's owner puts it", "fight target", "deals damage to target creature", "deals damage to any target"]));
    add("board_wipe", hasAny(oracle, ["destroy all", "exile all", "each creature", "all creatures get -"]));
    add("protection", hasAny(oracle, ["hexproof", "indestructible", "protection from", "ward {", "can't be countered", "phase out", "prevent all damage"]));
    add("blink", hasAny(oracle, ["return it to the battlefield", "return them to the battlefield", "return those cards to the battlefield", "flicker"]));
    add("recursion", hasAny(oracle, ["return target card from your graveyard", "return target creature card from your graveyard", "return a card from your graveyard", "from your graveyard to the battlefield", "reanimate"]));
    add("tokens", hasAny(oracle, ["create a", "create two", "create three", "create x", "token"]));
    add("sacrifice", hasAny(oracle, ["sacrifice another", "sacrifice a creature:", "sacrifice a permanent:", "whenever you sacrifice"]));
    add("graveyard", hasAny(oracle, ["mill ", "surveil", "delirium", "escape", "flashback", "threshold"]));
    add("stax", hasAny(oracle, ["can't cast", "can't attack", "don't untap", "skip", "spells cost", "players can't", "each opponent can't", "enters the battlefield tapped"]));
    add("combo", comboSignal(card));
    if (!keys.length) keys.push(typeLine.indexOf("creature") >= 0 || cmc >= 5 ? "threat" : "utility");
    return keys;
  }

  function primaryCategory(card) {
    var keys = categoryKeys(card);
    for (var i = 0; i < CATEGORY_PRIORITY.length; i++) {
      if (keys.indexOf(CATEGORY_PRIORITY[i]) >= 0) return CATEGORY_LABELS[CATEGORY_PRIORITY[i]];
    }
    return "Utility";
  }

  function typeRank(card) {
    var group = primaryType(card && card.typeLine);
    var index = TYPE_SORT_ORDER.indexOf(group);
    return index >= 0 ? index : TYPE_SORT_ORDER.length;
  }

  function compareCards(left, right, sort, direction) {
    left = left || {};
    right = right || {};
    sort = normalizeSort(sort);
    direction = normalizeSortDirection(direction);
    var directionMultiplier = direction === "desc" ? -1 : 1;
    if (sort === "mv") {
      var leftLand = primaryType(left.typeLine) === "Lands";
      var rightLand = primaryType(right.typeLine) === "Lands";
      if (leftLand !== rightLand) return leftLand ? 1 : -1;
      var leftMV = numberValue(left.cmc, 0);
      var rightMV = numberValue(right.cmc, 0);
      if (leftMV !== rightMV) return (leftMV - rightMV) * directionMultiplier;
    } else if (sort === "type") {
      var leftTypeLand = primaryType(left.typeLine) === "Lands";
      var rightTypeLand = primaryType(right.typeLine) === "Lands";
      if (leftTypeLand !== rightTypeLand) return leftTypeLand ? 1 : -1;
      var leftRank = typeRank(left);
      var rightRank = typeRank(right);
      if (leftRank !== rightRank) return (leftRank - rightRank) * directionMultiplier;
    } else if (sort === "price") {
      var leftPrice = priceValue(left);
      var rightPrice = priceValue(right);
      if (leftPrice === null && rightPrice !== null) return 1;
      if (leftPrice !== null && rightPrice === null) return -1;
      if (leftPrice !== null && rightPrice !== null && leftPrice !== rightPrice) {
        return (leftPrice - rightPrice) * directionMultiplier;
      }
    }
    var nameOrder = trim(left.name).localeCompare(trim(right.name));
    return sort === "alphabet" ? nameOrder * directionMultiplier : nameOrder;
  }

  function groupName(card, group) {
    group = normalizeGroup(group);
    if (group === "type") return primaryType(card.typeLine);
    if (group === "mv") return manaValueGroup(card);
    if (group === "category") return primaryCategory(card);
    if (group === "alphabet") return alphabetGroup(card);
    if (group === "price") return priceGroup(card);
    return "";
  }

  function groupOrder(group) {
    group = normalizeGroup(group);
    if (group === "type") return TYPE_ORDER.slice();
    if (group === "mv") return MANA_VALUE_ORDER.slice();
    if (group === "category") return CATEGORY_ORDER.slice();
    if (group === "alphabet") return "ABCDEFGHIJKLMNOPQRSTUVWXYZ".split("").concat(["#"]);
    if (group === "price") return PRICE_ORDER.slice();
    return [""];
  }

  function filterCards(cards, query) {
    cards = Array.isArray(cards) ? cards : [];
    var terms = trim(query).toLowerCase().split(/\s+/).filter(Boolean);
    if (!terms.length) return cards.slice();
    return cards.filter(function (card) {
      var haystack = [card.name, card.typeLine, card.oracleText, card.manaCost, displayManaValue(card), displayPrice(card)].join(" ").toLowerCase();
      for (var i = 0; i < terms.length; i++) {
        if (haystack.indexOf(terms[i]) < 0) return false;
      }
      return true;
    });
  }

  function groupCards(cards, options) {
    options = options || {};
    var group = normalizeGroup(options.group, "none");
    var sort = normalizeSort(options.sort);
    var direction = normalizeSortDirection(options.direction);
    var filtered = filterCards(cards, options.filter);
    if (group === "none") {
      filtered.sort(function (a, b) { return compareCards(a, b, sort, direction); });
      return [{ name: "", cards: filtered }];
    }

    var buckets = {};
    filtered.forEach(function (card) {
      var name = groupName(card, group);
      if (!buckets[name]) buckets[name] = [];
      buckets[name].push(card);
    });

    var groups = [];
    groupOrder(group).forEach(function (name) {
      if (!buckets[name] || !buckets[name].length) return;
      buckets[name].sort(function (a, b) { return compareCards(a, b, sort, direction); });
      groups.push({ name: name, cards: buckets[name] });
      delete buckets[name];
    });
    Object.keys(buckets).sort().forEach(function (name) {
      buckets[name].sort(function (a, b) { return compareCards(a, b, sort, direction); });
      groups.push({ name: name, cards: buckets[name] });
    });
    return groups;
  }

  function orderedCards(cards, options) {
    return groupCards(cards, options).reduce(function (out, group) {
      return out.concat(group.cards);
    }, []);
  }

  function groupRows(rows, options) {
    var cards = (Array.isArray(rows) ? rows : []).map(adaptEditorRow);
    return groupCards(cards, options).map(function (group) {
      return { name: group.name, rows: group.cards.map(function (card) { return card.source; }) };
    });
  }

  function filterRows(rows, query) {
    return filterCards((Array.isArray(rows) ? rows : []).map(adaptEditorRow), query).map(function (card) {
      return card.source;
    });
  }

  function displayManaValue(card) {
    card = card || {};
    if (primaryType(card.typeLine) === "Lands") return "Land";
    var value = numberValue(card.cmc, NaN);
    if (!Number.isFinite(value)) return "\u2014";
    return Math.round(value) === value ? String(value) : String(value.toFixed(1)).replace(/\.0$/, "");
  }

  function displayManaSummary(card) {
    var value = displayManaValue(card);
    return value === "Land" || value === "\u2014" ? value : (value + " MV");
  }

  function setLabel(card) {
    if (card.setName && card.setCode) return card.setName + " (" + card.setCode + ")";
    return card.setName || card.setCode || "Unknown set";
  }

  function boardCards(boards, board) {
    var cards = boards && Array.isArray(boards[board]) ? boards[board] : [];
    return cards.map(function (card, index) { return normalizeCard(card, index); });
  }

  function exportBoardCards(boards, board, metadata) {
    var cards = boardCards(boards, board);
    var commanderName = board === "main" ? trim(metadata && metadata.commanderName).toLowerCase() : "";
    var commanderRemaining = commanderName ? 1 : 0;
    return cards.map(function (card) {
      if (commanderRemaining > 0 && card.name.toLowerCase() === commanderName) {
        var removed = Math.min(commanderRemaining, card.quantity);
        card.quantity -= removed;
        commanderRemaining -= removed;
      }
      return card;
    }).filter(function (card) { return card.quantity > 0; });
  }

  function textExport(boards, metadata) {
    metadata = metadata || {};
    var lines = [];
    if (trim(metadata.name)) lines.push("Name: " + trim(metadata.name));
    if (trim(metadata.format)) lines.push("Format: " + trim(metadata.format));
    if (trim(metadata.commanderName)) {
      var commander = normalizeCard(metadata.commander || { name: metadata.commanderName }, -1);
      commander.name = trim(metadata.commanderName);
      var commanderIdentity = commander.name;
      if (commander.setCode && commander.collectorNumber) {
        commanderIdentity += " (" + commander.setCode + ") " + commander.collectorNumber;
      }
      var commanderPrintID = commander.printID || commander.preferredPrintID;
      if (commanderPrintID) commanderIdentity += " {scryfall:" + commanderPrintID + "}";
      lines.push("Commander:");
      lines.push("1 " + commanderIdentity);
    }
    if (lines.length) lines.push("");
    [["Mainboard", "main"], ["Sideboard", "side"], ["Maybeboard", "maybe"]].forEach(function (entry) {
      var cards = exportBoardCards(boards, entry[1], metadata);
      if (!cards.length && entry[1] !== "main") return;
      lines.push(entry[0] + " (" + String(quantityTotal(cards)) + ")");
      cards.sort(function (a, b) { return compareCards(a, b, "alphabet"); }).forEach(function (card) {
        var identity = card.name;
        if (card.setCode && card.collectorNumber) {
          identity += " (" + card.setCode + ") " + card.collectorNumber;
        }
        var printID = card.printID || card.preferredPrintID;
        if (printID) identity += " {scryfall:" + printID + "}";
        lines.push(String(card.quantity) + " " + identity);
      });
      lines.push("");
    });
    return lines.join("\n").trim() + "\n";
  }

  function csvCell(value) {
    var text = String(value == null ? "" : value);
    return /[",\n]/.test(text) ? ('"' + text.replace(/"/g, '""') + '"') : text;
  }

  function csvExport(boards, metadata) {
    metadata = metadata || {};
    var deckName = trim(metadata.name);
    var format = trim(metadata.format);
    var rows = [["Board", "Quantity", "Name", "Set", "Collector Number", "Print ID", "Price USD", "Deck Name", "Format"]];
    if (trim(metadata.commanderName)) {
      var commander = normalizeCard(metadata.commander || { name: metadata.commanderName }, -1);
      commander.name = trim(metadata.commanderName);
      rows.push([
        "Commander",
        1,
        commander.name,
        commander.setCode,
        commander.collectorNumber,
        commander.printID || commander.preferredPrintID,
        commander.price,
        deckName,
        format
      ]);
    }
    [["Mainboard", "main"], ["Sideboard", "side"], ["Maybeboard", "maybe"]].forEach(function (entry) {
      exportBoardCards(boards, entry[1], metadata).sort(function (a, b) { return compareCards(a, b, "alphabet"); }).forEach(function (card) {
        rows.push([
          entry[0],
          card.quantity,
          card.name,
          card.setCode,
          card.collectorNumber,
          card.printID || card.preferredPrintID,
          card.price,
          deckName,
          format
        ]);
      });
    });
    return rows.map(function (row) { return row.map(csvCell).join(","); }).join("\n") + "\n";
  }

  function fileBaseName(value) {
    var normalized = trim(value).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return normalized || "deck";
  }

  global.ManatombDeckBrowser = {
    adaptEditorRow: adaptEditorRow,
    adaptPublicCard: adaptPublicCard,
    alphabetGroup: alphabetGroup,
    categoryKeys: categoryKeys,
    compareCards: compareCards,
    csvExport: csvExport,
    displayManaSummary: displayManaSummary,
    displayManaValue: displayManaValue,
    displayPrice: displayPrice,
    fileBaseName: fileBaseName,
    filterCards: filterCards,
    filterRows: filterRows,
    groupCards: groupCards,
    groupRows: groupRows,
    manaValueGroup: manaValueGroup,
    normalizeBoard: normalizeBoard,
    normalizeCard: normalizeCard,
    normalizeGroup: normalizeGroup,
    normalizeSort: normalizeSort,
    normalizeSortDirection: normalizeSortDirection,
    normalizeView: normalizeView,
    orderedCards: orderedCards,
    priceGroup: priceGroup,
    priceValue: priceValue,
    primaryCategory: primaryCategory,
    primaryType: primaryType,
    quantityTotal: quantityTotal,
    setLabel: setLabel,
    textExport: textExport,
    trim: trim,
    typeRank: typeRank
  };
})(window);
