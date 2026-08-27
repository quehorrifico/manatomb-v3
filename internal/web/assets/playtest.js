      (function () {
        var cardUI = window.ManatombCardUI || {};
        var trimValue = cardUI.trimValue || function (value) {
          return String(value || "").trim();
        };
        var escapeHTML = cardUI.escapeHTML || function (value) {
          return String(value || "")
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#39;");
        };
        var formatPriceDisplay = cardUI.formatPriceDisplay || function (value, emptyText) {
          var raw = String(value || "").trim();
          if (!raw) return emptyText || "-";
          return raw.charAt(0) === "$" ? raw : ("$" + raw);
        };
        var renderCardTextMarkup = cardUI.renderCardTextMarkup || function (text, emptyText) {
          var raw = String(text || "");
          if (!String(raw || "").trim()) return emptyText || "";
          return raw
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;")
            .replace(/'/g, "&#39;")
            .replace(/\n/g, "<br>");
        };
        var playtestConfig = window.ManatombPlaytestConfig || {};
        var librarySeed = Array.isArray(playtestConfig.librarySeed) ? playtestConfig.librarySeed : [];
        var commanderSeed = playtestConfig.commanderSeed || null;
        var workbenchDraftSeed = playtestConfig.workbenchDraftSeed || null;
        var deckFormat = String(playtestConfig.deckFormat || "");
        var isWorkbenchPlaytest = !!playtestConfig.isWorkbenchPlaytest;
        var cardIDSeed = 1;

        var battlefieldCardWidth = 108;
        var battlefieldCardHeight = 151;
        var battlefieldSnapGap = 8;
        var battlefieldSnapRow = 38;
        var stackedCardWidth = 108;
        var stackedCardHeight = 151;
        var stackedOffsetX = 8;
        var stackedOffsetY = 3;

        function manaSymbolAssetName(symbol) {
          var normalized = String(symbol || "").trim().toUpperCase();
          if (!normalized) return "";
          return normalized.replace(/\//g, "");
        }

        function renderManaSymbols(manaCost) {
          if (cardUI.renderManaSymbols) {
            return cardUI.renderManaSymbols(manaCost);
          }
          var cost = String(manaCost || "").trim();
          if (!cost) return "";
          var html = "";
          var re = /\{([^}]+)\}/g;
          var match;
          while ((match = re.exec(cost)) !== null) {
            var sym = String(match[1] || "").trim();
            if (!sym) continue;
            var assetName = manaSymbolAssetName(sym);
            if (!assetName) continue;
            html += '<img src="https://svgs.scryfall.io/card-symbols/' +
              encodeURIComponent(assetName) +
              '.svg" alt="{' + sym + '}" class="inline-block h-5 w-5 align-text-bottom">';
          }
          return html;
        }

        function renderManaPip(symbol, sizeClass) {
          var raw = String(symbol || "").trim().toUpperCase();
          if (!raw) return "";
          if (cardUI.renderCardSymbol) {
            return cardUI.renderCardSymbol(raw, sizeClass || "h-5 w-5");
          }
          return renderManaSymbols("{" + raw + "}");
        }

        function renderToolbarManaPips() {
          var html = "";
          ["w", "u", "b", "r", "g", "c"].forEach(function (color) {
            var count = Number((state.mana && state.mana[color]) || 0);
            if (count <= 0) return;
            html += renderManaPip(color, "h-4 w-4");
          });
          return html || renderManaPip("c", "h-4 w-4");
        }

        function normalizeCardColorList(value) {
          var source = [];
          if (Array.isArray(value)) {
            source = value;
          } else if (typeof value === "string") {
            source = value.split(/[,\s]+/);
          }
          var seen = {};
          var out = [];
          for (var i = 0; i < source.length; i++) {
            var color = String(source[i] || "").trim().toUpperCase();
            if (!color || seen[color]) continue;
            seen[color] = true;
            out.push(color);
          }
          return out;
        }

        function createCardFromMeta(meta, isCommander) {
          meta = meta || {};
          var rawManaValue = meta.manaValue != null ? meta.manaValue : (meta.mana_value != null ? meta.mana_value : meta.cmc);
          var manaValue = Number(rawManaValue || 0);
          if (!Number.isFinite(manaValue)) manaValue = 0;
          return {
            id: cardIDSeed++,
            name: String(meta.name || "").trim(),
            imageURI: String(meta.imageURI || meta.image_uri || "").trim(),
            manaCost: String(meta.manaCost || meta.mana_cost || "").trim(),
            manaValue: manaValue,
            colors: normalizeCardColorList(meta.colors),
            colorIdentity: normalizeCardColorList(meta.colorIdentity || meta.color_identity),
            typeLine: String(meta.typeLine || meta.type_line || "").trim(),
            oracleText: String(meta.oracleText || meta.oracle_text || "").trim(),
            flavorText: String(meta.flavorText || meta.flavor_text || "").trim(),
            priceUSD: String(meta.priceUSD || meta.price_usd || "").trim(),
            artist: String(meta.artist || "").trim(),
            setCode: String(meta.setCode || meta.set_code || "").trim(),
            setName: String(meta.setName || meta.set_name || "").trim(),
            collectorNumber: String(meta.collectorNumber || meta.collector_number || "").trim(),
            tapped: false,
            isCommander: !!isCommander,
            counters: {},
            bfX: null,
            bfY: null
          };
        }

        function createCard(name, imageURI, manaCost, typeLine, oracleText, isCommander) {
          return createCardFromMeta({
            name: name,
            imageURI: imageURI,
            manaCost: manaCost,
            typeLine: typeLine,
            oracleText: oracleText
          }, isCommander);
        }

        function isLandCard(card) {
          if (!card) return false;
          return /land/i.test(String(card.typeLine || ""));
        }

        function cardHasType(card, typeName) {
          if (!card) return false;
          var wanted = String(typeName || "").trim();
          if (!wanted) return false;
          return new RegExp("\\b" + wanted.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "\\b", "i").test(String(card.typeLine || ""));
        }

        function isArtifactCard(card) {
          return cardHasType(card, "Artifact");
        }

        function isEnchantmentCard(card) {
          return cardHasType(card, "Enchantment");
        }

        function isPlaneswalkerCard(card) {
          return cardHasType(card, "Planeswalker");
        }

        function shuffle(cards) {
          for (var i = cards.length - 1; i > 0; i--) {
            var j = Math.floor(Math.random() * (i + 1));
            var tmp = cards[i];
            cards[i] = cards[j];
            cards[j] = tmp;
          }
          return cards;
        }

        function buildLibrary(seedRows) {
          var lib = [];
          for (var i = 0; i < seedRows.length; i++) {
            var row = seedRows[i] || {};
            var name = String(row.name || "").trim();
            var qty = Number(row.qty || 0);
            if (!name || qty <= 0) continue;
            for (var q = 0; q < qty; q++) {
              lib.push(createCardFromMeta(row, false));
            }
          }
          return lib;
        }

        function zoneLabel(zone) {
          if (zone === "command") return "Command";
          if (zone === "library") return "Library";
          if (zone === "hand") return "Hand";
          if (zone === "battlefield") return "Battlefield";
          if (zone === "graveyard") return "Graveyard";
          if (zone === "exile") return "Exile";
          return zone;
        }

        var zoneOrder = ["command", "library", "hand", "battlefield", "graveyard", "exile"];
        function hasCommanderPayload() {
          return !!(commanderSeed && String(commanderSeed.name || "").trim());
        }

        function startingLifeTotal(format) {
          var normalized = String(format || "").trim().toLowerCase();
          if (hasCommanderPayload() && (!normalized || normalized === "commander" || normalized === "sandbox")) return 40;
          switch (normalized) {
            case "commander":
              return 40;
            case "duel commander":
            case "standard":
            case "pioneer":
            case "modern":
            case "legacy":
            case "vintage":
            case "pauper":
            case "historic":
            case "explorer":
            case "timeless":
            case "alchemy":
            case "oathbreaker":
            case "premodern":
            case "draft":
            case "sealed":
            case "cube":
            case "casual":
              return 20;
            case "brawl":
            case "historic brawl":
              return 25;
            default:
              return 20;
          }
        }

        var state = {
          phase: "opening", // opening | cleanup | play
          zones: {
            command: [],
            library: [],
            hand: [],
            battlefield: [],
            graveyard: [],
            exile: []
          },
          turn: 0,
          life: startingLifeTotal(deckFormat),
          commanderDamage: 0,
          commandTax: 0,
          mana: {
            w: 0,
            u: 0,
            b: 0,
            r: 0,
            g: 0,
            c: 0
          },
          mulligans: 0,
          openingSelectionIDs: [],
          draggingCardID: null,
          draggingOpeningCard: false,
          draggingScryCard: false,
          hoveredCardID: null,
          pointerX: null,
          pointerY: null,
          activeCardID: null,
          scryCards: [],
          scryMode: "scry",
          scryDragPreview: null,
          revealBottomRandom: false,
          revealBottomedCount: 0,
          librarySearchQuery: "",
          libraryRevealed: false,
          countPickerAction: "",
          history: []
        };

        var openingScreen = document.getElementById("pt-opening-screen");
        var openingHandEl = document.getElementById("pt-opening-hand");
        var openingLabelEl = document.getElementById("pt-opening-label");
        var openingHintEl = document.getElementById("pt-opening-hint");
        var openingProgressEl = document.getElementById("pt-opening-progress");

        var zoneEls = {
          command: document.getElementById("pt-zone-command"),
          library: document.getElementById("pt-zone-library"),
          hand: document.getElementById("pt-zone-hand"),
          battlefield: document.getElementById("pt-zone-battlefield"),
          graveyard: document.getElementById("pt-zone-graveyard"),
          exile: document.getElementById("pt-zone-exile")
        };

        var gameBarEl = document.querySelector(".pt-game-bar");
        var elStatus = document.getElementById("pt-status");
        var elVisibleStatus = document.getElementById("pt-visible-status");
        var elVisibleStatusText = document.getElementById("pt-visible-status-text");
        var elTurnCount = document.getElementById("pt-turn-count");
        var elToolbarTurn = document.getElementById("pt-toolbar-turn");
        var elToolbarLife = document.getElementById("pt-toolbar-life");
        var elToolbarDamage = document.getElementById("pt-toolbar-damage");
        var elToolbarMana = document.getElementById("pt-toolbar-mana");
        var elLifeTotal = document.getElementById("pt-life-total");
        var elCommanderDamageTotal = document.getElementById("pt-commander-damage-total");
        var elCommandTaxValue = document.getElementById("pt-command-tax-value");
        var manaCountEls = {
          w: document.getElementById("pt-mana-w"),
          u: document.getElementById("pt-mana-u"),
          b: document.getElementById("pt-mana-b"),
          r: document.getElementById("pt-mana-r"),
          g: document.getElementById("pt-mana-g"),
          c: document.getElementById("pt-mana-c")
        };
        var countEls = {
          command: document.getElementById("pt-count-command"),
          library: document.getElementById("pt-count-library"),
          hand: document.getElementById("pt-count-hand"),
          graveyard: document.getElementById("pt-count-graveyard"),
          exile: document.getElementById("pt-count-exile")
        };

        var btnKeep = document.getElementById("pt-keep");
        var btnMulligan = document.getElementById("pt-mulligan");
        var btnLifeDown = document.getElementById("pt-life-down");
        var btnLifeUp = document.getElementById("pt-life-up");
        var btnCommanderDamageDown = document.getElementById("pt-commander-damage-down");
        var btnCommanderDamageUp = document.getElementById("pt-commander-damage-up");
        var btnCommandTaxDown = document.getElementById("pt-command-tax-down");
        var btnCommandTaxUp = document.getElementById("pt-command-tax-up");
        var manaStepButtons = document.querySelectorAll(".pt-mana-step");
        var btnManaClear = document.getElementById("pt-mana-clear");
        var btnTokenToggle = document.getElementById("pt-token-toggle");
        var tokenPanel = document.getElementById("pt-token-panel");
        var btnTokenOpen = document.getElementById("pt-token-open");
        var tokenPresetButtons = document.querySelectorAll(".pt-token-preset");
        var btnCoinFlip = document.getElementById("pt-coin-flip");
        var btnOrganizeBoard = document.getElementById("pt-organize-board");
        var btnDiceToggle = document.getElementById("pt-dice-toggle");
        var dicePanel = document.getElementById("pt-dice-panel");
        var diceOptionButtons = document.querySelectorAll(".pt-dice-option");
        var toolbarMenuButtons = document.querySelectorAll("[data-toolbar-menu-toggle]");
        var toolbarMenus = document.querySelectorAll("[data-toolbar-menu]");
        var btnCommandMenuToggle = document.getElementById("pt-command-menu-toggle");
        var commandMenu = document.getElementById("pt-command-menu");
        var btnLibraryMenuToggle = document.getElementById("pt-library-menu-toggle");
        var libraryMenu = document.getElementById("pt-library-menu");
        var btnLibraryMenuDraw = document.getElementById("pt-library-menu-draw");
        var btnLibraryMenuDrawX = document.getElementById("pt-library-menu-draw-x");
        var btnLibraryMenuShuffle = document.getElementById("pt-library-menu-shuffle");
        var btnLibraryMenuSearch = document.getElementById("pt-library-menu-search");
        var btnLibraryMenuReveal = document.getElementById("pt-library-menu-reveal");
        var btnLibraryMenuRevealX = document.getElementById("pt-library-menu-reveal-x");
        var btnLibraryMenuScry = document.getElementById("pt-library-menu-scry");
        var btnLibraryMenuScryX = document.getElementById("pt-library-menu-scry-x");
        var btnLibraryMenuSurveil = document.getElementById("pt-library-menu-surveil");
        var btnLibraryMenuSurveilX = document.getElementById("pt-library-menu-surveil-x");
        var btnLibraryMenuMill = document.getElementById("pt-library-menu-mill");
        var btnLibraryMenuMillX = document.getElementById("pt-library-menu-mill-x");
        var btnLibraryMenuKeywordToggle = document.getElementById("pt-library-menu-keyword-toggle");
        var libraryKeywordPanel = document.getElementById("pt-library-menu-keyword-panel");
        var btnLibraryMenuDiscover = document.getElementById("pt-library-menu-discover");
        var btnLibraryMenuCascade = document.getElementById("pt-library-menu-cascade");
        var btnLibraryMenuRevealUntilToggle = document.getElementById("pt-library-menu-reveal-until-toggle");
        var libraryRevealUntilPanel = document.getElementById("pt-library-menu-reveal-until-panel");
        var libraryRevealUntilButtons = document.querySelectorAll(".pt-library-reveal-until");
        var btnGameMenuRestart = document.getElementById("pt-game-menu-restart");
        var gameMenuReturnLink = document.getElementById("pt-game-menu-return");
        var scryOverlay = document.getElementById("pt-scry-overlay");
        var scryTitleEl = document.getElementById("pt-scry-title");
        var scryCountEl = document.getElementById("pt-scry-count");
        var scryHintEl = document.getElementById("pt-scry-hint");
        var btnScryBackdrop = document.getElementById("pt-scry-backdrop");
        var btnScryCancel = document.getElementById("pt-scry-cancel");
        var btnScryFinish = document.getElementById("pt-scry-finish");
        var scryCardsEl = document.getElementById("pt-scry-cards");
        var librarySearchOverlay = document.getElementById("pt-library-search-overlay");
        var btnLibrarySearchBackdrop = document.getElementById("pt-library-search-backdrop");
        var btnLibrarySearchClose = document.getElementById("pt-library-search-close");
        var librarySearchInput = document.getElementById("pt-library-search-input");
        var librarySearchResultsEl = document.getElementById("pt-library-search-results");
        var librarySearchCountEl = document.getElementById("pt-library-search-count");
        var countPickerOverlay = document.getElementById("pt-count-picker-overlay");
        var btnCountPickerBackdrop = document.getElementById("pt-count-picker-backdrop");
        var btnCountPickerClose = document.getElementById("pt-count-picker-close");
        var btnCountPickerCustom = document.getElementById("pt-count-picker-custom");
        var countPickerTitle = document.getElementById("pt-count-picker-title");
        var countPickerHint = document.getElementById("pt-count-picker-hint");
        var countPickerOptions = document.getElementById("pt-count-picker-options");
        var customCountOverlay = document.getElementById("pt-custom-count-overlay");
        var btnCustomCountBackdrop = document.getElementById("pt-custom-count-backdrop");
        var btnCustomCountClose = document.getElementById("pt-custom-count-close");
        var btnCustomCountSubmit = document.getElementById("pt-custom-count-submit");
        var customCountTitle = document.getElementById("pt-custom-count-title");
        var customCountInput = document.getElementById("pt-custom-count-input");
        var historyOverlay = document.getElementById("pt-history-overlay");
        var btnHistoryBackdrop = document.getElementById("pt-history-backdrop");
        var btnHistoryClose = document.getElementById("pt-history-close");
        var historyListEl = document.getElementById("pt-history-list");
        var dragAvatarEl = null;
        var dragSourceEl = null;
        var dragOffsetX = 0;
        var dragOffsetY = 0;
        var dragAvatarWidth = 0;
        var dragAvatarHeight = 0;
        var dragAvatarVisualWidth = 0;
        var dragAvatarVisualHeight = 0;
        var transparentDragImage = null;
        var cardInlineMenu = document.getElementById("pt-card-inline-menu");
        var btnCardInlineMenuPrimary = document.getElementById("pt-card-inline-menu-primary");
        var cardInlineMenuPrimaryLabel = document.getElementById("pt-card-inline-menu-primary-label");
        var cardInlineMenuPrimaryShortcut = document.getElementById("pt-card-inline-menu-primary-shortcut");
        var btnCardInlineMenuMoveToggle = document.getElementById("pt-card-inline-menu-move-toggle");
        var cardInlineMenuMovePanel = document.getElementById("pt-card-inline-menu-move-panel");
        var btnCardInlineMenuCountersToggle = document.getElementById("pt-card-inline-menu-counters-toggle");
        var cardInlineMenuCountersPanel = document.getElementById("pt-card-inline-menu-counters-panel");
        var btnCardInlineMenuCopyToggle = document.getElementById("pt-card-inline-menu-copy-toggle");
        var cardInlineMenuCopyPanel = document.getElementById("pt-card-inline-menu-copy-panel");
        var btnCardInlineMenuDetails = document.getElementById("pt-card-inline-menu-details");
        var cardInlineMoveButtons = document.querySelectorAll(".pt-card-inline-move-btn");
        var cardInlineCounterButtons = document.querySelectorAll(".pt-card-inline-counter-btn");
        var cardInlineCopyButtons = document.querySelectorAll(".pt-card-inline-copy-btn");
        var cardInlineMenuTrigger = null;
        var cardDetailOverlay = document.getElementById("pt-card-detail-overlay");
        var btnCardDetailBackdrop = document.getElementById("pt-card-detail-backdrop");
        var btnCardDetailClose = document.getElementById("pt-card-detail-close");
        var cardDetailName = document.getElementById("pt-card-detail-name");
        var cardDetailType = document.getElementById("pt-card-detail-type");
        var cardDetailNameField = document.getElementById("pt-card-detail-name-field");
        var cardDetailTypeField = document.getElementById("pt-card-detail-type-field");
        var cardDetailManaValue = document.getElementById("pt-card-detail-mana-value");
        var cardDetailColors = document.getElementById("pt-card-detail-colors");
        var cardDetailColorIdentity = document.getElementById("pt-card-detail-color-identity");
        var cardDetailPrice = document.getElementById("pt-card-detail-price");
        var cardDetailArtist = document.getElementById("pt-card-detail-artist");
        var cardDetailSet = document.getElementById("pt-card-detail-set");
        var cardDetailCollectorNumber = document.getElementById("pt-card-detail-collector-number");
        var cardDetailOracle = document.getElementById("pt-card-detail-oracle");
        var cardDetailFlavor = document.getElementById("pt-card-detail-flavor");
        var cardDetailImage = document.getElementById("pt-card-detail-image");
        var cardDetailFallback = document.getElementById("pt-card-detail-fallback");
        var openingReturnLink = document.getElementById("pt-opening-return");

        [
          tokenPanel,
          dicePanel,
          libraryKeywordPanel,
          libraryRevealUntilPanel,
          cardInlineMenuMovePanel,
          cardInlineMenuCountersPanel,
          cardInlineMenuCopyPanel
        ].forEach(function (panel) {
          if (panel && panel.parentElement !== document.body) {
            document.body.appendChild(panel);
          }
        });

        function buildWorkbenchReturnPayload() {
          if (!isWorkbenchPlaytest) return null;
          var baseDraft = (workbenchDraftSeed && typeof workbenchDraftSeed === "object") ? workbenchDraftSeed : {};
          var commanderName = String(baseDraft.commanderName || "").trim();
          var commanderPrintID = String(baseDraft.commanderPrintID || "").trim();
          var format = String(baseDraft.format || "").trim() || "Sandbox";
          var deckName = String(baseDraft.name || "").trim() || "Untitled Deck";

          var cardEntries = [];
          var cardSource = (baseDraft && baseDraft.cards && typeof baseDraft.cards === "object") ? baseDraft.cards : {};
          var cardNames = Object.keys(cardSource);
          for (var n = 0; n < cardNames.length; n++) {
            var cardName = String(cardNames[n] || "").trim();
            var qty = Number(cardSource[cardName] || 0);
            if (!cardName || !Number.isFinite(qty) || qty <= 0) continue;
            cardEntries.push({ name: cardName, qty: qty });
          }

          var maybeEntries = [];
          var sideboardEntries = [];
          var maybeCards = (baseDraft && baseDraft.maybeCards && typeof baseDraft.maybeCards === "object") ? baseDraft.maybeCards : {};
          var sideboardCards = (baseDraft && baseDraft.sideboardCards && typeof baseDraft.sideboardCards === "object") ? baseDraft.sideboardCards : {};
          var sideboardNames = Object.keys(sideboardCards);
          for (var s = 0; s < sideboardNames.length; s++) {
            var sideboardName = String(sideboardNames[s] || "").trim();
            var sideboardQty = Number(sideboardCards[sideboardName] || 0);
            if (!sideboardName || !Number.isFinite(sideboardQty) || sideboardQty <= 0) continue;
            sideboardEntries.push({ name: sideboardName, qty: sideboardQty });
          }

          var maybeNames = Object.keys(maybeCards);
          for (var m = 0; m < maybeNames.length; m++) {
            var maybeName = String(maybeNames[m] || "").trim();
            var maybeQty = Number(maybeCards[maybeName] || 0);
            if (!maybeName || !Number.isFinite(maybeQty) || maybeQty <= 0) continue;
            maybeEntries.push({ name: maybeName, qty: maybeQty });
          }

          return {
            commander_name: commanderName,
            commander_print_id: commanderPrintID,
            format: format,
            name: deckName,
            description: String(baseDraft.description || "").trim(),
            tags: String(baseDraft.tags || "").trim(),
            cards: cardEntries,
            sideboard_cards: sideboardEntries,
            maybe_cards: maybeEntries,
            commander_candidates: Array.isArray(baseDraft.commanderCandidates) ? baseDraft.commanderCandidates.slice() : [],
            card_meta: (baseDraft && baseDraft.cardMeta && typeof baseDraft.cardMeta === "object") ? baseDraft.cardMeta : {},
            board_card_meta: (baseDraft && baseDraft.boardCardMeta && typeof baseDraft.boardCardMeta === "object") ? baseDraft.boardCardMeta : {},
            sandbox: !!(baseDraft && baseDraft.sandbox)
          };
        }

        function persistWorkbenchDraftLocally() {
          if (!isWorkbenchPlaytest) return;

          try {
            var payload = buildWorkbenchReturnPayload();
            if (!payload) return;

            var storageKey = payload.sandbox ? "manatomb.draftDeck.sandbox" : "manatomb.draftDeck";
            var cards = {};
            var sideboardCards = {};
            var maybeCards = {};

            if (Array.isArray(payload.cards)) {
              for (var i = 0; i < payload.cards.length; i++) {
                var card = payload.cards[i] || {};
                var cardName = String(card.name || "").trim();
                var cardQty = Number(card.qty || 0);
                if (!cardName || !Number.isFinite(cardQty) || cardQty <= 0) continue;
                cards[cardName] = (cards[cardName] || 0) + cardQty;
              }
            }

            if (Array.isArray(payload.sideboard_cards)) {
              for (var s = 0; s < payload.sideboard_cards.length; s++) {
                var sideboardCard = payload.sideboard_cards[s] || {};
                var sideboardName = String(sideboardCard.name || "").trim();
                var sideboardQty = Number(sideboardCard.qty || 0);
                if (!sideboardName || !Number.isFinite(sideboardQty) || sideboardQty <= 0) continue;
                sideboardCards[sideboardName] = (sideboardCards[sideboardName] || 0) + sideboardQty;
              }
            }

            if (Array.isArray(payload.maybe_cards)) {
              for (var j = 0; j < payload.maybe_cards.length; j++) {
                var maybeCard = payload.maybe_cards[j] || {};
                var maybeName = String(maybeCard.name || "").trim();
                var maybeQty = Number(maybeCard.qty || 0);
                if (!maybeName || !Number.isFinite(maybeQty) || maybeQty <= 0) continue;
                maybeCards[maybeName] = (maybeCards[maybeName] || 0) + maybeQty;
              }
            }

            var draft = {
              commanderName: String(payload.commander_name || "").trim(),
              commanderPrintID: String(payload.commander_print_id || "").trim(),
              format: String(payload.format || "").trim() || "Sandbox",
              name: String(payload.name || "").trim() || "Untitled Deck",
              description: String(payload.description || "").trim(),
              tags: String(payload.tags || "").trim(),
              cards: cards,
              sideboardCards: sideboardCards,
              maybeCards: maybeCards,
              cardMeta: (payload.card_meta && typeof payload.card_meta === "object") ? payload.card_meta : {},
              boardCardMeta: (payload.board_card_meta && typeof payload.board_card_meta === "object") ? payload.board_card_meta : {},
              commanderCandidates: Array.isArray(payload.commander_candidates) ? payload.commander_candidates : [],
              sandbox: !!payload.sandbox,
              updatedAt: new Date().toISOString()
            };

            var raw = JSON.stringify(draft);
            localStorage.setItem(storageKey, raw);
            try {
              sessionStorage.setItem(storageKey + ".playtestReturn", raw);
            } catch (e) {
              // ignore session storage failures
            }
          } catch (e) {
            // ignore storage failures
          }
        }

        function submitWorkbenchReturn() {
          if (!isWorkbenchPlaytest) {
            if (gameMenuReturnLink && gameMenuReturnLink.href) {
              window.location.assign(gameMenuReturnLink.href);
            }
            return;
          }

          try {
            persistWorkbenchDraftLocally();
            if (gameMenuReturnLink && gameMenuReturnLink.href) {
              window.location.assign(gameMenuReturnLink.href);
              return;
            }
            window.location.assign("/decks/new/workbench");
          } catch (e) {
            if (gameMenuReturnLink && gameMenuReturnLink.href) {
              window.location.assign(gameMenuReturnLink.href);
            }
          }
        }

        function motionReduced() {
          return !!(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
        }

        function rectSnapshot(rect) {
          if (!rect) return null;
          return {
            left: Number(rect.left || 0),
            top: Number(rect.top || 0),
            width: Math.max(1, Number(rect.width || 0)),
            height: Math.max(1, Number(rect.height || 0))
          };
        }

        function cardNodeFor(cardID, zoneName) {
          var id = String(cardID || "");
          if (!id) return null;
          var roots = [];
          if (zoneName && zoneEls[zoneName]) roots.push(zoneEls[zoneName]);
          roots.push(document);
          for (var r = 0; r < roots.length; r++) {
            var nodes = roots[r].querySelectorAll("[data-card-id]");
            for (var i = 0; i < nodes.length; i++) {
              var node = nodes[i];
              if (String(node.dataset.cardId || "") !== id) continue;
              if (zoneName && String(node.dataset.zoneCard || "") !== zoneName) continue;
              if (!node.getClientRects || node.getClientRects().length === 0) continue;
              return node;
            }
          }
          return null;
        }

        function zoneAnchorRect(zoneName) {
          var el = zoneEls[String(zoneName || "")];
          if (!el) return null;
          var visibleCard = el.querySelector(".pt-card");
          var libraryBack = el.querySelector(".pt-library-stack__card");
          var anchor = visibleCard || libraryBack;
          if (anchor && anchor.getClientRects && anchor.getClientRects().length > 0) {
            return rectSnapshot(anchor.getBoundingClientRect());
          }

          var rect = el.getBoundingClientRect();
          var width = stackedCardWidth;
          var height = stackedCardHeight;
          return {
            left: rect.left + Math.max(0, (rect.width - width) / 2),
            top: rect.top + Math.max(0, (rect.height - height) / 2),
            width: width,
            height: height
          };
        }

        function cloneMotionNode(templateNode, className) {
          if (!templateNode) return null;
          var clone = templateNode.cloneNode(true);
          clone.removeAttribute("id");
          clone.removeAttribute("draggable");
          clone.classList.remove("pt-card--landing-hidden", "pt-card--dragging-origin", "pt-card-flight", "pt-drag-avatar");
          clone.classList.add(className);
          clone.querySelectorAll("button").forEach(function (btn) {
            btn.remove();
          });
          return clone;
        }

        function captureCardMotionSource(cardID, zoneName) {
          var node = cardNodeFor(cardID, zoneName);
          if (node) {
            return {
              node: node.cloneNode(true),
              rect: rectSnapshot(node.getBoundingClientRect())
            };
          }
          return {
            node: null,
            rect: zoneAnchorRect(zoneName)
          };
        }

        function animationDestination(cardID, zoneName) {
          var node = cardNodeFor(cardID, zoneName);
          if (node) {
            return {
              node: node,
              rect: rectSnapshot(node.getBoundingClientRect())
            };
          }
          return {
            node: null,
            rect: zoneAnchorRect(zoneName)
          };
        }

        function staggerRect(rect, index) {
          if (!rect) return null;
          var offset = Math.min(index || 0, 6) * 3;
          return {
            left: rect.left + offset,
            top: rect.top - offset,
            width: rect.width,
            height: rect.height
          };
        }

        function playCardFlight(templateNode, fromRect, toRect, landingNode, delay) {
          if (motionReduced() || !templateNode || !fromRect || !toRect) return;
          var templateTapped = !!(templateNode.classList && templateNode.classList.contains("pt-card--tapped"));
          var landingTapped = !!(landingNode && landingNode.classList && landingNode.classList.contains("pt-card--tapped"));
          if (templateTapped || landingTapped) return;
          var clone = cloneMotionNode(templateNode, "pt-card-flight");
          if (!clone) return;

          clone.style.left = fromRect.left + "px";
          clone.style.top = fromRect.top + "px";
          clone.style.width = fromRect.width + "px";
          clone.style.height = fromRect.height + "px";
          clone.style.transform = "translate3d(0, 0, 0)";
          document.body.appendChild(clone);

          if (landingNode) landingNode.classList.add("pt-card--landing-hidden");

          var dx = toRect.left - fromRect.left;
          var dy = toRect.top - fromRect.top;
          var sx = toRect.width / fromRect.width;
          var sy = toRect.height / fromRect.height;
          var wait = Math.max(0, Number(delay || 0));
          var done = false;

          function cleanup() {
            if (done) return;
            done = true;
            if (landingNode) landingNode.classList.remove("pt-card--landing-hidden");
            clone.remove();
          }

          window.setTimeout(function () {
            clone.style.transform = "translate3d(" + dx + "px, " + dy + "px, 0) scale(" + sx + ", " + sy + ")";
          }, wait + 20);
          window.setTimeout(cleanup, wait + 420);
        }

        function animateCardToZone(cardID, source, targetZone, delay) {
          if (!source || !source.rect) return;
          var dest = animationDestination(cardID, targetZone);
          var template = source.node || dest.node;
          playCardFlight(template, source.rect, dest.rect, dest.node, delay);
        }

        function animateCardFromRect(cardID, fromRect, targetZone, delay) {
          if (!fromRect) return;
          var dest = animationDestination(cardID, targetZone);
          playCardFlight(dest.node, fromRect, dest.rect, dest.node, delay);
        }

        function animateCardsFromRect(cards, fromRect, targetZone) {
          if (!Array.isArray(cards) || !fromRect) return;
          for (var i = 0; i < cards.length; i++) {
            if (!cards[i]) continue;
            animateCardFromRect(cards[i].id, staggerRect(fromRect, i), targetZone, i * 35);
          }
        }

        function animateOpeningMulligan(oldSources, drawnCards, libraryRect) {
          if (motionReduced()) return;
          var targetRect = libraryRect || zoneAnchorRect("library");
          if (Array.isArray(oldSources) && targetRect) {
            for (var i = 0; i < oldSources.length; i++) {
              var source = oldSources[i];
              if (!source || !source.node || !source.rect) continue;
              playCardFlight(source.node, staggerRect(source.rect, i), staggerRect(targetRect, i), null, i * 22);
            }
          }
          if (Array.isArray(drawnCards) && targetRect) {
            for (var j = 0; j < drawnCards.length; j++) {
              if (!drawnCards[j]) continue;
              animateCardFromRect(drawnCards[j].id, staggerRect(targetRect, j), "hand", 170 + j * 48);
            }
          }
        }

        function getTransparentDragImage() {
          if (transparentDragImage) return transparentDragImage;
          transparentDragImage = new Image();
          transparentDragImage.src = "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==";
          return transparentDragImage;
        }

        function updateDragAvatar(e) {
          if (!dragAvatarEl || !e || typeof e.clientX !== "number" || typeof e.clientY !== "number") return;
          if (e.clientX === 0 && e.clientY === 0) return;
          var visualLeft = e.clientX - dragOffsetX;
          var visualTop = e.clientY - dragOffsetY;
          dragAvatarEl.style.left = (visualLeft + (dragAvatarVisualWidth - dragAvatarWidth) / 2) + "px";
          dragAvatarEl.style.top = (visualTop + (dragAvatarVisualHeight - dragAvatarHeight) / 2) + "px";
        }

        function clearDragAvatar() {
          if (dragSourceEl) dragSourceEl.classList.remove("pt-card--dragging-origin");
          if (dragAvatarEl) dragAvatarEl.remove();
          dragAvatarEl = null;
          dragSourceEl = null;
          dragOffsetX = 0;
          dragOffsetY = 0;
          dragAvatarWidth = 0;
          dragAvatarHeight = 0;
          dragAvatarVisualWidth = 0;
          dragAvatarVisualHeight = 0;
        }

        function startDragAvatar(node, e) {
          if (!node) return;
          clearDragAvatar();
          var rect = node.getBoundingClientRect();
          var tapped = node.classList.contains("pt-card--tapped");
          dragAvatarWidth = Math.max(1, Number(node.offsetWidth || rect.width || stackedCardWidth));
          dragAvatarHeight = Math.max(1, Number(node.offsetHeight || rect.height || stackedCardHeight));
          dragAvatarVisualWidth = tapped ? dragAvatarHeight : dragAvatarWidth;
          dragAvatarVisualHeight = tapped ? dragAvatarWidth : dragAvatarHeight;
          dragOffsetX = Math.max(0, Math.min(rect.width, e.clientX - rect.left));
          dragOffsetY = Math.max(0, Math.min(rect.height, e.clientY - rect.top));
          dragAvatarEl = cloneMotionNode(node, "pt-drag-avatar");
          if (!dragAvatarEl) return;
          dragAvatarEl.style.left = (rect.left + (dragAvatarVisualWidth - dragAvatarWidth) / 2) + "px";
          dragAvatarEl.style.top = (rect.top + (dragAvatarVisualHeight - dragAvatarHeight) / 2) + "px";
          dragAvatarEl.style.width = dragAvatarWidth + "px";
          dragAvatarEl.style.height = dragAvatarHeight + "px";
          document.body.appendChild(dragAvatarEl);
          dragSourceEl = node;
          node.classList.add("pt-card--dragging-origin");
          updateDragAvatar(e);
          if (e.dataTransfer) e.dataTransfer.setDragImage(getTransparentDragImage(), 0, 0);
        }

        function setStatus(msg) {
          var text = String(msg || "").trim();
          if (elStatus) elStatus.textContent = text;
          if (elVisibleStatusText) {
            elVisibleStatusText.textContent = text || "Ready.";
          } else if (elVisibleStatus) {
            elVisibleStatus.textContent = text || "Ready.";
          }
          if (text) {
            var history = Array.isArray(state.history) ? state.history : [];
            var last = history.length > 0 ? history[history.length - 1] : null;
            if (!last || last.text !== text || last.turn !== state.turn) {
              history.push({
                turn: state.turn,
                text: text
              });
              if (history.length > 120) history.shift();
            }
            state.history = history;
            if (historyOverlayOpen()) renderHistory();
          }
        }

        function manaTotal() {
          var total = 0;
          Object.keys(state.mana || {}).forEach(function (color) {
            total += Number(state.mana[color] || 0);
          });
          return total;
        }

        function installManaPickerSymbols() {
          var pairs = [
            ["pt-mana-item--w", "W"],
            ["pt-mana-item--u", "U"],
            ["pt-mana-item--b", "B"],
            ["pt-mana-item--r", "R"],
            ["pt-mana-item--g", "G"],
            ["pt-mana-item--c", "C"]
          ];
          pairs.forEach(function (pair) {
            var item = document.querySelector("." + pair[0] + " .pt-mana-symbol");
            if (item) item.innerHTML = renderManaPip(pair[1], "h-5 w-5");
          });
        }

        function toolbarMenuOpen() {
          var open = false;
          toolbarMenus.forEach(function (menu) {
            if (!menu.classList.contains("hidden")) open = true;
          });
          return open;
        }

        function toolbarMenuFor(name) {
          var found = null;
          toolbarMenus.forEach(function (menu) {
            if (String(menu.dataset.toolbarMenu || "") === name) found = menu;
          });
          return found;
        }

        function toolbarTriggerFor(name) {
          var found = null;
          toolbarMenuButtons.forEach(function (btn) {
            if (String(btn.dataset.toolbarMenuToggle || "") === name) found = btn;
          });
          return found;
        }

        function expandedToolbarButton() {
          var found = null;
          toolbarMenuButtons.forEach(function (btn) {
            if (btn.getAttribute("aria-expanded") === "true") found = btn;
          });
          return found;
        }

        function clampNumber(value, min, max) {
          if (max < min) return min;
          return Math.min(max, Math.max(min, value));
        }

        function positionToolbarMenu(menu, trigger) {
          if (!menu || !trigger || !gameBarEl) return;
          var barRect = gameBarEl.getBoundingClientRect();
          var triggerRect = trigger.getBoundingClientRect();
          var menuWidth = menu.offsetWidth || 208;
          var left = triggerRect.left - barRect.left + (triggerRect.width / 2) - (menuWidth / 2);
          left = clampNumber(left, 8, barRect.width - menuWidth - 8);
          menu.style.left = String(Math.round(left)) + "px";
          menu.style.right = "auto";
        }

        function positionInlineMenu(menu, trigger) {
          if (!menu || !trigger) return;
          var triggerRect = trigger.getBoundingClientRect();
          var menuWidth = menu.offsetWidth || 136;
          var menuHeight = menu.offsetHeight || 220;
          var viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0;
          var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
          var left = triggerRect.right - menuWidth;
          left = clampNumber(left, 8, Math.max(8, viewportWidth - menuWidth - 8));
          var top = triggerRect.bottom + 8;
          if (top + menuHeight > viewportHeight - 8) {
            top = triggerRect.top - menuHeight - 8;
          }
          top = clampNumber(top, 8, Math.max(8, viewportHeight - menuHeight - 8));
          menu.style.left = String(Math.round(left)) + "px";
          menu.style.top = String(Math.round(top)) + "px";
          menu.style.right = "auto";
        }

        function positionFlyoutMenu(menu, trigger) {
          if (!menu || !trigger || menu.classList.contains("hidden")) return;
          var triggerRect = trigger.getBoundingClientRect();
          var menuWidth = menu.offsetWidth || 152;
          var menuHeight = menu.offsetHeight || 220;
          var viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0;
          var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
          var gap = 8;
          var left = triggerRect.right + gap;
          if (left + menuWidth > viewportWidth - gap) {
            left = triggerRect.left - menuWidth - gap;
          }
          left = clampNumber(left, gap, Math.max(gap, viewportWidth - menuWidth - gap));
          var top = clampNumber(triggerRect.top, gap, Math.max(gap, viewportHeight - menuHeight - gap));
          menu.style.left = String(Math.round(left)) + "px";
          menu.style.top = String(Math.round(top)) + "px";
          menu.style.right = "auto";
        }

        function positionLibraryKeywordMenu() {
          if (!libraryKeywordPanel || !libraryMenu || !btnLibraryMenuKeywordToggle) return;
          if (libraryKeywordPanel.classList.contains("hidden")) return;
          var menuRect = libraryMenu.getBoundingClientRect();
          var rowRect = btnLibraryMenuKeywordToggle.getBoundingClientRect();
          var menuWidth = libraryKeywordPanel.offsetWidth || 152;
          var menuHeight = libraryKeywordPanel.offsetHeight || 180;
          var viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0;
          var viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
          var gap = 8;
          var left = menuRect.right + gap;
          if (left + menuWidth > viewportWidth - gap) {
            left = menuRect.left - menuWidth - gap;
          }
          left = clampNumber(left, gap, Math.max(gap, viewportWidth - menuWidth - gap));
          var top = clampNumber(rowRect.top, gap, Math.max(gap, viewportHeight - menuHeight - gap));
          libraryKeywordPanel.style.left = String(Math.round(left)) + "px";
          libraryKeywordPanel.style.top = String(Math.round(top)) + "px";
          libraryKeywordPanel.style.right = "auto";
        }

        function closeToolbarMenus() {
          toolbarMenus.forEach(function (menu) {
            menu.classList.add("hidden");
          });
          toolbarMenuButtons.forEach(function (btn) {
            btn.setAttribute("aria-expanded", "false");
          });
          closeToolSubmenus();
        }

        function setTokenPanel(open) {
          if (!tokenPanel) return;
          tokenPanel.classList.toggle("hidden", !open);
          if (btnTokenToggle) btnTokenToggle.setAttribute("aria-expanded", open ? "true" : "false");
          if (open) {
            setDicePanel(false);
            positionFlyoutMenu(tokenPanel, btnTokenToggle);
          }
        }

        function setDicePanel(open) {
          if (!dicePanel) return;
          dicePanel.classList.toggle("hidden", !open);
          if (btnDiceToggle) btnDiceToggle.setAttribute("aria-expanded", open ? "true" : "false");
          if (open) {
            setTokenPanel(false);
            positionFlyoutMenu(dicePanel, btnDiceToggle);
          }
        }

        function closeToolSubmenus() {
          if (tokenPanel) tokenPanel.classList.add("hidden");
          if (dicePanel) dicePanel.classList.add("hidden");
          if (btnTokenToggle) btnTokenToggle.setAttribute("aria-expanded", "false");
          if (btnDiceToggle) btnDiceToggle.setAttribute("aria-expanded", "false");
        }

        function setToolbarMenu(menuName, trigger) {
          var name = String(menuName || "").trim();
          var shouldOpen = !!name;
          if (shouldOpen) {
            setLibraryMenu(false);
            setCommandMenu(false);
            setCardInlineMenu(false);
          }
          toolbarMenus.forEach(function (menu) {
            menu.classList.toggle("hidden", String(menu.dataset.toolbarMenu || "") !== name);
          });
          toolbarMenuButtons.forEach(function (btn) {
            var active = shouldOpen && String(btn.dataset.toolbarMenuToggle || "") === name;
            btn.setAttribute("aria-expanded", active ? "true" : "false");
          });
          if (name !== "tools") closeToolSubmenus();
          if (shouldOpen) {
            positionToolbarMenu(toolbarMenuFor(name), trigger || toolbarTriggerFor(name));
          } else {
            closeToolSubmenus();
          }
        }

        function getZoneArray(zone) {
          return state.zones[zone] || null;
        }

        function libraryMenuOpen() {
          return !!libraryMenu && !libraryMenu.classList.contains("hidden");
        }

        function commandMenuOpen() {
          return !!commandMenu && !commandMenu.classList.contains("hidden");
        }

        function setLibraryMenu(open) {
          if (!libraryMenu) return;
          if (open) {
            closeToolbarMenus();
            setCommandMenu(false);
            setCardInlineMenu(false);
            libraryMenu.classList.remove("hidden");
            positionInlineMenu(libraryMenu, btnLibraryMenuToggle);
          } else {
            libraryMenu.classList.add("hidden");
            setLibraryKeywordPanel(false);
            setLibraryRevealUntilPanel(false);
          }
        }

        function setCommandMenu(open) {
          if (!commandMenu) return;
          if (open) {
            closeToolbarMenus();
            setLibraryMenu(false);
            setCardInlineMenu(false);
            commandMenu.classList.remove("hidden");
            positionInlineMenu(commandMenu, btnCommandMenuToggle);
          } else {
            commandMenu.classList.add("hidden");
          }
        }

        function setLibraryKeywordPanel(open) {
          if (!libraryKeywordPanel) return;
          libraryKeywordPanel.classList.toggle("hidden", !open);
          if (btnLibraryMenuKeywordToggle) {
            btnLibraryMenuKeywordToggle.setAttribute("aria-expanded", open ? "true" : "false");
          }
          if (!open) setLibraryRevealUntilPanel(false);
          if (libraryMenuOpen()) {
            positionLibraryKeywordMenu();
          }
        }

        function setLibraryRevealUntilPanel(open) {
          if (!libraryRevealUntilPanel) return;
          libraryRevealUntilPanel.classList.toggle("hidden", !open);
          if (btnLibraryMenuRevealUntilToggle) {
            btnLibraryMenuRevealUntilToggle.setAttribute("aria-expanded", open ? "true" : "false");
          }
          if (libraryMenuOpen()) {
            positionFlyoutMenu(libraryRevealUntilPanel, btnLibraryMenuRevealUntilToggle);
          }
        }

        function currentLookMode() {
          var mode = String(state.scryMode || "scry");
          if (mode === "surveil" || mode === "reveal") return mode;
          return "scry";
        }

        function setScryOverlay(open) {
          if (!scryOverlay) return;
          if (open) {
            setLibraryMenu(false);
            closeToolbarMenus();
            var mode = currentLookMode();
            var count = Array.isArray(state.scryCards) ? state.scryCards.length : 0;
            var titlePrefix = "Scry ";
            if (mode === "surveil") titlePrefix = "Surveil ";
            if (mode === "reveal") titlePrefix = "Reveal ";
            if (scryTitleEl) scryTitleEl.textContent = titlePrefix + String(count);
            if (scryCountEl) scryCountEl.textContent = String(count) + " card" + (count === 1 ? "" : "s");
            if (scryHintEl) {
              if (mode === "surveil") {
                scryHintEl.textContent = "Drag cards between the top row and graveyard, then arrange each row left to right.";
              } else if (mode === "reveal") {
                scryHintEl.textContent = "Review each revealed card, then move it to a zone or the bottom of your library.";
              } else {
                scryHintEl.textContent = "Drag cards between the top row and bottom box, then arrange each row left to right.";
              }
            }
            if (btnScryCancel) {
              btnScryCancel.textContent = mode === "reveal" ? "Close" : "Cancel";
              btnScryCancel.classList.toggle("hidden", mode === "reveal");
            }
            if (btnScryFinish) {
              if (mode === "surveil") {
                btnScryFinish.textContent = "Finish Surveil";
              } else if (mode === "reveal") {
                btnScryFinish.textContent = "Close";
              } else {
                btnScryFinish.textContent = "Finish Scry";
              }
            }
          }
          scryOverlay.classList.toggle("hidden", !open);
          if (open) {
            renderScryCards();
          }
        }

        function scryOverlayOpen() {
          return !!scryOverlay && !scryOverlay.classList.contains("hidden");
        }

        function setLibrarySearchOverlay(open) {
          if (!librarySearchOverlay) return;
          if (open) {
            setLibraryMenu(false);
            closeToolbarMenus();
            state.librarySearchQuery = "";
            if (librarySearchInput) librarySearchInput.value = "";
          }
          librarySearchOverlay.classList.toggle("hidden", !open);
          if (open) {
            renderLibrarySearchCards();
            if (librarySearchInput) librarySearchInput.focus();
          }
        }

        function librarySearchOverlayOpen() {
          return !!librarySearchOverlay && !librarySearchOverlay.classList.contains("hidden");
        }

        function countPickerOpen() {
          return !!countPickerOverlay && !countPickerOverlay.classList.contains("hidden");
        }

        function customCountOpen() {
          return !!customCountOverlay && !customCountOverlay.classList.contains("hidden");
        }

        function countPickerLabel(action) {
          if (action === "draw") return "Draw";
          if (action === "reveal") return "Reveal";
          if (action === "scry") return "Scry";
          if (action === "surveil") return "Surveil";
          if (action === "mill") return "Mill";
          if (action === "discover") return "Discover";
          if (action === "cascade") return "Cascade";
          if (action === "copy-card") return "Create Copies";
          if (action === "counter-plus-x") return "+X/+X";
          if (action === "counter-minus-x") return "-X/-X";
          return "Choose";
        }

        function countPickerOptionLabel(action, count) {
          if (action === "reveal") return "Reveal " + String(count);
          if (action === "scry") return "Scry " + String(count);
          if (action === "surveil") return "Surveil " + String(count);
          if (action === "discover") return "Discover " + String(count);
          if (action === "cascade") return "Cascade " + String(count);
          if (action === "counter-plus-x") return "+" + String(count) + "/+" + String(count);
          if (action === "counter-minus-x") return "-" + String(count) + "/-" + String(count);
          return String(count);
        }

        function renderCountPicker() {
          if (!countPickerOptions) return;
          var action = String(state.countPickerAction || "");
          var label = countPickerLabel(action);
          if (countPickerTitle) {
            if (action === "counter-plus-x" || action === "counter-minus-x") {
              countPickerTitle.textContent = (action === "counter-minus-x" ? "-X/-X" : "+X/+X") + " Counter";
            } else if (action === "discover") {
              countPickerTitle.textContent = "Discover Value";
            } else if (action === "copy-card") {
              countPickerTitle.textContent = "Create Copies";
            } else {
              countPickerTitle.textContent = action === "cascade" ? "Cascade Value" : label + " X";
            }
          }
          if (countPickerHint) {
            if (action === "counter-plus-x" || action === "counter-minus-x") {
              countPickerHint.textContent = "Choose X to " + (action === "counter-minus-x" ? "subtract" : "add") + " that much power and toughness from the selected permanent.";
            } else if (action === "cascade") {
              countPickerHint.textContent = "Choose the triggering spell's mana value. The first revealed nonland card with lesser mana value goes to exile.";
            } else if (action === "discover") {
              countPickerHint.textContent = "Choose the discover value. The first revealed nonland card with equal or lesser mana value goes to hand.";
            } else if (action === "copy-card") {
              countPickerHint.textContent = "Choose how many copies to create on the battlefield.";
            } else if (action === "reveal") {
              countPickerHint.textContent = "Choose how many cards to reveal from the top of your library.";
            } else if (action === "scry") {
              countPickerHint.textContent = "Choose how many cards to look at from the top of your library.";
            } else if (action === "surveil") {
              countPickerHint.textContent = "Choose how many cards to look at from the top of your library.";
            } else {
              countPickerHint.textContent = "Choose how many cards to " + label.toLowerCase() + ".";
            }
          }
          if (btnCountPickerCustom) btnCountPickerCustom.textContent = "Custom value";

          countPickerOptions.innerHTML = "";
          countPickerOptions.classList.toggle("pt-count-picker-options--labeled",
            action === "reveal" || action === "scry" || action === "surveil" || action === "discover" || action === "cascade" || action === "copy-card" || action === "counter-plus-x" || action === "counter-minus-x");
          for (var i = 1; i <= 10; i++) {
            (function (count) {
              var btn = document.createElement("button");
              btn.type = "button";
              btn.className = "pt-count-picker-option";
              btn.textContent = countPickerOptionLabel(action, count);
              btn.addEventListener("click", function () {
                performLibraryCountAction(action, count);
              });
              countPickerOptions.appendChild(btn);
            })(i);
          }
        }

        function setCountPickerOverlay(open, action) {
          if (!countPickerOverlay) return;
          if (open) {
            state.countPickerAction = String(action || "").trim();
            setLibraryMenu(false);
            closeToolbarMenus();
            renderCountPicker();
          }
          countPickerOverlay.classList.toggle("hidden", !open);
        }

        function setCustomCountOverlay(open) {
          if (!customCountOverlay) return;
          customCountOverlay.classList.toggle("hidden", !open);
          if (open) {
            var label = countPickerLabel(state.countPickerAction);
            if (customCountTitle) {
              if (state.countPickerAction === "counter-plus-x" || state.countPickerAction === "counter-minus-x") {
                customCountTitle.textContent = (state.countPickerAction === "counter-minus-x" ? "-X/-X" : "+X/+X") + " Counter";
              } else if (state.countPickerAction === "discover") {
                customCountTitle.textContent = "Discover Value";
              } else if (state.countPickerAction === "copy-card") {
                customCountTitle.textContent = "Create Copies";
              } else {
                customCountTitle.textContent = state.countPickerAction === "cascade" ? "Cascade Value" : label + " X";
              }
            }
            if (customCountInput) {
              customCountInput.value = "1";
              customCountInput.focus();
              customCountInput.select();
            }
          }
        }

        function closeCountPickers() {
          setCountPickerOverlay(false);
          setCustomCountOverlay(false);
        }

        function historyOverlayOpen() {
          return !!historyOverlay && !historyOverlay.classList.contains("hidden");
        }

        function renderHistory() {
          if (!historyListEl) return;
          historyListEl.innerHTML = "";
          var history = Array.isArray(state.history) ? state.history : [];
          if (history.length === 0) {
            var empty = document.createElement("li");
            empty.className = "pt-empty-message pt-empty-message--padded";
            empty.textContent = "No actions yet.";
            historyListEl.appendChild(empty);
            return;
          }

          for (var i = 0; i < history.length; i++) {
            var entry = history[i] || {};
            var item = document.createElement("li");
            item.className = "pt-history-item";

            var turn = document.createElement("span");
            turn.className = "pt-history-item__turn";
            turn.textContent = "Turn " + String(entry.turn || 0);
            item.appendChild(turn);

            var text = document.createElement("span");
            text.className = "pt-history-item__text";
            text.textContent = String(entry.text || "");
            item.appendChild(text);

            historyListEl.appendChild(item);
          }
          var scrollToLatest = function () {
            historyListEl.scrollTop = historyListEl.scrollHeight;
          };
          scrollToLatest();
          window.requestAnimationFrame(scrollToLatest);
        }

        function setHistoryOverlay(open) {
          if (!historyOverlay) return;
          if (open) {
            setLibraryMenu(false);
            setCardInlineMenu(false);
            closeToolbarMenus();
            renderHistory();
          }
          historyOverlay.classList.toggle("hidden", !open);
          if (open && btnHistoryClose) btnHistoryClose.focus();
        }

        function cardInlineMenuOpen() {
          return !!cardInlineMenu && !cardInlineMenu.classList.contains("hidden");
        }

        function cardInlinePrimaryAction(located) {
          if (!located || !located.card) return null;
          if (located.zone === "battlefield") {
            return {
              label: located.card.tapped ? "Untap" : "Tap",
              shortcut: "T"
            };
          }
          if (located.zone === "hand") {
            return { label: "Play", shortcut: "P" };
          }
          if (located.zone === "command") {
            return { label: "Cast", shortcut: "P" };
          }
          return null;
        }

        function syncCardInlineBattlefieldActions(available) {
          if (!cardInlineMenu) return;

          if (available) {
            if (btnCardInlineMenuCountersToggle && btnCardInlineMenuCountersToggle.parentElement !== cardInlineMenu) {
              cardInlineMenu.insertBefore(btnCardInlineMenuCountersToggle, btnCardInlineMenuDetails || null);
            }
            if (btnCardInlineMenuCopyToggle && btnCardInlineMenuCopyToggle.parentElement !== cardInlineMenu) {
              cardInlineMenu.insertBefore(btnCardInlineMenuCopyToggle, btnCardInlineMenuDetails || null);
            }
          } else {
            if (cardInlineMenuCountersPanel) cardInlineMenuCountersPanel.classList.add("hidden");
            if (cardInlineMenuCopyPanel) cardInlineMenuCopyPanel.classList.add("hidden");
            if (btnCardInlineMenuCountersToggle) {
              btnCardInlineMenuCountersToggle.setAttribute("aria-expanded", "false");
              if (btnCardInlineMenuCountersToggle.parentElement) {
                btnCardInlineMenuCountersToggle.parentElement.removeChild(btnCardInlineMenuCountersToggle);
              }
            }
            if (btnCardInlineMenuCopyToggle) {
              btnCardInlineMenuCopyToggle.setAttribute("aria-expanded", "false");
              if (btnCardInlineMenuCopyToggle.parentElement) {
                btnCardInlineMenuCopyToggle.parentElement.removeChild(btnCardInlineMenuCopyToggle);
              }
            }
          }

          [
            [btnCardInlineMenuCountersToggle, cardInlineMenuCountersPanel],
            [btnCardInlineMenuCopyToggle, cardInlineMenuCopyPanel]
          ].forEach(function (pair) {
            var btn = pair[0];
            var panel = pair[1];
            if (!btn) return;
            btn.classList.toggle("hidden", !available);
            btn.disabled = !available;
            btn.setAttribute("aria-hidden", available ? "false" : "true");
            btn.tabIndex = available ? 0 : -1;
            btn.setAttribute("aria-expanded", available && panel && !panel.classList.contains("hidden") ? "true" : "false");
          });
        }

        function renderCardInlineMenu() {
          var located = activeCard();
          var card = located ? located.card : null;
          if (!card || !getZoneArray(located.zone)) {
            setCardInlineMenu(false);
            return;
          }

          var primaryAction = cardInlinePrimaryAction(located);
          if (btnCardInlineMenuPrimary) {
            btnCardInlineMenuPrimary.classList.toggle("hidden", !primaryAction);
            btnCardInlineMenuPrimary.disabled = !primaryAction;
            btnCardInlineMenuPrimary.setAttribute("aria-hidden", primaryAction ? "false" : "true");
            btnCardInlineMenuPrimary.tabIndex = primaryAction ? 0 : -1;
          }
          if (cardInlineMenuPrimaryLabel) {
            cardInlineMenuPrimaryLabel.textContent = primaryAction ? primaryAction.label : "";
          } else if (btnCardInlineMenuPrimary) {
            btnCardInlineMenuPrimary.textContent = primaryAction ? primaryAction.label : "";
          }
          if (cardInlineMenuPrimaryShortcut) {
            cardInlineMenuPrimaryShortcut.textContent = primaryAction ? primaryAction.shortcut : "";
          }
          cardInlineMoveButtons.forEach(function (btn) {
            var targetZone = String(btn.dataset.zone || "").trim();
            var isCommandButton = targetZone === "command";
            if (isCommandButton) btn.classList.toggle("hidden", !card.isCommander);
            btn.disabled = !targetZone || !canDrop(located.zone, targetZone, card) || targetZone === located.zone;
          });
          syncCardInlineBattlefieldActions(located.zone === "battlefield");
        }

        function setCardInlineMovePanel(open) {
          if (!cardInlineMenuMovePanel) return;
          if (open) {
            setCardInlineCounterPanel(false);
            setCardInlineCopyPanel(false);
          }
          cardInlineMenuMovePanel.classList.toggle("hidden", !open);
          if (btnCardInlineMenuMoveToggle) {
            btnCardInlineMenuMoveToggle.setAttribute("aria-expanded", open ? "true" : "false");
          }
          if (open) renderCardInlineMenu();
          if (cardInlineMenuOpen() && cardInlineMenuTrigger) {
            positionInlineMenu(cardInlineMenu, cardInlineMenuTrigger);
            positionFlyoutMenu(cardInlineMenuMovePanel, btnCardInlineMenuMoveToggle);
          }
        }

        function setCardInlineCounterPanel(open) {
          if (!cardInlineMenuCountersPanel) return;
          if (open) {
            setCardInlineMovePanel(false);
            setCardInlineCopyPanel(false);
          }
          cardInlineMenuCountersPanel.classList.toggle("hidden", !open);
          if (btnCardInlineMenuCountersToggle) {
            btnCardInlineMenuCountersToggle.setAttribute("aria-expanded", open ? "true" : "false");
          }
          if (open) renderCardInlineMenu();
          if (cardInlineMenuOpen() && cardInlineMenuTrigger) {
            positionInlineMenu(cardInlineMenu, cardInlineMenuTrigger);
            positionFlyoutMenu(cardInlineMenuCountersPanel, btnCardInlineMenuCountersToggle);
          }
        }

        function setCardInlineCopyPanel(open) {
          if (!cardInlineMenuCopyPanel) return;
          if (open) {
            setCardInlineMovePanel(false);
            setCardInlineCounterPanel(false);
          }
          cardInlineMenuCopyPanel.classList.toggle("hidden", !open);
          if (btnCardInlineMenuCopyToggle) {
            btnCardInlineMenuCopyToggle.setAttribute("aria-expanded", open ? "true" : "false");
          }
          if (open) renderCardInlineMenu();
          if (cardInlineMenuOpen() && cardInlineMenuTrigger) {
            positionInlineMenu(cardInlineMenu, cardInlineMenuTrigger);
            positionFlyoutMenu(cardInlineMenuCopyPanel, btnCardInlineMenuCopyToggle);
          }
        }

        function setCardInlineMenu(open, cardID, trigger) {
          if (!cardInlineMenu) return;
          if (open) {
            setLibraryMenu(false);
            setCommandMenu(false);
            closeToolbarMenus();
            if (typeof cardID === "number") state.activeCardID = cardID;
            cardInlineMenuTrigger = trigger || cardInlineMenuTrigger;
            cardInlineMenu.classList.remove("hidden");
            setCardInlineMovePanel(false);
            setCardInlineCounterPanel(false);
            setCardInlineCopyPanel(false);
            renderCardInlineMenu();
            positionInlineMenu(cardInlineMenu, cardInlineMenuTrigger);
          } else {
            cardInlineMenu.classList.add("hidden");
            cardInlineMenuTrigger = null;
            setCardInlineMovePanel(false);
            setCardInlineCounterPanel(false);
            setCardInlineCopyPanel(false);
          }
        }

        function setCardDetailOverlay(open) {
          if (!cardDetailOverlay) return;
          if (open) {
            setCardInlineMenu(false);
            closeToolbarMenus();
          }
          cardDetailOverlay.classList.toggle("hidden", !open);
          if (!open) return;
          renderCardDetail();
        }

        function cardDetailOverlayOpen() {
          return !!cardDetailOverlay && !cardDetailOverlay.classList.contains("hidden");
        }

        function requiredOpeningReturns() {
          var returns = Math.max(0, Math.floor(Number(state.mulligans || 0)) - 1);
          return Math.max(0, Math.min(returns, state.zones.hand.length));
        }

        function isOpeningSelected(cardID) {
          return Array.isArray(state.openingSelectionIDs) && state.openingSelectionIDs.indexOf(cardID) >= 0;
        }

        function toggleOpeningSelection(cardID) {
          if (state.phase !== "cleanup") return;
          var required = requiredOpeningReturns();
          if (required <= 0) return;

          var next = Array.isArray(state.openingSelectionIDs) ? state.openingSelectionIDs.slice() : [];
          var idx = next.indexOf(cardID);
          if (idx >= 0) {
            next.splice(idx, 1);
          } else if (next.length < required) {
            next.push(cardID);
          }

          state.openingSelectionIDs = next;
          render();
        }

        function findCard(cardID) {
          for (var i = 0; i < zoneOrder.length; i++) {
            var zoneName = zoneOrder[i];
            var zone = getZoneArray(zoneName);
            if (!zone) continue;
            for (var j = 0; j < zone.length; j++) {
              if (zone[j] && zone[j].id === cardID) {
                return {
                  zone: zoneName,
                  index: j,
                  card: zone[j]
                };
              }
            }
          }
          return null;
        }

        function activeCard() {
          if (typeof state.activeCardID !== "number") return null;
          var located = findCard(state.activeCardID);
          if (located) return located;
          var scryCards = Array.isArray(state.scryCards) ? state.scryCards : [];
          for (var i = 0; i < scryCards.length; i++) {
            if (scryCards[i] && scryCards[i].id === state.activeCardID) {
              return {
                zone: "scry",
                index: i,
                card: scryCards[i]
              };
            }
          }
          return null;
        }

        function drawCards(count) {
          var drew = [];
          for (var i = 0; i < count; i++) {
            if (state.zones.library.length === 0) break;
            var card = state.zones.library.pop();
            card.tapped = false;
            card.bfX = null;
            card.bfY = null;
            card.bfZ = null;
            state.zones.hand.push(card);
            drew.push(card);
          }
          return drew;
        }

        function clampBattlefieldPosition(x, y) {
          var el = zoneEls.battlefield;
          if (!el) {
            return {
              x: Math.max(0, x),
              y: Math.max(0, y)
            };
          }
          var maxX = Math.max(0, el.clientWidth - battlefieldCardWidth);
          var maxY = Math.max(0, el.clientHeight - battlefieldCardHeight);
          return {
            x: Math.min(maxX, Math.max(0, Math.round(x))),
            y: Math.min(maxY, Math.max(0, Math.round(y)))
          };
        }

        function battlefieldPositionConflicts(card, pos) {
          if (!card || !pos) return false;
          var bf = state.zones.battlefield || [];
          var tolerance = 12;
          for (var i = 0; i < bf.length; i++) {
            var other = bf[i];
            if (!other || other === card || other.id === card.id) continue;
            if (typeof other.bfX !== "number" || typeof other.bfY !== "number") continue;
            if (Math.abs(other.bfX - pos.x) < tolerance && Math.abs(other.bfY - pos.y) < tolerance) {
              return true;
            }
          }
          return false;
        }

        function findOpenBattlefieldPosition(card, pos) {
          if (!pos) pos = battlefieldSnapPosition(card);
          var preferred = clampBattlefieldPosition(pos.x, pos.y);
          if (!battlefieldPositionConflicts(card, preferred)) return preferred;

          var offsets = [
            [24, 18], [-24, 18], [48, 36], [-48, 36],
            [72, 54], [-72, 54], [24, -18], [-24, -18],
            [96, 72], [-96, 72]
          ];
          for (var i = 0; i < offsets.length; i++) {
            var offset = offsets[i];
            var shifted = clampBattlefieldPosition(preferred.x + offset[0], preferred.y + offset[1]);
            if (!battlefieldPositionConflicts(card, shifted)) return shifted;
          }

          var el = zoneEls.battlefield;
          var width = el ? el.clientWidth : 720;
          var height = el ? el.clientHeight : 420;
          var stepX = Math.max(24, battlefieldCardWidth + battlefieldSnapGap);
          var stepY = Math.max(34, battlefieldSnapRow);
          var columns = Math.max(1, Math.floor((width + battlefieldSnapGap) / stepX));
          var rows = Math.max(1, Math.floor((height + battlefieldSnapGap) / stepY));
          for (var row = 0; row < rows + 4; row++) {
            for (var col = 0; col < columns; col++) {
              var candidate = clampBattlefieldPosition(col * stepX, row * stepY);
              if (!battlefieldPositionConflicts(card, candidate)) return candidate;
            }
          }

          return preferred;
        }

        function battlefieldDropPosition(dropEvent) {
          if (!dropEvent || !zoneEls.battlefield) return null;
          var rect = zoneEls.battlefield.getBoundingClientRect();
          var x = dropEvent.clientX - rect.left - battlefieldCardWidth / 2;
          var y = dropEvent.clientY - rect.top - battlefieldCardHeight / 2;
          return clampBattlefieldPosition(x, y);
        }

        function landSnapPosition(card, dropEvent) {
          var base = battlefieldDropPosition(dropEvent);
          var el = zoneEls.battlefield;
          var maxY = 0;
          if (el) {
            maxY = Math.max(0, el.clientHeight - battlefieldCardHeight - 2);
          }

          var x = 0;
          if (base) {
            x = base.x;
          } else {
            var lands = [];
            var bf = state.zones.battlefield;
            for (var i = 0; i < bf.length; i++) {
              if (isLandCard(bf[i])) lands.push(bf[i]);
            }
            var idx = lands.indexOf(card);
            if (idx < 0) idx = Math.max(0, lands.length - 1);
            x = idx * (battlefieldCardWidth + battlefieldSnapGap);
          }
          return clampBattlefieldPosition(x, maxY);
        }

        function battlefieldSnapPosition(card) {
          var bf = state.zones.battlefield;
          var idx = bf.indexOf(card);
          if (idx < 0) idx = Math.max(0, bf.length - 1);

          var width = zoneEls.battlefield ? zoneEls.battlefield.clientWidth : 720;
          var columns = Math.max(1, Math.floor((width + battlefieldSnapGap) / (battlefieldCardWidth + battlefieldSnapGap)));
          var col = idx % columns;
          var row = Math.floor(idx / columns);
          var x = col * (battlefieldCardWidth + battlefieldSnapGap);
          var y = row * battlefieldSnapRow;
          return clampBattlefieldPosition(x, y);
        }

        function setBattlefieldPosition(card, dropEvent, preferDropPoint) {
          if (!card) return;

          if (isLandCard(card)) {
            var landPos = findOpenBattlefieldPosition(card, landSnapPosition(card, preferDropPoint ? dropEvent : null));
            card.bfX = landPos.x;
            card.bfY = landPos.y;
            return;
          }

          var pos = null;
          if (preferDropPoint) {
            pos = battlefieldDropPosition(dropEvent);
          }
          if (!pos) {
            pos = battlefieldSnapPosition(card);
          }
          pos = findOpenBattlefieldPosition(card, pos);
          card.bfX = pos.x;
          card.bfY = pos.y;
        }

        function battlefieldCardByID(cardID) {
          var bf = state.zones.battlefield || [];
          for (var i = 0; i < bf.length; i++) {
            if (bf[i] && bf[i].id === cardID) return bf[i];
          }
          return null;
        }

        function copyGroupSourceID(card) {
          if (!card) return null;
          if (typeof card.copySourceID === "number") return card.copySourceID;
          return typeof card.id === "number" ? card.id : null;
        }

        function positionCopyNearSource(copy, sourceCard, sequenceIndex) {
          if (!copy || !sourceCard) {
            setBattlefieldPosition(copy, null, false);
            return;
          }
          if (typeof sourceCard.bfX !== "number" || typeof sourceCard.bfY !== "number") {
            setBattlefieldPosition(sourceCard, null, false);
          }

          var sourceID = copyGroupSourceID(sourceCard);
          if (sourceID != null) copy.copySourceID = sourceID;
          var copyIndex = Math.max(0, Math.floor(Number(sequenceIndex || 0))) + 1;
          if (!sourceCard.copySourceID && sourceID != null) {
            var bf = state.zones.battlefield || [];
            var existing = 0;
            for (var i = 0; i < bf.length; i++) {
              if (bf[i] && bf[i] !== copy && bf[i].copySourceID === sourceID) existing += 1;
            }
            copyIndex = existing + 1;
          }

          var pos = clampBattlefieldPosition(
            Number(sourceCard.bfX || 0) + copyIndex * 26,
            Number(sourceCard.bfY || 0) + 34
          );
          copy.bfX = pos.x;
          copy.bfY = pos.y;
          copy.bfZ = Math.floor(Number(sourceCard.bfZ || 80)) + copyIndex;
        }

        function battlefieldGroups() {
          var bf = state.zones.battlefield || [];
          var groups = [];
          var groupByKey = {};
          function ensureGroup(key, root) {
            if (!groupByKey[key]) {
              groupByKey[key] = { key: key, root: root || null, cards: [] };
              groups.push(groupByKey[key]);
            } else if (root && !groupByKey[key].root) {
              groupByKey[key].root = root;
            }
            return groupByKey[key];
          }

          for (var i = 0; i < bf.length; i++) {
            var card = bf[i];
            if (!card || card.copySourceID) continue;
            ensureGroup("card:" + String(card.id), card).cards.push(card);
          }
          for (var j = 0; j < bf.length; j++) {
            var copy = bf[j];
            if (!copy || !copy.copySourceID) continue;
            var key = "card:" + String(copy.copySourceID);
            ensureGroup(key, battlefieldCardByID(copy.copySourceID)).cards.push(copy);
          }
          return groups;
        }

        function representativeGroupCard(group) {
          if (!group || !Array.isArray(group.cards) || group.cards.length === 0) return null;
          return group.root || group.cards[0];
        }

        function battlefieldGroupWidth(group, fanX) {
          var count = group && Array.isArray(group.cards) ? group.cards.length : 0;
          return battlefieldCardWidth + Math.max(0, count - 1) * fanX;
        }

        function battlefieldGroupCategory(group) {
          var card = representativeGroupCard(group);
          if (!card) return "side";
          if (card.isToken) return "side";
          if (isLandCard(card)) return "land";
          if (isCreatureCard(card)) return "creature";
          if (isEnchantmentCard(card) || isArtifactCard(card) || isPlaneswalkerCard(card)) return "side";
          return "side";
        }

        function landTypeInfo(card) {
          var typeLine = String(card && card.typeLine || "");
          var types = ["Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes"];
          var found = [];
          for (var i = 0; i < types.length; i++) {
            if (new RegExp("\\b" + types[i] + "\\b", "i").test(typeLine)) found.push(types[i]);
          }
          if (found.length === 0) {
            return {
              key: "Nonbasic " + String(card && card.name || "Land"),
              order: 20,
              label: String(card && card.name || "Land")
            };
          }
          return {
            key: found.join("/"),
            order: types.indexOf(found[0]),
            label: found.join("/")
          };
        }

        function sideGroupOrder(group) {
          var card = representativeGroupCard(group);
          if (!card) return 50;
          if (isEnchantmentCard(card)) return 10;
          if (card.isToken) return 30;
          if (isArtifactCard(card)) return 20;
          if (isPlaneswalkerCard(card)) return 40;
          return 50;
        }

        function groupName(group) {
          var card = representativeGroupCard(group);
          return String(card && card.name || "").toLowerCase();
        }

        function countBattlefieldRows(groups, width, gapX, fanX) {
          if (!groups || groups.length === 0) return 0;
          var rows = 1;
          var cursor = 0;
          for (var i = 0; i < groups.length; i++) {
            var groupWidth = battlefieldGroupWidth(groups[i], fanX);
            if (cursor > 0 && cursor + gapX + groupWidth > width) {
              rows += 1;
              cursor = groupWidth;
            } else {
              cursor += (cursor > 0 ? gapX : 0) + groupWidth;
            }
          }
          return rows;
        }

        function mergeLandGroupsByType(groups) {
          var merged = [];
          var byKey = {};
          if (!Array.isArray(groups)) return merged;
          for (var i = 0; i < groups.length; i++) {
            var group = groups[i];
            var info = landTypeInfo(representativeGroupCard(group));
            var key = String(info.key || "Land");
            if (!byKey[key]) {
              byKey[key] = {
                key: "land:" + key,
                root: representativeGroupCard(group),
                landInfo: info,
                cards: []
              };
              merged.push(byKey[key]);
            }
            Array.prototype.push.apply(byKey[key].cards, group.cards || []);
          }
          return merged;
        }

        function placeBattlefieldGroup(group, x, y, fanX, fanY, zBase, reverseZ) {
          var cards = group && Array.isArray(group.cards) ? group.cards : [];
          zBase = Math.floor(Number(zBase || 30));
          for (var i = 0; i < cards.length; i++) {
            var card = cards[i];
            if (!card) continue;
            var pos = clampBattlefieldPosition(
              x + i * fanX,
              y + (i > 0 ? fanY : 0)
            );
            card.bfX = pos.x;
            card.bfY = pos.y;
            card.bfZ = zBase + (reverseZ ? cards.length - i : i);
          }
        }

        function layoutBattlefieldRows(groups, region, options) {
          options = options || {};
          if (!groups || groups.length === 0) return;
          var gapX = options.gapX || 18;
          var rowGap = options.rowGap || 46;
          var fanX = options.fanX || 26;
          var fanY = options.fanY || 34;
          var rowHeight = Math.max(battlefieldCardHeight, Number(options.rowHeight || (battlefieldCardHeight + fanY)));
          var left = Number(region.left || 0);
          var width = Math.max(battlefieldCardWidth, Number(region.width || 0));
          var bottomUp = !!options.bottomUp;
          var cursorX = left;
          var cursorY = bottomUp
            ? Number(region.bottom || 0) - rowHeight
            : Number(region.top || 0);

          for (var i = 0; i < groups.length; i++) {
            var group = groups[i];
            var groupWidth = battlefieldGroupWidth(group, fanX);
            if (cursorX > left && cursorX + groupWidth > left + width) {
              cursorX = left;
              cursorY += bottomUp ? -(rowHeight + rowGap) : (rowHeight + rowGap);
            }
            placeBattlefieldGroup(group, cursorX, cursorY, fanX, fanY, 40 + i * 10, true);
            cursorX += groupWidth + gapX;
          }
        }

        function layoutLandGroups(groups, region, options) {
          options = options || {};
          if (!groups || groups.length === 0) return;
          var landFanX = options.fanX || 14;
          var landFanY = options.fanY || 5;
          var groupGap = options.gapX || 14;
          var left = Number(region.left || 0);
          var width = Math.max(battlefieldCardWidth, Number(region.width || 0));
          var y = Math.max(0, Number(region.bottom || 0) - battlefieldCardHeight);
          var totalCardsWidth = 0;
          for (var i = 0; i < groups.length; i++) {
            totalCardsWidth += battlefieldGroupWidth(groups[i], landFanX);
          }
          if (groups.length > 1) {
            groupGap = Math.min(groupGap, (width - totalCardsWidth) / (groups.length - 1));
            groupGap = Math.max(-Math.round(battlefieldCardWidth * 0.55), groupGap);
          }

          var cursorX = left;
          for (var g = 0; g < groups.length; g++) {
            var group = groups[g];
            placeBattlefieldGroup(group, cursorX, y, landFanX, landFanY, 120 + g * 10, true);
            cursorX += battlefieldGroupWidth(group, landFanX) + groupGap;
          }
        }

        function layoutSideGroups(groups, region, options) {
          options = options || {};
          if (!groups || groups.length === 0) return;
          var fanX = options.fanX || 18;
          var fanY = options.fanY || 20;
          var top = Number(region.top || 0);
          var bottom = Number(region.bottom || 0);
          var left = Number(region.left || 0);
          var available = Math.max(0, bottom - top - battlefieldCardHeight);
          var step = groups.length > 1 ? available / (groups.length - 1) : 0;
          step = Math.max(22, Math.min(44, step));
          for (var i = 0; i < groups.length; i++) {
            placeBattlefieldGroup(groups[i], left, top + i * step, fanX, fanY, 220 + i * 10, false);
          }
        }

        function organizeBattlefield(options) {
          options = options || {};
          var bf = state.zones.battlefield || [];
          if (bf.length === 0) {
            if (!options.silent) setStatus("Battlefield is empty.");
            render();
            return;
          }

          var groups = battlefieldGroups();
          var creatureGroups = [];
          var landGroups = [];
          var sideGroups = [];
          for (var i = 0; i < groups.length; i++) {
            var category = battlefieldGroupCategory(groups[i]);
            if (category === "land") {
              landGroups.push(groups[i]);
            } else if (category === "creature") {
              creatureGroups.push(groups[i]);
            } else {
              sideGroups.push(groups[i]);
            }
          }

          creatureGroups.sort(function (a, b) {
            return groupName(a).localeCompare(groupName(b));
          });
          landGroups.sort(function (a, b) {
            var ai = landTypeInfo(representativeGroupCard(a));
            var bi = landTypeInfo(representativeGroupCard(b));
            if (ai.order !== bi.order) return ai.order - bi.order;
            if (ai.key !== bi.key) return ai.key.localeCompare(bi.key);
            return groupName(a).localeCompare(groupName(b));
          });
          landGroups = mergeLandGroupsByType(landGroups);
          sideGroups.sort(function (a, b) {
            var ao = sideGroupOrder(a);
            var bo = sideGroupOrder(b);
            if (ao !== bo) return ao - bo;
            return groupName(a).localeCompare(groupName(b));
          });

          var fieldWidth = zoneEls.battlefield ? zoneEls.battlefield.clientWidth : 720;
          var fieldHeight = zoneEls.battlefield ? zoneEls.battlefield.clientHeight : 420;
          var pad = 4;
          var gap = 22;
          var gapX = 18;
          var rowGap = 10;
          var fanX = 26;
          var fanY = 34;
          var rowHeight = battlefieldCardHeight + 8;
          var sideWidth = sideGroups.length > 0
            ? Math.min(Math.max(168, Math.round(fieldWidth * 0.3)), Math.max(0, fieldWidth - 260))
            : 0;
          var mainWidth = Math.max(battlefieldCardWidth, fieldWidth - pad * 2 - sideWidth - (sideWidth > 0 ? gap : 0));
          var sideLeft = pad + mainWidth + gap;
          var landRows = landGroups.length > 0 ? 1 : 0;
          var landTop = Math.max(pad, fieldHeight - pad - battlefieldCardHeight);
          var creatureBottom = Math.max(pad + rowHeight, landTop - gap);

          layoutBattlefieldRows(creatureGroups, {
            left: pad,
            top: pad,
            width: mainWidth,
            bottom: creatureBottom
          }, {
            gapX: gapX,
            rowGap: rowGap,
            fanX: fanX,
            fanY: fanY,
            rowHeight: rowHeight
          });

          layoutLandGroups(landGroups, {
            left: pad,
            top: landTop,
            width: mainWidth,
            bottom: fieldHeight - pad
          }, {
            gapX: 14,
            fanX: 14,
            fanY: 5
          });

          if (sideGroups.length > 0) {
            layoutSideGroups(sideGroups, {
              left: sideLeft,
              top: pad,
              width: Math.max(battlefieldCardWidth, fieldWidth - sideLeft - pad),
              bottom: fieldHeight - pad
            }, {
              fanX: 18,
              fanY: 20
            });
          }

          if (!options.silent) setStatus("Battlefield organized by card type.");
          render();
        }

        function untapAllBattlefield() {
          var untapped = 0;
          var battlefield = state.zones.battlefield;
          for (var i = 0; i < battlefield.length; i++) {
            if (battlefield[i] && battlefield[i].tapped) {
              battlefield[i].tapped = false;
              untapped += 1;
            }
          }
          return untapped;
        }

        function resetManaPool() {
          state.mana = {
            w: 0,
            u: 0,
            b: 0,
            r: 0,
            g: 0,
            c: 0
          };
        }

        function startOpening(options) {
          options = options || {};
          state.phase = "opening";
          state.turn = 0;
          state.life = startingLifeTotal(deckFormat);
          state.commanderDamage = 0;
          state.commandTax = 0;
          resetManaPool();
          state.mulligans = 0;
          state.openingSelectionIDs = [];
          state.draggingCardID = null;
          state.draggingOpeningCard = false;
          state.draggingScryCard = false;
          state.hoveredCardID = null;
          state.pointerX = null;
          state.pointerY = null;
          state.activeCardID = null;
          state.scryCards = [];
          state.scryMode = "scry";
          state.scryDragPreview = null;
          state.revealBottomRandom = false;
          state.revealBottomedCount = 0;
          state.librarySearchQuery = "";
          state.libraryRevealed = false;
          state.countPickerAction = "";
          state.history = [];
          setLibraryMenu(false);
          closeToolbarMenus();
          setCardDetailOverlay(false);
          setScryOverlay(false);
          setLibrarySearchOverlay(false);
          closeCountPickers();

          state.zones.command = [];
          state.zones.library = shuffle(buildLibrary(librarySeed));
          state.zones.hand = [];
          state.zones.battlefield = [];
          state.zones.graveyard = [];
          state.zones.exile = [];

          if (commanderSeed && commanderSeed.name) {
            state.zones.command.push(createCardFromMeta(commanderSeed, true));
          }

          if (state.zones.library.length === 0) {
            state.phase = "empty";
            setStatus("Add cards to this deck before playtesting.");
            render();
            return;
          }

          drawCards(7);
          setStatus("Opening hand ready.");
          render();
        }

        function adjustLife(delta) {
          delta = Number(delta || 0);
          if (!Number.isFinite(delta) || delta === 0) return;
          state.life += delta;
          setStatus("Life total is " + state.life + ".");
          render();
        }

        function adjustCommanderDamage(delta) {
          delta = Number(delta || 0);
          if (!Number.isFinite(delta) || delta === 0) return;
          state.commanderDamage = Math.max(0, state.commanderDamage + delta);
          setStatus("Commander damage is " + state.commanderDamage + ".");
          render();
        }

        function adjustCommandTax(delta) {
          delta = Number(delta || 0);
          if (!Number.isFinite(delta) || delta === 0) return;
          state.commandTax = Math.max(0, state.commandTax + delta);
          setStatus("Commander tax is " + state.commandTax + ".");
          render();
        }

        function adjustMana(color, delta) {
          color = String(color || "").toLowerCase();
          if (!Object.prototype.hasOwnProperty.call(state.mana, color)) return;
          delta = Number(delta || 0);
          if (!Number.isFinite(delta) || delta === 0) return;
          state.mana[color] = Math.max(0, state.mana[color] + delta);
          setStatus((delta > 0 ? "Added " : "Spent ") + color.toUpperCase() + " mana.");
          render();
        }

        function keepOpening() {
          if (state.phase === "cleanup") {
            commitOpeningSelection();
            return;
          }
          if (state.phase !== "opening") return;
          var required = requiredOpeningReturns();
          if (required > 0) {
            state.phase = "cleanup";
            state.openingSelectionIDs = [];
            setStatus("");
            render();
            return;
          }
          state.phase = "play";
          state.turn = 1;
          setStatus("");
          render();
        }

        function mulliganOpening() {
          if (state.phase !== "opening") return;

          var oldSources = [];
          var oldHand = state.zones.hand || [];
          for (var i = 0; i < oldHand.length; i++) {
            if (oldHand[i]) oldSources.push(captureCardMotionSource(oldHand[i].id, "hand"));
          }
          var libraryRect = zoneAnchorRect("library");
          state.zones.library = state.zones.library.concat(state.zones.hand);
          state.zones.hand = [];
          shuffle(state.zones.library);
          state.mulligans += 1;
          var drawnCards = drawCards(7);

          setStatus("");
          render();
          animateOpeningMulligan(oldSources, drawnCards, libraryRect);
        }

        function commitOpeningSelection() {
          if (state.phase !== "cleanup") return;

          var required = requiredOpeningReturns();
          var selected = Array.isArray(state.openingSelectionIDs) ? state.openingSelectionIDs.slice() : [];
          if (selected.length !== required) {
            setStatus("");
            return;
          }

          var selectedMap = {};
          for (var i = 0; i < selected.length; i++) {
            selectedMap[selected[i]] = true;
          }

          var kept = [];
          var returned = [];
          var hand = state.zones.hand || [];
          for (var j = 0; j < hand.length; j++) {
            var card = hand[j];
            if (card && selectedMap[card.id]) {
              card.tapped = false;
              card.bfX = null;
              card.bfY = null;
              card.bfZ = null;
              returned.push(card);
            } else {
              kept.push(card);
            }
          }

          state.zones.hand = kept;
          state.zones.library = returned.concat(state.zones.library);
          state.openingSelectionIDs = [];
          state.phase = "play";
          state.turn = 1;
          setStatus("");
          render();
        }

        function drawOne() {
          drawFromLibrary(1);
        }

        function drawFromLibrary(count) {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          count = Math.max(1, Math.floor(Number(count || 1)));
          if (!Number.isFinite(count)) count = 1;
          var sourceRect = zoneAnchorRect("library");
          var drewCards = drawCards(count);
          var drew = drewCards.length;
          if (drew === 0) {
            setStatus("Library is empty.");
          } else {
            setStatus("Drew " + drew + " card" + (drew === 1 ? "." : "s."));
          }
          render();
          animateCardsFromRect(drewCards, sourceRect, "hand");
        }

        function millCards(count) {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          count = Math.max(1, Math.floor(Number(count || 1)));
          if (!Number.isFinite(count)) count = 1;
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }

          var sourceRect = zoneAnchorRect("library");
          var milledCards = [];
          var limit = Math.min(count, state.zones.library.length);
          for (var i = 0; i < limit; i++) {
            var card = state.zones.library.pop();
            if (!card) continue;
            card.tapped = false;
            card.bfX = null;
            card.bfY = null;
            card.bfZ = null;
            state.zones.graveyard.push(card);
            milledCards.push(card);
          }

          if (state.zones.library.length === 0) state.libraryRevealed = false;
          var milled = milledCards.length;
          setStatus("Milled " + milled + " card" + (milled === 1 ? "." : "s."));
          render();
          animateCardsFromRect(milledCards, sourceRect, "graveyard");
        }

        function shuffleLibrary() {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }
          shuffle(state.zones.library);
          state.libraryRevealed = false;
          setStatus("Library shuffled.");
          render();
          playLibraryShuffleAnimation();
        }

        function playLibraryShuffleAnimation() {
          var el = zoneEls.library;
          if (!el) return;
          el.classList.remove("pt-library-zone--shuffle");
          void el.offsetWidth;
          el.classList.add("pt-library-zone--shuffle");
          window.setTimeout(function () {
            el.classList.remove("pt-library-zone--shuffle");
          }, 560);
        }

        function toggleLibraryReveal() {
          setLibraryMenu(false);
          revealCards(1);
        }

        function cardManaValue(card) {
          var value = Number(card && card.manaValue || 0);
          return Number.isFinite(value) ? value : 0;
        }

        function isCreatureCard(card) {
          return /\bcreature\b/i.test(String(card && card.typeLine || ""));
        }

        function isBasicLandCard(card) {
          var typeLine = String(card && card.typeLine || "");
          return /\bbasic\b/i.test(typeLine) && /\bland\b/i.test(typeLine);
        }

        function keywordMatchLabel(kind) {
          if (kind === "basic-land") return "Basic Land";
          if (kind === "creature") return "Creature";
          return "Land";
        }

        function keywordPredicate(kind) {
          if (kind === "creature") return isCreatureCard;
          if (kind === "basic-land") return isBasicLandCard;
          return isLandCard;
        }

        function bottomRevealedCards(cards) {
          if (!Array.isArray(cards) || cards.length === 0) return;
          for (var i = 0; i < cards.length; i++) {
            if (!cards[i]) continue;
            cards[i].tapped = false;
            cards[i].bfX = null;
            cards[i].bfY = null;
            cards[i].bfZ = null;
          }
          state.zones.library = cards.concat(state.zones.library);
        }

        function revealForKeyword(kind) {
          setLibraryMenu(false);
          kind = String(kind || "").trim();
          var predicate = keywordPredicate(kind);
          var label = keywordMatchLabel(kind);
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }

          var sourceRect = zoneAnchorRect("library");
          var revealed = [];
          var hit = null;
          while (state.zones.library.length > 0) {
            var card = state.zones.library.pop();
            if (!card) continue;
            if (predicate(card)) {
              hit = card;
              break;
            }
            revealed.push(card);
          }

          bottomRevealedCards(revealed);
          if (hit) {
            hit.tapped = false;
            hit.bfX = null;
            hit.bfY = null;
            hit.bfZ = null;
            state.zones.hand.push(hit);
            setStatus("Revealed until " + label + ": " + hit.name + " to hand; bottomed " + revealed.length + " card" + (revealed.length === 1 ? "." : "s."));
          } else {
            setStatus("No " + label.toLowerCase() + " found; revealed library returned to bottom.");
          }
          if (state.zones.library.length === 0) state.libraryRevealed = false;
          render();
          if (hit) animateCardFromRect(hit.id, sourceRect, "hand");
        }

        function resolveLibraryValueKeyword(action, value) {
          action = String(action || "").trim();
          value = Math.max(1, Math.floor(Number(value || 1)));
          if (!Number.isFinite(value)) value = 1;
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }

          var sourceRect = zoneAnchorRect("library");
          var revealed = [];
          var hit = null;
          while (state.zones.library.length > 0) {
            var card = state.zones.library.pop();
            if (!card) continue;
            var nonland = !isLandCard(card);
            var manaValue = cardManaValue(card);
            var matches = action === "cascade"
              ? (nonland && manaValue < value)
              : (nonland && manaValue <= value);
            if (matches) {
              hit = card;
              break;
            }
            revealed.push(card);
          }

          bottomRevealedCards(revealed);
          if (hit) {
            hit.tapped = false;
            hit.bfX = null;
            hit.bfY = null;
            hit.bfZ = null;
            var targetZone = action === "cascade" ? "exile" : "hand";
            state.zones[targetZone].push(hit);
            var actionLabel = action === "cascade" ? "Cascaded into " : "Discovered ";
            var destination = action === "cascade" ? "exile" : "hand";
            setStatus(actionLabel + hit.name + " to " + destination + "; bottomed " + revealed.length + " card" + (revealed.length === 1 ? "." : "s."));
          } else {
            setStatus((action === "cascade" ? "Cascade" : "Discover") + " found no eligible card.");
          }
          if (state.zones.library.length === 0) state.libraryRevealed = false;
          render();
          if (hit) animateCardFromRect(hit.id, sourceRect, action === "cascade" ? "exile" : "hand");
        }

        function performLibraryCountAction(action, count) {
          action = String(action || "").trim();
          count = Math.max(1, Math.floor(Number(count || 1)));
          if (!Number.isFinite(count)) count = 1;
          closeCountPickers();
          if (action === "draw") {
            drawFromLibrary(count);
          } else if (action === "reveal") {
            revealCards(count);
          } else if (action === "scry") {
            startScry(count);
          } else if (action === "surveil") {
            startSurveil(count);
          } else if (action === "mill") {
            millCards(count);
          } else if (action === "discover" || action === "cascade") {
            resolveLibraryValueKeyword(action, count);
          } else if (action === "counter-plus-x") {
            applyActiveCardCounter("plusx", count, "+X/+X");
          } else if (action === "counter-minus-x") {
            applyActiveCardCounter("minusx", count, "-X/-X");
          } else if (action === "copy-card") {
            createActiveCardCopies(count);
          }
        }

        function revealCards(count) {
          count = Math.max(1, Math.floor(Number(count || 1)));
          if (!Number.isFinite(count)) count = 1;
          setLibraryMenu(false);
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }

          var limit = Math.min(count, state.zones.library.length);
          state.scryMode = "reveal";
          state.scryCards = [];
          state.scryDragPreview = null;
          state.revealBottomRandom = false;
          state.revealBottomedCount = 0;
          for (var i = 0; i < limit; i++) {
            var card = state.zones.library.pop();
            if (!card) continue;
            card.revealOriginalIndex = i;
            state.scryCards.push(card);
          }
          setStatus("Revealed top " + limit + " card" + (limit === 1 ? "." : "s."));
          render();
          setScryOverlay(true);
        }

        function cleanupLookCard(card) {
          if (!card) return;
          delete card.scryBottom;
          delete card.scryOriginalIndex;
          delete card.revealOriginalIndex;
          card.tapped = false;
          card.bfX = null;
          card.bfY = null;
          card.bfZ = null;
        }

        function returnRevealCardsToLibrary() {
          var cards = Array.isArray(state.scryCards) ? state.scryCards.slice() : [];
          if (cards.length === 0) return 0;
          cards.sort(function (a, b) {
            return Math.floor(Number(a && a.revealOriginalIndex || 0)) - Math.floor(Number(b && b.revealOriginalIndex || 0));
          });
          for (var i = cards.length - 1; i >= 0; i--) {
            cleanupLookCard(cards[i]);
            state.zones.library.push(cards[i]);
          }
          return cards.length;
        }

        function closeRevealLook(statusText) {
          state.scryCards = [];
          state.scryMode = "scry";
          state.draggingScryCard = false;
          state.scryDragPreview = null;
          state.revealBottomRandom = false;
          state.revealBottomedCount = 0;
          setScryOverlay(false);
          if (statusText) setStatus(statusText);
          render();
        }

        function removeRevealedCard(cardID) {
          var cards = Array.isArray(state.scryCards) ? state.scryCards : [];
          for (var i = 0; i < cards.length; i++) {
            if (cards[i] && cards[i].id === cardID) {
              return cards.splice(i, 1)[0];
            }
          }
          return null;
        }

        function moveRevealedCard(cardID, targetZone) {
          targetZone = String(targetZone || "").trim();
          var card = removeRevealedCard(cardID);
          if (!card) return;

          var sourceMotion = captureCardMotionSource(card.id, "library");
          cleanupLookCard(card);

          if (targetZone === "library-bottom") {
            var bottomed = Math.max(0, Math.floor(Number(state.revealBottomedCount || 0)));
            if (state.revealBottomRandom) {
              var insert = Math.floor(Math.random() * (bottomed + 1));
              state.zones.library.splice(insert, 0, card);
            } else {
              state.zones.library.unshift(card);
            }
            state.revealBottomedCount = bottomed + 1;
            setStatus(card.name + " moved to library bottom.");
            render();
            if (scryOverlayOpen()) renderScryCards();
            if (state.scryCards.length === 0) closeRevealLook();
            return;
          }

          var target = getZoneArray(targetZone);
          if (!target) {
            state.scryCards.push(card);
            renderScryCards();
            return;
          }
          target.push(card);
          if (targetZone === "battlefield") {
            setBattlefieldPosition(card, null, false);
          }
          setStatus(card.name + " moved to " + zoneLabel(targetZone) + ".");
          render();
          animateCardToZone(card.id, sourceMotion, targetZone);
          if (scryOverlayOpen()) renderScryCards();
          if (state.scryCards.length === 0) closeRevealLook();
        }

        function startScry(count) {
          return startLibraryLook(count, "scry");
        }

        function startSurveil(count) {
          return startLibraryLook(count, "surveil");
        }

        function startLibraryLook(count, mode) {
          count = Number(count || 0);
          mode = mode === "surveil" ? "surveil" : "scry";
          setLibraryMenu(false);
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!Number.isFinite(count) || count <= 0) return;
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }

          state.scryMode = mode;
          state.scryCards = [];
          state.scryDragPreview = null;

          var limit = Math.min(count, state.zones.library.length);
          for (var i = 0; i < limit; i++) {
            var card = state.zones.library.pop();
            if (!card) continue;
            card.scryBottom = false;
            card.scryOriginalIndex = i;
            state.scryCards.push(card);
          }

          render();
          setScryOverlay(true);
        }

        function cancelScry() {
          var mode = currentLookMode();
          if (mode === "reveal") {
            var returned = returnRevealCardsToLibrary();
            closeRevealLook(returned > 0 ? ("Returned " + returned + " revealed card" + (returned === 1 ? "" : "s") + " to library top.") : "");
            return;
          }
          if (!Array.isArray(state.scryCards) || state.scryCards.length === 0) {
            setScryOverlay(false);
            return;
          }
          var cards = state.scryCards.slice().sort(function (a, b) {
            return Math.floor(Number(a && a.scryOriginalIndex || 0)) - Math.floor(Number(b && b.scryOriginalIndex || 0));
          });
          for (var i = cards.length - 1; i >= 0; i--) {
            delete cards[i].scryBottom;
            delete cards[i].scryOriginalIndex;
            state.zones.library.push(cards[i]);
          }
          state.scryCards = [];
          state.scryMode = "scry";
          state.draggingScryCard = false;
          state.scryDragPreview = null;
          setScryOverlay(false);
          render();
        }

        function finishScry() {
          var cards = Array.isArray(state.scryCards) ? state.scryCards.slice() : [];
          var mode = currentLookMode();
          if (mode === "reveal") {
            var returned = returnRevealCardsToLibrary();
            closeRevealLook(returned > 0 ? ("Returned " + returned + " revealed card" + (returned === 1 ? "" : "s") + " to library top.") : "");
            return;
          }
          var keepTop = [];
          var moveAway = [];
          for (var i = 0; i < cards.length; i++) {
            if (cards[i] && cards[i].scryBottom) {
              moveAway.push(cards[i]);
            } else if (cards[i]) {
              keepTop.push(cards[i]);
            }
          }

          if (moveAway.length > 0) {
            for (var k = 0; k < moveAway.length; k++) {
              delete moveAway[k].scryBottom;
              delete moveAway[k].scryOriginalIndex;
              moveAway[k].tapped = false;
              moveAway[k].bfX = null;
              moveAway[k].bfY = null;
              moveAway[k].bfZ = null;
            }
            if (mode === "surveil") {
              Array.prototype.push.apply(state.zones.graveyard, moveAway);
            } else {
              state.zones.library = moveAway.slice().reverse().concat(state.zones.library);
            }
          }
          for (var j = keepTop.length - 1; j >= 0; j--) {
            delete keepTop[j].scryBottom;
            delete keepTop[j].scryOriginalIndex;
            state.zones.library.push(keepTop[j]);
          }

          state.scryCards = [];
          state.scryMode = "scry";
          state.draggingScryCard = false;
          state.scryDragPreview = null;
          setScryOverlay(false);
          setStatus(mode === "surveil" ? "Surveil finished." : "Scry finished.");
          render();
        }

        function nextTurn() {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          state.turn += 1;
          var untapped = untapAllBattlefield();
          resetManaPool();
          var sourceRect = zoneAnchorRect("library");
          var drewCards = drawCards(1);
          var drew = drewCards.length;
          var text = "Turn " + state.turn + ": ";
          if (untapped > 0) {
            text += "untapped " + untapped + " permanent(s), ";
          }
          text += drew > 0 ? "drew a card." : "no card to draw.";
          setStatus(text);
          render();
          animateCardsFromRect(drewCards, sourceRect, "hand");
        }

        function canDrop(sourceZone, targetZone, card) {
          if (!card) return false;

          if (sourceZone === targetZone) {
            return targetZone === "battlefield" || targetZone === "hand";
          }

          if (targetZone === "command" && !card.isCommander) return false;
          if (state.phase !== "play") return false;
          return true;
        }

        function handInsertIndex(clientX, draggingCardID) {
          if (!zoneEls.hand) return state.zones.hand.length;
          var nodes = zoneEls.hand.querySelectorAll("[data-zone-card='hand']");
          var insert = state.zones.hand.length;
          for (var i = 0; i < nodes.length; i++) {
            var node = nodes[i];
            var nodeID = Number(node.dataset.cardId || 0);
            if (nodeID === draggingCardID) continue;
            var rect = node.getBoundingClientRect();
            var mid = rect.left + rect.width / 2;
            if (clientX < mid) {
              var loc = findCard(nodeID);
              if (loc && loc.zone === "hand") {
                insert = loc.index;
              }
              break;
            }
          }
          return insert;
        }

        function reorderHandCard(cardID, dropEvent) {
          var found = findCard(cardID);
          if (!found || found.zone !== "hand") return;
          var hand = state.zones.hand;
          var from = found.index;
          var insert = hand.length;
          if (dropEvent && typeof dropEvent.clientX === "number") {
            insert = handInsertIndex(dropEvent.clientX, cardID);
          }

          var card = hand[from];
          hand.splice(from, 1);
          if (insert > from) insert -= 1;
          insert = Math.max(0, Math.min(insert, hand.length));
          hand.splice(insert, 0, card);
        }

        function openingHandInsertIndex(clientX, clientY, draggingCardID) {
          if (!openingHandEl) return state.zones.hand.length;
          var nodes = openingHandEl.querySelectorAll("[data-opening-card='true']");
          var insert = state.zones.hand.length;
          for (var i = 0; i < nodes.length; i++) {
            var node = nodes[i];
            var nodeID = Number(node.dataset.cardId || 0);
            if (nodeID === draggingCardID) continue;
            var rect = node.getBoundingClientRect();
            var beforeRow = clientY < rect.top + rect.height / 2;
            var beforeInRow = clientY <= rect.bottom && clientY >= rect.top && clientX < rect.left + rect.width / 2;
            if (beforeRow || beforeInRow) {
              var loc = findCard(nodeID);
              if (loc && loc.zone === "hand") insert = loc.index;
              break;
            }
          }
          return insert;
        }

        function reorderOpeningHandCard(cardID, dropEvent) {
          var found = findCard(cardID);
          if (!found || found.zone !== "hand") return;
          var hand = state.zones.hand;
          var from = found.index;
          var insert = hand.length;
          if (dropEvent && typeof dropEvent.clientX === "number" && typeof dropEvent.clientY === "number") {
            insert = openingHandInsertIndex(dropEvent.clientX, dropEvent.clientY, cardID);
          }

          var card = hand[from];
          hand.splice(from, 1);
          if (insert > from) insert -= 1;
          insert = Math.max(0, Math.min(insert, hand.length));
          hand.splice(insert, 0, card);
        }

        function moveCard(cardID, targetZone, dropEvent, options) {
          options = options || {};
          var located = findCard(cardID);
          if (!located) return;
          var sourceMotion = captureCardMotionSource(cardID, located.zone);

          if (!canDrop(located.zone, targetZone, located.card)) {
            if (state.phase !== "play") {
              setStatus("Complete keep/mulligan first.");
            }
            return;
          }

          if (located.zone === "hand" && targetZone === "hand") {
            reorderHandCard(cardID, dropEvent);
            render();
            return;
          }

          if (located.zone === "battlefield" && targetZone === "battlefield") {
            setBattlefieldPosition(located.card, dropEvent, true);
            var bf = state.zones.battlefield;
            bf.splice(located.index, 1);
            bf.push(located.card);
            setStatus(located.card.name + " repositioned.");
            render();
            animateCardToZone(located.card.id, sourceMotion, "battlefield");
            return;
          }

          var source = getZoneArray(located.zone);
          var target = getZoneArray(targetZone);
          if (!source || !target) return;

          source.splice(located.index, 1);
          var card = located.card;

          if (targetZone !== "battlefield") {
            card.tapped = false;
            card.bfX = null;
            card.bfY = null;
            card.bfZ = null;
          }

          if (targetZone === "library" && options.libraryPosition === "bottom") {
            target.unshift(card);
          } else {
            target.push(card);
          }

          if (targetZone === "battlefield") {
            setBattlefieldPosition(card, dropEvent, true);
          }

          var destinationLabel = zoneLabel(targetZone);
          if (targetZone === "library") {
            destinationLabel += options.libraryPosition === "bottom" ? " bottom" : " top";
          }
          setStatus(card.name + " moved to " + destinationLabel + ".");
          render();
          animateCardToZone(card.id, sourceMotion, targetZone);
        }

        function clearDropHighlights() {
          for (var i = 0; i < zoneOrder.length; i++) {
            var zoneName = zoneOrder[i];
            var el = zoneEls[zoneName];
            if (el) el.classList.remove("pt-drop-zone--active");
          }
        }

        function createCardFallback(card) {
          var fallback = document.createElement("div");
          fallback.className = "pt-card__fallback";
          fallback.textContent = (card && card.name) ? card.name : "Unknown card";
          return fallback;
        }

        function appendCardVisual(node, card) {
          if (!node) return;
          if (card && card.imageURI) {
            var img = document.createElement("img");
            img.className = "pt-card__image";
            img.src = card.imageURI;
            img.alt = card.name || "Card";
            img.addEventListener("error", function () {
              img.replaceWith(createCardFallback(card));
            }, { once: true });
            node.appendChild(img);
            return;
          }
          node.appendChild(createCardFallback(card));
        }

        function normalizeCardCounters(card) {
          if (!card) return {};
          if (!card.counters || typeof card.counters !== "object") card.counters = {};
          var counters = card.counters;
          var ptTotal = 0;
          if (counters.pt && Number.isFinite(Number(counters.pt.count))) {
            ptTotal += Math.floor(Number(counters.pt.count || 0));
          }
          [
            { key: "plus1", sign: 1 },
            { key: "plusx", sign: 1 },
            { key: "minus1", sign: -1 },
            { key: "minusx", sign: -1 }
          ].forEach(function (entry) {
            if (!counters[entry.key]) return;
            var amount = Math.max(0, Math.floor(Number(counters[entry.key].count || 0)));
            if (Number.isFinite(amount)) ptTotal += amount * entry.sign;
            delete counters[entry.key];
          });
          if (ptTotal === 0) {
            delete counters.pt;
          } else {
            counters.pt = { label: "Power/Toughness", count: ptTotal };
          }
          return card.counters;
        }

        function formatPowerToughnessCounter(value) {
          value = Math.floor(Number(value || 0));
          if (!Number.isFinite(value) || value === 0) return "";
          if (value > 0) return "+" + String(value) + "/+" + String(value);
          return String(value) + "/" + String(value);
        }

        function counterDisplayLabel(key, value) {
          value = value || {};
          if (key === "pt") {
            return formatPowerToughnessCounter(value.count);
          }
          var count = Math.max(0, Math.floor(Number(value.count || 0)));
          var label = String(value.label || "").trim();
          if (!label) label = String(key || "").trim();
          if (!label || count <= 0) return "";
          return count > 1 ? (label + " x" + count) : label;
        }

        function appendCardCounters(node, card, zoneName) {
          if (!node || zoneName !== "battlefield") return;
          var counters = normalizeCardCounters(card);
          var keys = Object.keys(counters).filter(function (key) {
            return counterDisplayLabel(key, counters[key]);
          });
          if (keys.length === 0) return;

          var wrap = document.createElement("div");
          wrap.className = "pt-card__counters";
          keys.forEach(function (key) {
            var badge = document.createElement("span");
            badge.className = "pt-card__counter";
            badge.textContent = counterDisplayLabel(key, counters[key]);
            wrap.appendChild(badge);
          });
          node.appendChild(wrap);
        }

        function applyCounterToCard(card, key, amount, label) {
          if (!card) return;
          key = String(key || "").trim();
          amount = Math.max(1, Math.floor(Number(amount || 1)));
          if (!key || !Number.isFinite(amount)) return;
          var counters = normalizeCardCounters(card);
          var ptDelta = 0;
          if (key === "plus1") ptDelta = 1;
          if (key === "plusx") ptDelta = amount;
          if (key === "minus1") ptDelta = -1;
          if (key === "minusx") ptDelta = -amount;
          if (ptDelta !== 0) {
            var pt = counters.pt || { label: "Power/Toughness", count: 0 };
            pt.count = Math.floor(Number(pt.count || 0)) + ptDelta;
            if (!Number.isFinite(pt.count) || pt.count === 0) {
              delete counters.pt;
              setStatus(card.name + " power/toughness counters are cleared.");
            } else {
              counters.pt = pt;
              setStatus(card.name + " power/toughness counters total " + formatPowerToughnessCounter(pt.count) + ".");
            }
            render();
            return;
          }
          var existing = counters[key] || { label: label, count: 0 };
          existing.label = label || existing.label || key;
          existing.count = Math.max(0, Math.floor(Number(existing.count || 0))) + amount;
          counters[key] = existing;
          var display = counterDisplayLabel(key, existing) || existing.label;
          setStatus("Added " + display + " counter to " + card.name + ".");
          render();
        }

        function applyActiveCardCounter(key, amount, label) {
          var located = activeCard();
          if (!located || located.zone !== "battlefield" || !located.card) {
            setStatus("Choose a battlefield card first.");
            return;
          }
          applyCounterToCard(located.card, key, amount, label);
        }

        function cloneCardForBattlefieldCopy(card) {
          var copy = createCardFromMeta({
            name: card.name,
            imageURI: card.imageURI,
            manaCost: card.manaCost,
            manaValue: card.manaValue,
            colors: card.colors,
            colorIdentity: card.colorIdentity,
            typeLine: card.typeLine,
            oracleText: card.oracleText,
            flavorText: card.flavorText,
            priceUSD: card.priceUSD,
            artist: card.artist,
            setCode: card.setCode,
            setName: card.setName,
            collectorNumber: card.collectorNumber
          }, false);
          copy.isToken = !!card.isToken;
          copy.isCopy = true;
          copy.copySourceID = copyGroupSourceID(card);
          return copy;
        }

        function createActiveCardCopies(count) {
          var located = activeCard();
          if (!located || located.zone !== "battlefield" || !located.card) {
            setStatus("Choose a battlefield card first.");
            return;
          }

          count = Math.max(1, Math.min(50, Math.floor(Number(count || 1))));
          if (!Number.isFinite(count)) count = 1;
          var sourceMotion = captureCardMotionSource(located.card.id, "battlefield");
          var sourceRect = sourceMotion && sourceMotion.rect ? sourceMotion.rect : zoneAnchorRect("battlefield");
          var copies = [];
          for (var i = 0; i < count; i++) {
            var copy = cloneCardForBattlefieldCopy(located.card);
            state.zones.battlefield.push(copy);
            positionCopyNearSource(copy, located.card, i);
            copies.push(copy);
          }

          setCardInlineMenu(false);
          setStatus("Created " + count + " cop" + (count === 1 ? "y" : "ies") + " of " + located.card.name + ".");
          render();
          animateCardsFromRect(copies, sourceRect, "battlefield");
        }

        function createCardNode(card, zoneName, isOpening, options) {
          options = options || {};
          var openingDrag = !!(isOpening && options.openingDrag);
          var scryDrag = !!(isOpening && options.scryDrag);
          var node = document.createElement("article");
          node.className = "pt-card";
          if (isOpening) node.classList.add("pt-card--opening");
          if (isOpening && isOpeningSelected(card.id)) node.classList.add("pt-card--selected");
          if (zoneName === "battlefield" && card.tapped) {
            node.classList.add("pt-card--tapped");
          }
          if (zoneName === "command" || zoneName === "graveyard" || zoneName === "exile") {
            node.classList.add("pt-card--stack");
          }
          node.setAttribute("draggable", (!isOpening || openingDrag || scryDrag) ? "true" : "false");
          node.dataset.cardId = String(card.id);
          node.dataset.zoneCard = zoneName;
          if (openingDrag) node.dataset.openingCard = "true";
          if (scryDrag) node.dataset.scryCard = "true";
          if (!isOpening) {
            node.tabIndex = 0;
            node.addEventListener("pointerenter", function () {
              state.hoveredCardID = card.id;
              try {
                node.focus({ preventScroll: true });
              } catch (err) {
                node.focus();
              }
            });
            node.addEventListener("mouseenter", function () {
              state.hoveredCardID = card.id;
            });
            node.addEventListener("pointerleave", function () {
              if (state.hoveredCardID === card.id) state.hoveredCardID = null;
            });
            node.addEventListener("mouseleave", function () {
              if (state.hoveredCardID === card.id) state.hoveredCardID = null;
            });
          }

          appendCardVisual(node, card);
          appendCardCounters(node, card, zoneName);

          if (!isOpening) {
            var menuBtn = document.createElement("button");
            menuBtn.type = "button";
            menuBtn.className = "pt-menu-trigger pt-card__menu";
            menuBtn.textContent = "...";
            menuBtn.setAttribute("aria-label", "Card menu");
            menuBtn.draggable = false;
            menuBtn.addEventListener("mousedown", function (e) {
              e.stopPropagation();
            });
            menuBtn.addEventListener("click", function (e) {
              e.preventDefault();
              e.stopPropagation();
              setCardInlineMenu(true, card.id, menuBtn);
            });
            node.appendChild(menuBtn);
          }

          if (!isOpening || openingDrag || scryDrag) {
            node.addEventListener("dragstart", function (e) {
              state.draggingCardID = card.id;
              state.draggingOpeningCard = openingDrag;
              state.draggingScryCard = scryDrag;
              startDragAvatar(node, e);
              if (e.dataTransfer) {
                e.dataTransfer.effectAllowed = "move";
                e.dataTransfer.setData("text/plain", String(card.id));
              }
            });

            node.addEventListener("dragend", function () {
              state.draggingCardID = null;
              state.draggingOpeningCard = false;
              state.draggingScryCard = false;
              state.scryDragPreview = null;
              clearDragAvatar();
              clearDropHighlights();
              if (scryDrag && scryOverlayOpen()) renderScryCards();
            });
          }

          if (isOpening) {
            if (scryDrag) {
              node.title = currentLookMode() === "surveil"
                ? "Drag to reorder or move to the graveyard."
                : "Drag to reorder or move to the bottom box.";
              node.addEventListener("click", function () {
                state.activeCardID = card.id;
                setCardDetailOverlay(true);
              });
              return node;
            }
            node.title = state.phase === "cleanup"
              ? "Click to select this card to put back."
              : "Click to view details.";
            node.addEventListener("click", function () {
              if (state.phase === "cleanup") {
                toggleOpeningSelection(card.id);
              } else {
                state.activeCardID = card.id;
                setCardDetailOverlay(true);
              }
            });
            return node;
          }

          if (zoneName === "hand") {
            node.title = "Click to play to battlefield. Drag to reorder or move.";
            node.addEventListener("click", function () {
              if (state.phase !== "play") return;
              moveCard(card.id, "battlefield");
            });
          } else if (zoneName === "command") {
            node.title = "Click to cast to battlefield. Drag to move zones.";
            node.addEventListener("click", function () {
              if (state.phase !== "play") return;
              moveCard(card.id, "battlefield");
            });
          } else if (zoneName === "battlefield") {
            node.title = "Click to tap/untap. Drag to reposition or move.";
            node.addEventListener("click", function () {
              card.tapped = !card.tapped;
              var bf = state.zones.battlefield;
              var idx = bf.indexOf(card);
              if (idx >= 0) {
                bf.splice(idx, 1);
                bf.push(card);
              }
              render();
            });
          } else {
            node.title = "Click to view details. Drag to move zones.";
            node.addEventListener("click", function () {
              if (state.phase !== "play") return;
              state.activeCardID = card.id;
              setCardDetailOverlay(true);
            });
          }

          return node;
        }

        function scryPileCards(pile) {
          var cards = Array.isArray(state.scryCards) ? state.scryCards : [];
          var wantsBottom = pile === "bottom";
          return cards.filter(function (card) {
            return !!(card && card.scryBottom) === wantsBottom;
          });
        }

        function captureScryFanRects() {
          var rects = {};
          if (!scryCardsEl || motionReduced()) return rects;
          var nodes = scryCardsEl.querySelectorAll(".pt-scry-fan-card[data-card-id]");
          for (var i = 0; i < nodes.length; i++) {
            var node = nodes[i];
            var cardID = String(node.dataset.cardId || "");
            if (!cardID || !node.getClientRects || node.getClientRects().length === 0) continue;
            rects[cardID] = rectSnapshot(node.getBoundingClientRect());
          }
          return rects;
        }

        function animateScryFanLayout(previousRects) {
          if (!scryCardsEl || motionReduced() || !previousRects) return;
          window.requestAnimationFrame(function () {
            var nodes = scryCardsEl.querySelectorAll(".pt-scry-fan-card[data-card-id]");
            for (var i = 0; i < nodes.length; i++) {
              (function (node) {
                var before = previousRects[String(node.dataset.cardId || "")];
                if (!before || !node.getClientRects || node.getClientRects().length === 0) return;
                var after = rectSnapshot(node.getBoundingClientRect());
                var dx = before.left - after.left;
                var dy = before.top - after.top;
                if (Math.abs(dx) < 1 && Math.abs(dy) < 1) return;

                node.style.transition = "none";
                node.style.transform = "translate3d(" + dx + "px, " + dy + "px, 0)";
                window.requestAnimationFrame(function () {
                  node.style.transition = "transform 170ms cubic-bezier(0.22, 1, 0.36, 1)";
                  node.style.transform = "";
                });
                window.setTimeout(function () {
                  node.style.transition = "";
                }, 210);
              })(nodes[i]);
            }
          });
        }

        function scryFanInsertIndex(fanEl, clientX, draggingCardID) {
          if (!fanEl) return 0;
          var nodes = fanEl.querySelectorAll(".pt-scry-fan-card");
          var insert = 0;
          for (var i = 0; i < nodes.length; i++) {
            var nodeID = Number(nodes[i].dataset.cardId || 0);
            if (nodeID === draggingCardID) continue;
            var rect = nodes[i].getBoundingClientRect();
            if (clientX < rect.left + rect.width / 2) {
              return insert;
            }
            insert += 1;
          }
          return insert;
        }

        function moveScryCard(cardID, pile, insertIndex, options) {
          options = options || {};
          var cards = Array.isArray(state.scryCards) ? state.scryCards : [];
          var top = [];
          var bottom = [];
          var moving = null;
          for (var i = 0; i < cards.length; i++) {
            var card = cards[i];
            if (!card) continue;
            if (card.id === cardID) {
              moving = card;
              continue;
            }
            if (card.scryBottom) {
              bottom.push(card);
            } else {
              top.push(card);
            }
          }
          if (!moving) return;
          var wantsBottom = pile === "bottom";
          var target = wantsBottom ? bottom : top;
          insertIndex = clampNumber(Math.floor(Number(insertIndex || 0)), 0, target.length);
          target.splice(insertIndex, 0, moving);

          var nextCards = top.concat(bottom);
          var currentKey = cards.map(function (card) {
            return String(card && card.id) + ":" + (!!(card && card.scryBottom) ? "bottom" : "top");
          }).join("|");
          var nextKey = nextCards.map(function (card) {
            var bottomed = card === moving ? wantsBottom : !!(card && card.scryBottom);
            return String(card && card.id) + ":" + (bottomed ? "bottom" : "top");
          }).join("|");
          if (currentKey === nextKey) return;

          var previousRects = options.animate === false ? null : captureScryFanRects();
          moving.scryBottom = wantsBottom;
          state.scryCards = nextCards;
          state.scryDragPreview = {
            cardID: cardID,
            pile: wantsBottom ? "bottom" : "top",
            insertIndex: insertIndex
          };
          renderScryCards();
          animateScryFanLayout(previousRects);
        }

        function bindScryLaneDrop(box, fan, pile) {
          if (!box || !fan) return;
          box.addEventListener("dragover", function (e) {
            if (!state.draggingScryCard || state.draggingCardID == null) return;
            e.preventDefault();
            box.classList.add("pt-drop-zone--active");
            var cardID = Number(state.draggingCardID);
            var insert = scryFanInsertIndex(fan, e.clientX, cardID);
            moveScryCard(cardID, pile, insert, { animate: true });
          });
          box.addEventListener("dragleave", function () {
            box.classList.remove("pt-drop-zone--active");
          });
          box.addEventListener("drop", function (e) {
            if (!state.draggingScryCard || state.draggingCardID == null) return;
            e.preventDefault();
            box.classList.remove("pt-drop-zone--active");
            var cardID = Number(state.draggingCardID);
            var insert = scryFanInsertIndex(fan, e.clientX, cardID);
            moveScryCard(cardID, pile, insert);
            state.draggingCardID = null;
            state.draggingScryCard = false;
            state.scryDragPreview = null;
            renderScryCards();
            clearDragAvatar();
          });
        }

        function renderScryFanLane(pile, title, hint, emptyText) {
          var cards = scryPileCards(pile);
          var lane = document.createElement("section");
          lane.className = "pt-scry-lane";

          var header = document.createElement("div");
          header.className = "pt-scry-lane__header";
          var titleEl = document.createElement("p");
          titleEl.className = "pt-scry-lane__title";
          titleEl.textContent = title;
          header.appendChild(titleEl);
          var hintEl = document.createElement("p");
          hintEl.className = "pt-scry-lane__hint";
          hintEl.textContent = hint;
          header.appendChild(hintEl);
          lane.appendChild(header);

          var box = document.createElement("div");
          box.className = "pt-scry-fan-box" + (pile === "bottom" ? " pt-scry-fan-box--bottom" : "");
          if (state.draggingScryCard && state.scryDragPreview && state.scryDragPreview.pile === pile) {
            box.classList.add("pt-drop-zone--active");
          }
          box.dataset.scryPile = pile;
          var fan = document.createElement("div");
          fan.className = "pt-scry-fan";

          if (cards.length === 0) {
            var empty = document.createElement("div");
            empty.className = "pt-scry-empty";
            empty.textContent = emptyText;
            fan.appendChild(empty);
          } else {
            for (var i = 0; i < cards.length; i++) {
              (function (card, index) {
                var wrap = document.createElement("div");
                wrap.className = "pt-scry-fan-card";
                wrap.dataset.cardId = String(card.id);
                wrap.style.marginLeft = index === 0 ? "0" : "-44px";
                wrap.style.zIndex = String(200 - index);
                var node = createCardNode(card, "scry", true, { scryDrag: true });
                if (state.draggingScryCard && card.id === state.draggingCardID) {
                  node.classList.add("pt-card--dragging-origin");
                }
                wrap.appendChild(node);
                fan.appendChild(wrap);
              })(cards[i], i);
            }
          }

          box.appendChild(fan);
          bindScryLaneDrop(box, fan, pile);
          lane.appendChild(box);
          return lane;
        }

        function renderRevealCards(cards) {
          var tools = document.createElement("div");
          tools.className = "pt-reveal-tools";
          var label = document.createElement("p");
          label.className = "pt-reveal-tools__label";
          label.textContent = "Put cards on bottom of library in random order?";
          tools.appendChild(label);

          var toggle = document.createElement("div");
          toggle.className = "pt-reveal-toggle";
          [
            { value: false, label: "No" },
            { value: true, label: "Yes" }
          ].forEach(function (option) {
            var btn = document.createElement("button");
            btn.type = "button";
            btn.className = "pt-reveal-toggle__option";
            btn.textContent = option.label;
            btn.setAttribute("aria-pressed", state.revealBottomRandom === option.value ? "true" : "false");
            btn.addEventListener("click", function () {
              state.revealBottomRandom = option.value;
              renderScryCards();
            });
            toggle.appendChild(btn);
          });
          tools.appendChild(toggle);
          scryCardsEl.appendChild(tools);

          var lane = document.createElement("section");
          lane.className = "pt-scry-lane";

          var header = document.createElement("div");
          header.className = "pt-scry-lane__header";
          var titleEl = document.createElement("p");
          titleEl.className = "pt-scry-lane__title";
          titleEl.textContent = "Revealed Cards";
          header.appendChild(titleEl);
          var hintEl = document.createElement("p");
          hintEl.className = "pt-scry-lane__hint";
          hintEl.textContent = "Leftmost is nearest the top of your library.";
          header.appendChild(hintEl);
          lane.appendChild(header);

          var grid = document.createElement("div");
          grid.className = "pt-reveal-grid";

          for (var i = 0; i < cards.length; i++) {
            (function (card) {
              var wrap = document.createElement("div");
              wrap.className = "pt-reveal-card";
              wrap.dataset.cardId = String(card.id);
              var node = createCardNode(card, "library", true);
              wrap.appendChild(node);

              var actions = document.createElement("div");
              actions.className = "pt-reveal-actions";
              [
                { label: "Hand", zone: "hand" },
                { label: "Graveyard", zone: "graveyard" },
                { label: "Exile", zone: "exile" },
                { label: "Battlefield", zone: "battlefield" },
                { label: "Bottom of Library", zone: "library-bottom" }
              ].forEach(function (action) {
                var btn = document.createElement("button");
                btn.type = "button";
                btn.className = "pt-reveal-action";
                btn.textContent = action.label;
                btn.addEventListener("click", function (e) {
                  e.preventDefault();
                  e.stopPropagation();
                  moveRevealedCard(card.id, action.zone);
                });
                actions.appendChild(btn);
              });
              wrap.appendChild(actions);
              grid.appendChild(wrap);
            })(cards[i]);
          }

          lane.appendChild(grid);
          scryCardsEl.appendChild(lane);
        }

        function renderSurveilCards(cards) {
          for (var i = 0; i < cards.length; i++) {
            (function (card, index) {
              var wrap = document.createElement("div");
              wrap.className = "pt-scry-card";

              var meta = document.createElement("div");
              meta.className = "pt-scry-card__meta";
              var order = document.createElement("span");
              order.textContent = "Top " + String(index + 1);
              meta.appendChild(order);
              var destination = document.createElement("span");
              destination.className = "pt-scry-card__destination";
              destination.textContent = card.scryBottom ? "Graveyard" : "Library";
              meta.appendChild(destination);
              wrap.appendChild(meta);

              var node = createCardNode(card, "hand", true);
              wrap.appendChild(node);

              var toggle = document.createElement("button");
              toggle.type = "button";
              toggle.className = "mt-btn mt-btn--secondary mt-btn--xs pt-scry-card__toggle " +
                (card.scryBottom ? "pt-scry-card__toggle--away" : "pt-scry-card__toggle--top");
              toggle.textContent = card.scryBottom ? "Move to Graveyard" : "Keep on Top";
              toggle.addEventListener("click", function () {
                card.scryBottom = !card.scryBottom;
                renderScryCards();
              });
              wrap.appendChild(toggle);

              scryCardsEl.appendChild(wrap);
            })(cards[i], i);
          }
        }

        function renderScryCards() {
          if (!scryCardsEl) return;
          scryCardsEl.innerHTML = "";

          var cards = Array.isArray(state.scryCards) ? state.scryCards : [];
          var mode = currentLookMode();
          if (cards.length === 0) {
            var empty = document.createElement("p");
            empty.className = "pt-empty-message pt-empty-message--centered";
            empty.textContent = mode === "surveil" ? "No cards to surveil." : (mode === "reveal" ? "No cards to reveal." : "No cards to scry.");
            scryCardsEl.appendChild(empty);
            return;
          }

          if (mode === "reveal") {
            renderRevealCards(cards);
            return;
          }

          if (mode === "surveil") {
            scryCardsEl.appendChild(renderScryFanLane(
              "top",
              "Top of Library",
              "Leftmost is the next card you will draw.",
              "Drop cards here to keep them on top."
            ));
            scryCardsEl.appendChild(renderScryFanLane(
              "bottom",
              "Graveyard",
              "Cards in this row move to graveyard when you finish.",
              "Drag cards here to put them in the graveyard."
            ));
            return;
          }

          scryCardsEl.appendChild(renderScryFanLane(
            "top",
            "Top of Library",
            "Leftmost is the next card you will draw.",
            "Drop cards here to keep them on top."
          ));
          scryCardsEl.appendChild(renderScryFanLane(
            "bottom",
            "Bottom of Library",
            "Left is closest to top; right is closest to bottom.",
            "Drag cards here to put them on the bottom."
          ));
        }

        function librarySearchMatches(card, query) {
          if (!query) return true;
          var haystack = [
            card && card.name,
            card && card.typeLine,
            card && card.oracleText,
            card && card.manaCost
          ].join(" ").toLowerCase();
          return haystack.indexOf(query) >= 0;
        }

        function renderLibrarySearchCards() {
          if (!librarySearchResultsEl) return;
          librarySearchResultsEl.innerHTML = "";

          var query = String(state.librarySearchQuery || "").trim().toLowerCase();
          var cards = (state.zones.library || []).slice().reverse();
          var matches = [];
          for (var i = 0; i < cards.length; i++) {
            if (librarySearchMatches(cards[i], query)) matches.push(cards[i]);
          }

          if (librarySearchCountEl) {
            var label = String(matches.length) + " of " + String(cards.length) + " card";
            if (cards.length !== 1) label += "s";
            librarySearchCountEl.textContent = label + " in library.";
          }

          if (matches.length === 0) {
            var empty = document.createElement("p");
            empty.className = "pt-empty-message pt-empty-message--centered";
            empty.textContent = cards.length === 0 ? "Library is empty." : "No matching cards.";
            librarySearchResultsEl.appendChild(empty);
            return;
          }

          for (var j = 0; j < matches.length; j++) {
            (function (card) {
              var wrap = document.createElement("div");
              wrap.className = "pt-library-search-card";

              var node = createCardNode(card, "library", true);
              wrap.appendChild(node);

              var actions = document.createElement("div");
              actions.className = "pt-library-search-actions";

              [
                { label: "Hand", zone: "hand" },
                { label: "Battlefield", zone: "battlefield" },
                { label: "Graveyard", zone: "graveyard" },
                { label: "Exile", zone: "exile" }
              ].forEach(function (action) {
                var btn = document.createElement("button");
                btn.type = "button";
                btn.className = "mt-btn mt-btn--secondary mt-btn--xs";
                btn.textContent = action.label;
                btn.addEventListener("click", function () {
                  moveCard(card.id, action.zone);
                  renderLibrarySearchCards();
                });
                actions.appendChild(btn);
              });

              wrap.appendChild(actions);
              librarySearchResultsEl.appendChild(wrap);
            })(matches[j]);
          }
        }

        function openLibrarySearch() {
          setLibraryMenu(false);
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            return;
          }
          if (!state.zones.library.length) {
            setStatus("Library is empty.");
            return;
          }
          setLibrarySearchOverlay(true);
        }

        function runActiveCardInlinePrimaryAction() {
          var located = activeCard();
          var card = located ? located.card : null;
          var primaryAction = cardInlinePrimaryAction(located);
          if (!card || !primaryAction) {
            setCardInlineMenu(false);
            return;
          }

          if (located.zone === "hand" || located.zone === "command") {
            setCardInlineMenu(false);
            moveCard(card.id, "battlefield");
            return;
          }

          card.tapped = !card.tapped;
          var bf = state.zones.battlefield;
          var idx = bf.indexOf(card);
          if (idx >= 0) {
            bf.splice(idx, 1);
            bf.push(card);
          }
          setStatus(card.name + (card.tapped ? " tapped." : " untapped."));
          setCardInlineMenu(false);
          render();
        }

        function cardElementFromPointer() {
          var pointerX = Number(state.pointerX);
          var pointerY = Number(state.pointerY);
          if (!Number.isFinite(pointerX) || !Number.isFinite(pointerY) || !document.elementFromPoint) return null;
          var target = document.elementFromPoint(pointerX, pointerY);
          return target && target.closest ? target.closest(".pt-card[data-zone-card]") : null;
        }

        function hoveredCardElement() {
          var current = document.querySelector(".pt-card[data-zone-card]:hover");
          if (current) return current;
          return cardElementFromPointer();
        }

        function hoveredZoneCard() {
          var hoverEl = hoveredCardElement();
          if (hoverEl) {
            var hoverID = Number(hoverEl.dataset.cardId || 0);
            if (hoverID) state.hoveredCardID = hoverID;
          }
          if (typeof state.hoveredCardID !== "number") {
            return cardInlineMenuOpen() ? activeCard() : null;
          }
          var located = findCard(state.hoveredCardID);
          if ((!located || !located.card) && cardInlineMenuOpen()) {
            located = activeCard();
          }
          if (!located || !located.card || !getZoneArray(located.zone)) return null;
          return located;
        }

        function cardMenuTrigger(cardID, zoneName) {
          var node = cardNodeFor(cardID, zoneName);
          if (!node) return null;
          return node.querySelector(".pt-card__menu") || node;
        }

        function openHoveredCardMenu(panelName) {
          var located = hoveredZoneCard();
          if (!located) return false;
          state.activeCardID = located.card.id;
          setCardInlineMenu(true, located.card.id, cardMenuTrigger(located.card.id, located.zone));
          panelName = String(panelName || "").trim();
          if (panelName === "move") {
            setCardInlineMovePanel(true);
          } else if (panelName === "counters" && located.zone === "battlefield") {
            setCardInlineCounterPanel(true);
          } else if (panelName === "copy" && located.zone === "battlefield") {
            setCardInlineCopyPanel(true);
          }
          return true;
        }

        function moveShortcutCard(located, targetZone, options) {
          if (!located || !located.card || !targetZone) return false;
          if (located.zone === targetZone) return true;
          if (targetZone === "command" && !located.card.isCommander) {
            setStatus("Only commanders can move to the command zone.");
            return true;
          }
          state.activeCardID = located.card.id;
          setCardInlineMenu(false);
          moveCard(located.card.id, targetZone, null, options || {});
          return true;
        }

        function cardMoveShortcutForKey(key) {
          switch (String(key || "").toLowerCase()) {
            case "f":
              return { zone: "battlefield" };
            case "h":
              return { zone: "hand" };
            case "l":
              return { zone: "library", options: { libraryPosition: "top" } };
            case "b":
              return { zone: "library", options: { libraryPosition: "bottom" } };
            case "g":
              return { zone: "graveyard" };
            case "e":
              return { zone: "exile" };
            case "q":
              return { zone: "command" };
            default:
              return null;
          }
        }

        function shortcutBlockedByOverlay() {
          return tokenOverlayOpen() ||
            scryOverlayOpen() ||
            librarySearchOverlayOpen() ||
            countPickerOpen() ||
            customCountOpen() ||
            historyOverlayOpen() ||
            cardDetailOverlayOpen() ||
            libraryMenuOpen() ||
            commandMenuOpen() ||
            toolbarMenuOpen();
        }

        function runHoveredCardShortcut(key) {
          key = String(key || "").toLowerCase();
          if (!key || shortcutBlockedByOverlay()) return false;
          var located = hoveredZoneCard();
          if (!located) return false;
          state.activeCardID = located.card.id;

          if (key === "p" && cardInlinePrimaryAction(located)) {
            runActiveCardInlinePrimaryAction();
            return true;
          }
          if (key === "t" && located.zone === "battlefield") {
            runActiveCardInlinePrimaryAction();
            return true;
          }
          var moveShortcut = cardMoveShortcutForKey(key);
          if (moveShortcut) return moveShortcutCard(located, moveShortcut.zone, moveShortcut.options);
          if (key === "c" && located.zone === "battlefield") return openHoveredCardMenu("counters");
          if (key === "x" && located.zone === "battlefield") return openHoveredCardMenu("copy");
          if (key === "v") {
            setCardInlineMenu(false);
            setCardDetailOverlay(true);
            return true;
          }
          return false;
        }

        function formatManaValue(value) {
          var number = Number(value || 0);
          if (!Number.isFinite(number)) return "0";
          return String(Math.round(number * 10) / 10).replace(/\.0$/, "");
        }

        function formatSetLabel(card) {
          var setName = String(card && card.setName || "").trim();
          var setCode = String(card && card.setCode || "").trim().toUpperCase();
          if (setName && setCode) return setName + " (" + setCode + ")";
          return setName || setCode || "-";
        }

        function renderColorList(target, colors) {
          if (!target) return;
          var list = normalizeCardColorList(colors);
          if (!list.length) {
            target.textContent = "Colorless";
            return;
          }
          var html = "";
          for (var i = 0; i < list.length; i++) {
            var color = list[i];
            if (cardUI.renderCardSymbol) {
              html += cardUI.renderCardSymbol(color, "h-5 w-5");
            } else {
              html += renderManaSymbols("{" + color + "}") || escapeHTML(color);
            }
          }
          html += '<span class="sr-only">' + escapeHTML(list.join(", ")) + "</span>";
          target.innerHTML = html;
        }

        function renderCardDetail() {
          var located = activeCard();
          var card = located ? located.card : null;
          if (!card) {
            setCardDetailOverlay(false);
            return;
          }

          if (cardDetailName) cardDetailName.textContent = card.name || "Card";
          if (cardDetailType) cardDetailType.textContent = card.typeLine || "Unknown type";
          if (cardDetailNameField) cardDetailNameField.textContent = card.name || "-";
          if (cardDetailTypeField) cardDetailTypeField.textContent = card.typeLine || "-";
          if (cardDetailManaValue) cardDetailManaValue.textContent = formatManaValue(card.manaValue);
          renderColorList(cardDetailColors, card.colors);
          renderColorList(cardDetailColorIdentity, card.colorIdentity);
          if (cardDetailPrice) cardDetailPrice.textContent = formatPriceDisplay(card.priceUSD, "-");
          if (cardDetailArtist) cardDetailArtist.textContent = card.artist || "-";
          if (cardDetailSet) cardDetailSet.textContent = formatSetLabel(card);
          if (cardDetailCollectorNumber) cardDetailCollectorNumber.textContent = card.collectorNumber || "-";
          if (cardDetailOracle) {
            cardDetailOracle.innerHTML = renderCardTextMarkup(card.oracleText, "No oracle text.");
          }
          if (cardDetailFlavor) {
            cardDetailFlavor.innerHTML = renderCardTextMarkup(card.flavorText, "No flavor text.");
          }
          if (cardDetailImage) {
            if (card.imageURI) {
              cardDetailImage.onerror = function () {
                cardDetailImage.src = "";
                cardDetailImage.classList.add("hidden");
                if (cardDetailFallback) {
                  cardDetailFallback.textContent = card.name || "Unknown card";
                  cardDetailFallback.classList.remove("hidden");
                }
              };
              cardDetailImage.alt = card.name || "Card";
              cardDetailImage.src = card.imageURI;
              cardDetailImage.classList.remove("hidden");
              if (cardDetailFallback) cardDetailFallback.classList.add("hidden");
            } else {
              cardDetailImage.onerror = null;
              cardDetailImage.src = "";
              cardDetailImage.classList.add("hidden");
              if (cardDetailFallback) {
                cardDetailFallback.textContent = card.name || "Unknown card";
                cardDetailFallback.classList.remove("hidden");
              }
            }
          }
        }

        function renderOpeningHand() {
          if (!openingHandEl) return;
          openingHandEl.innerHTML = "";
          var hand = state.zones.hand;
          if (!hand || hand.length === 0) {
            var empty = document.createElement("p");
            empty.className = "pt-empty-message pt-empty-message--centered";
            empty.textContent = "No cards in opening hand.";
            openingHandEl.appendChild(empty);
            return;
          }
          for (var i = 0; i < hand.length; i++) {
            openingHandEl.appendChild(createCardNode(hand[i], "hand", true, { openingDrag: true }));
          }
        }

        function renderLibraryZone(el) {
          if (!el) return;
          el.innerHTML = "";
          el.title = state.phase === "play" ? "Click to draw a card." : "";
          if (state.zones.library.length === 0) {
            state.libraryRevealed = false;
            return;
          }

          if (state.libraryRevealed) {
            var topCard = state.zones.library[state.zones.library.length - 1];
            if (topCard) {
              var revealWrap = document.createElement("div");
              revealWrap.className = "flex flex-col items-center gap-1 pt-1";
              revealWrap.addEventListener("click", function (e) {
                e.stopPropagation();
              });
              var node = createCardNode(topCard, "library", false);
              node.classList.add("pt-card--stack");
              revealWrap.appendChild(node);
              var revealLabel = document.createElement("p");
              revealLabel.className = "pt-card-note";
              revealLabel.textContent = "Top card";
              revealWrap.appendChild(revealLabel);
              el.appendChild(revealWrap);
              return;
            }
          }

          var stack = document.createElement("div");
          stack.className = "pt-library-stack";
          for (var i = 0; i < 3; i++) {
            var back = document.createElement("div");
            back.className = "pt-library-stack__card";
            stack.appendChild(back);
          }
          el.appendChild(stack);

          var label = document.createElement("p");
          label.className = "pt-card-note pt-card-note--spaced";
          label.textContent = state.zones.library.length + " cards";
          el.appendChild(label);
        }

        function renderStackZone(zoneName, el) {
          if (!el) return;
          el.innerHTML = "";

          var zone = getZoneArray(zoneName);
          if (!zone || zone.length === 0) {
            return;
          }

          var visible = Math.min(zone.length, 5);
          var start = zone.length - visible;

          var wrap = document.createElement("div");
          wrap.className = "relative";
          wrap.style.height = String(stackedCardHeight + 12) + "px";
          wrap.style.width = String(stackedCardWidth + Math.max(0, visible - 1) * stackedOffsetX) + "px";
          wrap.style.margin = "4px auto 0";

          for (var i = 0; i < visible; i++) {
            var card = zone[start + i];
            var node = createCardNode(card, zoneName, false);
            node.style.position = "absolute";
            node.style.left = String(i * stackedOffsetX) + "px";
            node.style.top = String(i * stackedOffsetY) + "px";
            node.style.zIndex = String(10 + i);
            wrap.appendChild(node);
          }

          el.appendChild(wrap);
        }

        function renderHandZone() {
          var el = zoneEls.hand;
          if (!el) return;
          el.innerHTML = "";

          if (state.phase !== "play") {
            var waiting = document.createElement("p");
            waiting.className = "pt-empty-message pt-empty-message--waiting";
            waiting.textContent = "Opening hand is shown in the start overlay.";
            el.appendChild(waiting);
            return;
          }

          var hand = state.zones.hand;
          if (!hand || hand.length === 0) {
            return;
          }

          for (var i = 0; i < hand.length; i++) {
            el.appendChild(createCardNode(hand[i], "hand", false));
          }
        }

        function renderBattlefieldZone() {
          var el = zoneEls.battlefield;
          if (!el) return;
          el.innerHTML = "";

          var zone = state.zones.battlefield;
          if (!zone || zone.length === 0) {
            return;
          }

          for (var i = 0; i < zone.length; i++) {
            var card = zone[i];
            if (typeof card.bfX !== "number" || typeof card.bfY !== "number") {
              setBattlefieldPosition(card, null, false);
            }
            var node = createCardNode(card, "battlefield", false);
            node.style.position = "absolute";
            node.style.left = String(card.bfX) + "px";
            node.style.top = String(card.bfY) + "px";
            node.style.zIndex = String(typeof card.bfZ === "number" ? card.bfZ : (20 + Math.min(i, 180)));
            el.appendChild(node);
          }
        }

        function bindDropZone(zoneName, el) {
          if (!el) return;

          el.addEventListener("dragover", function (e) {
            if (state.draggingScryCard) return;
            var cardID = state.draggingCardID;
            if (cardID == null) return;
            var located = findCard(cardID);
            if (!located) return;
            if (!canDrop(located.zone, zoneName, located.card)) return;
            e.preventDefault();
            el.classList.add("pt-drop-zone--active");
          });

          el.addEventListener("dragleave", function () {
            el.classList.remove("pt-drop-zone--active");
          });

          el.addEventListener("drop", function (e) {
            if (state.draggingScryCard) return;
            e.preventDefault();
            el.classList.remove("pt-drop-zone--active");

            var cardID = state.draggingCardID;
            if (cardID == null && e.dataTransfer) {
              var raw = e.dataTransfer.getData("text/plain");
              if (raw) cardID = Number(raw);
            }
            if (cardID == null) return;

            moveCard(Number(cardID), zoneName, e);
            state.draggingCardID = null;
            state.draggingOpeningCard = false;
            clearDragAvatar();
          });
        }

        function bindOpeningHandReorder() {
          if (!openingHandEl) return;

          openingHandEl.addEventListener("dragover", function (e) {
            var cardID = state.draggingCardID;
            if (cardID == null || !state.draggingOpeningCard) return;
            var located = findCard(cardID);
            if (!located || located.zone !== "hand") return;
            e.preventDefault();
            openingHandEl.classList.add("pt-drop-zone--active");
          });

          openingHandEl.addEventListener("dragleave", function () {
            openingHandEl.classList.remove("pt-drop-zone--active");
          });

          openingHandEl.addEventListener("drop", function (e) {
            e.preventDefault();
            openingHandEl.classList.remove("pt-drop-zone--active");
            var cardID = state.draggingCardID;
            if (cardID == null && e.dataTransfer) {
              var raw = e.dataTransfer.getData("text/plain");
              if (raw) cardID = Number(raw);
            }
            if (cardID == null) return;
            reorderOpeningHandCard(Number(cardID), e);
            state.draggingCardID = null;
            state.draggingOpeningCard = false;
            render();
            clearDragAvatar();
          });
        }

        function render() {
          for (var i = 0; i < zoneOrder.length; i++) {
            var zoneName = zoneOrder[i];
            var zone = getZoneArray(zoneName) || [];
            var countEl = countEls[zoneName];
            if (countEl) countEl.textContent = zoneLabel(zoneName) + " (" + String(zone.length) + ")";
          }

          if (elTurnCount) elTurnCount.textContent = String(state.turn);
          if (elLifeTotal) elLifeTotal.textContent = String(state.life);
          if (elCommanderDamageTotal) elCommanderDamageTotal.textContent = String(state.commanderDamage);
          if (elCommandTaxValue) elCommandTaxValue.textContent = String(state.commandTax || 0);
          if (btnCommandTaxDown) btnCommandTaxDown.disabled = (state.commandTax || 0) <= 0;
          if (elToolbarTurn) elToolbarTurn.textContent = "End Turn " + String(state.turn);
          if (elToolbarLife) elToolbarLife.textContent = "Life " + String(state.life);
          if (elToolbarDamage) elToolbarDamage.textContent = "Cmd " + String(state.commanderDamage);
          if (elToolbarMana) {
            elToolbarMana.innerHTML = '<span>Mana</span><span class="pt-toolbar-mana-pips" aria-hidden="true">' +
              renderToolbarManaPips() +
              '</span><span class="pt-toolbar-mana-summary">' + String(manaTotal()) + "</span>";
          }
          if (btnLibraryMenuReveal) btnLibraryMenuReveal.textContent = "Reveal";
          Object.keys(manaCountEls).forEach(function (color) {
            if (manaCountEls[color]) {
              manaCountEls[color].textContent = String((state.mana && state.mana[color]) || 0);
            }
          });

          var pregame = state.phase !== "play";
          var required = requiredOpeningReturns();
          var selected = Array.isArray(state.openingSelectionIDs) ? state.openingSelectionIDs.length : 0;
          if (openingScreen) openingScreen.classList.toggle("hidden", !pregame);
          if (openingLabelEl) {
            openingLabelEl.textContent = state.phase === "empty"
              ? "No Library Cards"
              : (state.phase === "cleanup" ? "Choose Cards to Put Back" : "Opening Hand");
          }
          if (openingHintEl) {
            openingHintEl.textContent = state.phase === "empty"
              ? "Add cards to this deck, then come back to shuffle and draw an opening hand."
              : (state.phase === "cleanup"
                ? ("Choose " + required + " card(s) to put on the bottom of your library.")
                : "Review this hand, then keep or take a fresh seven.");
          }
          if (openingProgressEl) {
            openingProgressEl.classList.toggle("hidden", state.phase === "opening");
            openingProgressEl.textContent = state.phase === "empty"
              ? "0 cards"
              : (state.phase === "cleanup"
                ? (selected + "/" + required + " selected")
                : ((state.zones.hand || []).length + " cards"));
          }
          if (btnMulligan) {
            btnMulligan.textContent = state.mulligans === 0
              ? "Mulligan (Free)"
              : "Mulligan (" + state.mulligans + ")";
            btnMulligan.classList.toggle("hidden", state.phase !== "opening");
          }
          if (btnKeep) {
            btnKeep.classList.toggle("hidden", state.phase === "empty");
            if (state.phase === "cleanup") {
              btnKeep.textContent = "Begin (" + selected + "/" + required + ")";
              btnKeep.disabled = selected !== required;
            } else {
              btnKeep.textContent = "Keep";
              btnKeep.disabled = state.phase !== "opening";
            }
          }
          if (openingReturnLink) {
            openingReturnLink.classList.toggle("hidden", state.phase !== "empty");
          }

          renderOpeningHand();
          renderBattlefieldZone();
          renderHandZone();
          renderStackZone("command", zoneEls.command);
          renderStackZone("graveyard", zoneEls.graveyard);
          renderStackZone("exile", zoneEls.exile);
          renderLibraryZone(zoneEls.library);
          if (librarySearchOverlayOpen()) renderLibrarySearchCards();
        }

        if (btnKeep) btnKeep.addEventListener("click", keepOpening);
        if (btnMulligan) btnMulligan.addEventListener("click", mulliganOpening);

        bindDropZone("command", zoneEls.command);
        bindDropZone("library", zoneEls.library);
        bindDropZone("hand", zoneEls.hand);
        bindDropZone("battlefield", zoneEls.battlefield);
        bindDropZone("graveyard", zoneEls.graveyard);
        bindDropZone("exile", zoneEls.exile);
        bindOpeningHandReorder();

        if (zoneEls.library) {
          zoneEls.library.addEventListener("click", function (e) {
            var target = e.target;
            if (target && target.closest && target.closest(".pt-card")) return;
            drawOne();
          });
        }
        if (btnLibraryMenuToggle) {
          btnLibraryMenuToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setLibraryMenu(!libraryMenuOpen());
          });
        }
        if (btnCommandMenuToggle) {
          btnCommandMenuToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setCommandMenu(!commandMenuOpen());
          });
        }
        if (btnLibraryMenuDraw) {
          btnLibraryMenuDraw.addEventListener("click", function () {
            setLibraryMenu(false);
            drawFromLibrary(1);
          });
        }
        if (btnLibraryMenuDrawX) {
          btnLibraryMenuDrawX.addEventListener("click", function () {
            setCountPickerOverlay(true, "draw");
          });
        }
        if (btnLibraryMenuShuffle) {
          btnLibraryMenuShuffle.addEventListener("click", function () {
            setLibraryMenu(false);
            shuffleLibrary();
          });
        }
        if (btnLibraryMenuSearch) {
          btnLibraryMenuSearch.addEventListener("click", function () {
            openLibrarySearch();
          });
        }
        if (btnLibraryMenuReveal) {
          btnLibraryMenuReveal.addEventListener("click", toggleLibraryReveal);
        }
        if (btnLibraryMenuRevealX) {
          btnLibraryMenuRevealX.addEventListener("click", function () {
            setCountPickerOverlay(true, "reveal");
          });
        }
        if (btnLibraryMenuScry) {
          btnLibraryMenuScry.addEventListener("click", function () { startScry(1); });
        }
        if (btnLibraryMenuScryX) {
          btnLibraryMenuScryX.addEventListener("click", function () {
            setCountPickerOverlay(true, "scry");
          });
        }
        if (btnLibraryMenuSurveil) {
          btnLibraryMenuSurveil.addEventListener("click", function () { startSurveil(1); });
        }
        if (btnLibraryMenuSurveilX) {
          btnLibraryMenuSurveilX.addEventListener("click", function () {
            setCountPickerOverlay(true, "surveil");
          });
        }
        if (btnLibraryMenuMill) {
          btnLibraryMenuMill.addEventListener("click", function () {
            setLibraryMenu(false);
            millCards(1);
          });
        }
        if (btnLibraryMenuMillX) {
          btnLibraryMenuMillX.addEventListener("click", function () {
            setCountPickerOverlay(true, "mill");
          });
        }
        if (btnLibraryMenuKeywordToggle) {
          btnLibraryMenuKeywordToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setLibraryKeywordPanel(libraryKeywordPanel ? libraryKeywordPanel.classList.contains("hidden") : true);
          });
        }
        if (btnLibraryMenuDiscover) {
          btnLibraryMenuDiscover.addEventListener("click", function () {
            setCountPickerOverlay(true, "discover");
          });
        }
        if (btnLibraryMenuCascade) {
          btnLibraryMenuCascade.addEventListener("click", function () {
            setCountPickerOverlay(true, "cascade");
          });
        }
        if (btnLibraryMenuRevealUntilToggle) {
          btnLibraryMenuRevealUntilToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setLibraryRevealUntilPanel(libraryRevealUntilPanel ? libraryRevealUntilPanel.classList.contains("hidden") : true);
          });
        }
        libraryRevealUntilButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            revealForKeyword(btn.dataset.revealUntil);
          });
        });
        toolbarMenuButtons.forEach(function (btn) {
          btn.setAttribute("aria-expanded", "false");
          btn.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            var menuName = String(btn.dataset.toolbarMenuToggle || "").trim();
            var open = btn.getAttribute("aria-expanded") === "true";
            setToolbarMenu(open ? "" : menuName, btn);
          });
        });

        // --- Coin flip ---
        function flipCoin() {
          var result = Math.random() < 0.5 ? "Heads" : "Tails";
          setStatus("Coin flip: " + result + "!");
        }

        // --- Dice roller ---
        function rollDice(sides) {
          sides = Math.max(2, Math.floor(Number(sides || 20)));
          if (!Number.isFinite(sides)) sides = 20;
          var result = Math.floor(Math.random() * sides) + 1;
          setStatus("d" + sides + " roll: " + result + "!");
        }

        // --- Token creation ---
        var tokenOverlay = document.getElementById("pt-token-overlay");
        var btnTokenClose = document.getElementById("pt-token-close");
        var btnTokenBackdrop = document.getElementById("pt-token-backdrop");
        var btnTokenSubmit = document.getElementById("pt-token-submit");
        var inputTokenName = document.getElementById("pt-token-name");
        var inputTokenPower = document.getElementById("pt-token-power");
        var inputTokenToughness = document.getElementById("pt-token-toughness");
        var inputTokenType = document.getElementById("pt-token-type");
        var inputTokenQty = document.getElementById("pt-token-qty");

        function setTokenOverlay(open) {
          if (!tokenOverlay) return;
          if (open) closeToolbarMenus();
          tokenOverlay.classList.toggle("hidden", !open);
          if (open && inputTokenName) inputTokenName.focus();
        }

        function tokenOverlayOpen() {
          if (!tokenOverlay) return false;
          return !tokenOverlay.classList.contains("hidden");
        }

        function createTokens() {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            setTokenOverlay(false);
            return;
          }

          var name = (inputTokenName ? inputTokenName.value : "Token").trim() || "Token";
          var power = inputTokenPower ? parseInt(inputTokenPower.value, 10) || 0 : 0;
          var toughness = inputTokenToughness ? parseInt(inputTokenToughness.value, 10) || 0 : 0;
          var typeLine = inputTokenType ? inputTokenType.value.trim() : "";
          var qty = inputTokenQty ? Math.max(1, Math.min(20, parseInt(inputTokenQty.value, 10) || 1)) : 1;

          var label = name + " " + power + "/" + toughness;
          if (!typeLine) typeLine = "Token Creature";

          for (var i = 0; i < qty; i++) {
            var token = createCard(label, "", "", typeLine, "", false);
            token.isToken = true;
            state.zones.battlefield.push(token);
            setBattlefieldPosition(token, null, false);
          }

          setStatus("Created " + qty + "x " + label + " token" + (qty > 1 ? "s" : "") + ".");
          setTokenOverlay(false);
          render();
        }

        function createPresetToken(kind) {
          if (state.phase !== "play") {
            setStatus("Keep your opening hand first.");
            closeToolbarMenus();
            return;
          }
          kind = String(kind || "").trim().toLowerCase();
          var preset = {
            name: "Treasure",
            imageURI: "https://cards.scryfall.io/normal/front/b/4/b4f61b5e-9c53-40b1-b93e-3ffa351ff052.jpg?1775828602",
            typeLine: "Token Artifact - Treasure",
            oracleText: "{T}, Sacrifice this artifact: Add one mana of any color."
          };
          if (kind === "food") {
            preset = {
              name: "Food",
              imageURI: "https://cards.scryfall.io/normal/front/d/a/da1fca2e-4ea7-4c5f-bd58-f9529dfb6f37.jpg?1775827284",
              typeLine: "Token Artifact - Food",
              oracleText: "{2}, {T}, Sacrifice this artifact: You gain 3 life."
            };
          } else if (kind === "clue") {
            preset = {
              name: "Clue",
              imageURI: "https://cards.scryfall.io/normal/front/c/3/c321b9e4-ab7e-4e8a-988f-5463c776d685.jpg?1771590258",
              typeLine: "Token Artifact - Clue",
              oracleText: "{2}, Sacrifice this artifact: Draw a card."
            };
          }

          var token = createCard(preset.name, preset.imageURI, "", preset.typeLine, preset.oracleText, false);
          token.isToken = true;
          state.zones.battlefield.push(token);
          setBattlefieldPosition(token, null, false);
          setStatus("Created a " + preset.name + " token.");
          closeToolbarMenus();
          render();
        }

        if (btnTokenClose) btnTokenClose.addEventListener("click", function () { setTokenOverlay(false); });
        if (btnTokenBackdrop) btnTokenBackdrop.addEventListener("click", function () { setTokenOverlay(false); });
        if (btnTokenSubmit) btnTokenSubmit.addEventListener("click", createTokens);
        if (btnLifeDown) btnLifeDown.addEventListener("click", function () { adjustLife(-1); });
        if (btnLifeUp) btnLifeUp.addEventListener("click", function () { adjustLife(1); });
        if (btnCommanderDamageDown) btnCommanderDamageDown.addEventListener("click", function () { adjustCommanderDamage(-1); });
        if (btnCommanderDamageUp) btnCommanderDamageUp.addEventListener("click", function () { adjustCommanderDamage(1); });
        if (btnCommandTaxDown) btnCommandTaxDown.addEventListener("click", function () { adjustCommandTax(-2); });
        if (btnCommandTaxUp) btnCommandTaxUp.addEventListener("click", function () { adjustCommandTax(2); });
        manaStepButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            adjustMana(btn.dataset.mana, btn.dataset.delta);
          });
        });
        if (btnManaClear) btnManaClear.addEventListener("click", function () {
          resetManaPool();
          setStatus("Mana pool cleared.");
          render();
        });
        if (elToolbarTurn) elToolbarTurn.addEventListener("click", function () {
          closeToolbarMenus();
          nextTurn();
        });
        if (btnTokenToggle) {
          btnTokenToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setTokenPanel(tokenPanel ? tokenPanel.classList.contains("hidden") : true);
          });
        }
        tokenPresetButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            createPresetToken(btn.dataset.tokenPreset);
          });
        });
        if (btnTokenOpen) btnTokenOpen.addEventListener("click", function () {
          if (state.phase === "play") {
            setTokenOverlay(true);
          } else {
            setStatus("Keep your opening hand first.");
          }
        });
        if (btnCoinFlip) btnCoinFlip.addEventListener("click", flipCoin);
        if (btnDiceToggle) {
          btnDiceToggle.addEventListener("click", function (e) {
            e.preventDefault();
            e.stopPropagation();
            setDicePanel(dicePanel ? dicePanel.classList.contains("hidden") : true);
          });
        }
        diceOptionButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            closeToolbarMenus();
            rollDice(btn.dataset.sides);
          });
        });
        if (btnGameMenuRestart) btnGameMenuRestart.addEventListener("click", function () {
          closeToolbarMenus();
          startOpening({ animate: true });
        });
        if (gameMenuReturnLink && isWorkbenchPlaytest) {
          gameMenuReturnLink.addEventListener("click", function (e) {
            e.preventDefault();
            submitWorkbenchReturn();
          });
        }
        if (openingReturnLink && isWorkbenchPlaytest) {
          openingReturnLink.addEventListener("click", function (e) {
            e.preventDefault();
            submitWorkbenchReturn();
          });
        }
        if (isWorkbenchPlaytest) {
          window.addEventListener("pagehide", persistWorkbenchDraftLocally);
        }
        if (btnScryCancel) btnScryCancel.addEventListener("click", cancelScry);
        if (btnScryBackdrop) btnScryBackdrop.addEventListener("click", cancelScry);
        if (btnScryFinish) btnScryFinish.addEventListener("click", finishScry);
        if (btnOrganizeBoard) {
          btnOrganizeBoard.addEventListener("click", function () {
            closeToolbarMenus();
            organizeBattlefield();
          });
        }
        if (btnLibrarySearchClose) btnLibrarySearchClose.addEventListener("click", function () { setLibrarySearchOverlay(false); });
        if (btnLibrarySearchBackdrop) btnLibrarySearchBackdrop.addEventListener("click", function () { setLibrarySearchOverlay(false); });
        if (librarySearchInput) {
          librarySearchInput.addEventListener("input", function () {
            state.librarySearchQuery = librarySearchInput.value || "";
            renderLibrarySearchCards();
          });
        }
        if (elVisibleStatus) elVisibleStatus.addEventListener("click", function () { setHistoryOverlay(true); });
        if (btnHistoryClose) btnHistoryClose.addEventListener("click", function () { setHistoryOverlay(false); });
        if (btnHistoryBackdrop) btnHistoryBackdrop.addEventListener("click", function () { setHistoryOverlay(false); });
        if (btnCountPickerClose) btnCountPickerClose.addEventListener("click", function () { setCountPickerOverlay(false); });
        if (btnCountPickerBackdrop) btnCountPickerBackdrop.addEventListener("click", function () { setCountPickerOverlay(false); });
        if (btnCountPickerCustom) {
          btnCountPickerCustom.addEventListener("click", function () {
            setCountPickerOverlay(false);
            setCustomCountOverlay(true);
          });
        }
        if (btnCustomCountClose) btnCustomCountClose.addEventListener("click", function () { setCustomCountOverlay(false); });
        if (btnCustomCountBackdrop) btnCustomCountBackdrop.addEventListener("click", function () { setCustomCountOverlay(false); });
        if (btnCustomCountSubmit) {
          btnCustomCountSubmit.addEventListener("click", function () {
            performLibraryCountAction(state.countPickerAction, customCountInput ? customCountInput.value : 1);
          });
        }
        if (customCountInput) {
          customCountInput.addEventListener("keydown", function (e) {
            if (String(e.key || "").toLowerCase() !== "enter") return;
            e.preventDefault();
            performLibraryCountAction(state.countPickerAction, customCountInput.value);
          });
        }
        if (btnCardInlineMenuPrimary) {
          btnCardInlineMenuPrimary.addEventListener("click", runActiveCardInlinePrimaryAction);
        }
        if (btnCardInlineMenuMoveToggle) {
          btnCardInlineMenuMoveToggle.addEventListener("click", function () {
            setCardInlineMovePanel(cardInlineMenuMovePanel ? cardInlineMenuMovePanel.classList.contains("hidden") : false);
          });
        }
        if (btnCardInlineMenuCountersToggle) {
          btnCardInlineMenuCountersToggle.addEventListener("click", function () {
            setCardInlineCounterPanel(cardInlineMenuCountersPanel ? cardInlineMenuCountersPanel.classList.contains("hidden") : false);
          });
        }
        if (btnCardInlineMenuCopyToggle) {
          btnCardInlineMenuCopyToggle.addEventListener("click", function () {
            setCardInlineCopyPanel(cardInlineMenuCopyPanel ? cardInlineMenuCopyPanel.classList.contains("hidden") : false);
          });
        }
        if (btnCardInlineMenuDetails) {
          btnCardInlineMenuDetails.addEventListener("click", function () {
            setCardInlineMenu(false);
            setCardDetailOverlay(true);
          });
        }
        cardInlineMoveButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            var zone = String(btn.dataset.zone || "").trim();
            var located = activeCard();
            if (!located || !zone || btn.disabled) return;
            moveShortcutCard(located, zone, {
              libraryPosition: String(btn.dataset.libraryPosition || "").trim()
            });
          });
        });
        cardInlineCounterButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            var type = String(btn.dataset.counterType || "").trim();
            var label = String(btn.dataset.counterLabel || "").trim();
            if (!type) return;
            if (type === "plusx") {
              setCardInlineCounterPanel(false);
              setCardInlineMenu(false);
              setCountPickerOverlay(true, "counter-plus-x");
              return;
            }
            if (type === "minusx") {
              setCardInlineCounterPanel(false);
              setCardInlineMenu(false);
              setCountPickerOverlay(true, "counter-minus-x");
              return;
            }
            setCardInlineMenu(false);
            applyActiveCardCounter(type, 1, label);
          });
        });
        cardInlineCopyButtons.forEach(function (btn) {
          btn.addEventListener("click", function () {
            var value = String(btn.dataset.copyCount || "").trim();
            if (value === "custom") {
              setCardInlineCopyPanel(false);
              setCardInlineMenu(false);
              state.countPickerAction = "copy-card";
              setCustomCountOverlay(true);
              return;
            }
            setCardInlineMenu(false);
            createActiveCardCopies(value);
          });
        });
        if (btnCardDetailClose) btnCardDetailClose.addEventListener("click", function () { setCardDetailOverlay(false); });
        if (btnCardDetailBackdrop) btnCardDetailBackdrop.addEventListener("click", function () { setCardDetailOverlay(false); });

        document.addEventListener("dragover", updateDragAvatar);
        document.addEventListener("drag", updateDragAvatar);
        document.addEventListener("drop", clearDragAvatar);
        function updatePointerCardTarget(e) {
          state.pointerX = typeof e.clientX === "number" ? e.clientX : null;
          state.pointerY = typeof e.clientY === "number" ? e.clientY : null;
          var target = e.target;
          var cardEl = target && target.closest ? target.closest(".pt-card[data-zone-card]") : null;
          if (!cardEl) return;
          var cardID = Number(cardEl.dataset.cardId || 0);
          if (cardID) state.hoveredCardID = cardID;
        }
        document.addEventListener("mousemove", updatePointerCardTarget);
        document.addEventListener("pointermove", updatePointerCardTarget);

        document.addEventListener("click", function (e) {
          var target = e.target;
          if (cardInlineMenuOpen()) {
            if (cardInlineMenu && cardInlineMenu.contains(target)) return;
            if (cardInlineMenuMovePanel && cardInlineMenuMovePanel.contains(target)) return;
            if (cardInlineMenuCountersPanel && cardInlineMenuCountersPanel.contains(target)) return;
            if (cardInlineMenuCopyPanel && cardInlineMenuCopyPanel.contains(target)) return;
            if (cardInlineMenuTrigger && cardInlineMenuTrigger.contains && cardInlineMenuTrigger.contains(target)) return;
            setCardInlineMenu(false);
          }
          if (commandMenuOpen()) {
            if (commandMenu && commandMenu.contains(target)) return;
            if (btnCommandMenuToggle && btnCommandMenuToggle.contains(target)) return;
            setCommandMenu(false);
          }
          if (libraryMenuOpen()) {
            if (libraryMenu && libraryMenu.contains(target)) return;
            if (libraryKeywordPanel && libraryKeywordPanel.contains(target)) return;
            if (libraryRevealUntilPanel && libraryRevealUntilPanel.contains(target)) return;
            if (btnLibraryMenuToggle && btnLibraryMenuToggle.contains(target)) return;
            setLibraryMenu(false);
          }
          if (toolbarMenuOpen()) {
            if (target && target.closest && target.closest(".pt-game-bar")) return;
            if (tokenPanel && tokenPanel.contains(target)) return;
            if (dicePanel && dicePanel.contains(target)) return;
            closeToolbarMenus();
          }
        });

        document.addEventListener("keydown", function (e) {
          var key = String(e.key || "").toLowerCase();

          // Close overlays on Escape
          if (key === "escape") {
            if (customCountOpen()) { setCustomCountOverlay(false); return; }
            if (countPickerOpen()) { setCountPickerOverlay(false); return; }
            if (historyOverlayOpen()) { setHistoryOverlay(false); return; }
            if (cardDetailOverlayOpen()) { setCardDetailOverlay(false); return; }
            if (cardInlineMenuOpen()) { setCardInlineMenu(false); return; }
            if (librarySearchOverlayOpen()) { setLibrarySearchOverlay(false); return; }
            if (scryOverlayOpen()) { cancelScry(); return; }
            if (tokenOverlayOpen()) { setTokenOverlay(false); return; }
            if (libraryMenuOpen()) { setLibraryMenu(false); return; }
            if (commandMenuOpen()) { setCommandMenu(false); return; }
            if (toolbarMenuOpen()) { closeToolbarMenus(); return; }
            return;
          }

          // Don't fire shortcuts if typing in an input
          if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) {
            if (key === "enter" && tokenOverlayOpen()) { createTokens(); e.preventDefault(); }
            return;
          }

          if (runHoveredCardShortcut(key)) {
            e.preventDefault();
            return;
          }

          if (tokenOverlayOpen() || scryOverlayOpen() || librarySearchOverlayOpen() || countPickerOpen() || customCountOpen() || historyOverlayOpen() || cardInlineMenuOpen() || cardDetailOverlayOpen() || libraryMenuOpen() || commandMenuOpen() || toolbarMenuOpen()) return;
          if (key === "d") drawOne();
          if (key === "n") nextTurn();
          if (key === "m") mulliganOpening();
          if (key === "k") keepOpening();
          if (key === "f") flipCoin();
          if (key === "r") rollDice();
          if (key === "t") {
            if (state.phase === "play") setTokenOverlay(true);
          }
        });

        window.addEventListener("resize", function () {
          if (libraryMenuOpen()) positionInlineMenu(libraryMenu, btnLibraryMenuToggle);
          if (commandMenuOpen()) positionInlineMenu(commandMenu, btnCommandMenuToggle);
          if (libraryKeywordPanel && !libraryKeywordPanel.classList.contains("hidden")) {
            positionLibraryKeywordMenu();
          }
          if (libraryRevealUntilPanel && !libraryRevealUntilPanel.classList.contains("hidden")) {
            positionFlyoutMenu(libraryRevealUntilPanel, btnLibraryMenuRevealUntilToggle);
          }
          if (cardInlineMenuOpen()) positionInlineMenu(cardInlineMenu, cardInlineMenuTrigger);
          if (cardInlineMenuMovePanel && !cardInlineMenuMovePanel.classList.contains("hidden")) {
            positionFlyoutMenu(cardInlineMenuMovePanel, btnCardInlineMenuMoveToggle);
          }
          if (cardInlineMenuCountersPanel && !cardInlineMenuCountersPanel.classList.contains("hidden")) {
            positionFlyoutMenu(cardInlineMenuCountersPanel, btnCardInlineMenuCountersToggle);
          }
          if (cardInlineMenuCopyPanel && !cardInlineMenuCopyPanel.classList.contains("hidden")) {
            positionFlyoutMenu(cardInlineMenuCopyPanel, btnCardInlineMenuCopyToggle);
          }
          var activeToolbar = expandedToolbarButton();
          if (activeToolbar) {
            positionToolbarMenu(toolbarMenuFor(String(activeToolbar.dataset.toolbarMenuToggle || "")), activeToolbar);
          }
          if (tokenPanel && !tokenPanel.classList.contains("hidden")) {
            positionFlyoutMenu(tokenPanel, btnTokenToggle);
          }
          if (dicePanel && !dicePanel.classList.contains("hidden")) {
            positionFlyoutMenu(dicePanel, btnDiceToggle);
          }
          if (state.phase === "play") {
            organizeBattlefield({ silent: true });
          }
        });

        installManaPickerSymbols();
        startOpening();
      })();
