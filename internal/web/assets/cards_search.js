(function () {
  "use strict";

  function readFormJSON(form, selector) {
    const source = form.querySelector(selector);
    if (!source) return [];
    try {
      return JSON.parse(source.textContent || "[]");
    } catch (error) {
      return [];
    }
  }

  function setupCardSearchForm(form) {
    if (!form || form.getAttribute("data-card-search-bound") === "true") return;
    form.setAttribute("data-card-search-bound", "true");

    const typeRoot = form.querySelector("[data-type-filter-root]");
    const typeInput = typeRoot ? typeRoot.querySelector("[data-type-filter-input]") : null;
    const typeMenu = typeRoot ? typeRoot.querySelector("[data-type-filter-menu]") : null;
    const typeSelected = typeRoot ? typeRoot.querySelector("[data-type-filter-selected]") : null;
    const typeHidden = typeRoot ? typeRoot.querySelector("[data-type-filter-hidden]") : null;
    const typeStatus = typeRoot ? typeRoot.querySelector("[data-type-filter-status]") : null;

    const setRoot = form.querySelector("[data-set-autocomplete-root]");
    const setInput = setRoot ? setRoot.querySelector("[data-set-autocomplete-input]") : null;
    const setMenu = setRoot ? setRoot.querySelector("[data-set-autocomplete-menu]") : null;

    const summaryCount = form.querySelector("[data-search-summary-count]");
    const summaryChips = form.querySelector("[data-search-summary-chips]");
    const rangeError = form.querySelector("[data-search-range-error]");
    const searchSubmit = form.querySelector("[data-search-submit]");

    const statRowsRoot = form.querySelector("[data-stat-rows]");
    const statRowTemplate = form.querySelector("[data-stat-row-template]");

    const priceRowsRoot = form.querySelector("[data-price-rows]");
    const priceRowTemplate = form.querySelector("[data-price-row-template]");

    const manaCostInput = form.querySelector("[data-mana-cost-input]");
    const manaCostStatus = form.querySelector("[data-mana-cost-status]");
    const manaCostUndo = form.querySelector("[data-mana-cost-undo]");
    const manaCostClear = form.querySelector("[data-mana-cost-clear]");
    const manaSymbolButtons = Array.from(form.querySelectorAll("[data-mana-symbol]"));
    const colorInputs = Array.from(form.querySelectorAll('input[name="color"]'));
    const colorlessInput = form.querySelector('input[name="color"][value="C"]');
    const rarityInputs = Array.from(form.querySelectorAll('input[name="rarity"]'));
    const rarityAnyInput = form.querySelector("[data-rarity-any]");
    const rawTypeOptions = readFormJSON(form, "[data-card-search-type-options]");
    const rawTypeFilters = readFormJSON(form, "[data-card-search-type-filters]");
    const rawSetOptions = readFormJSON(form, "[data-card-search-set-options]");

    const typeOptions = Array.from(new Set((Array.isArray(rawTypeOptions) ? rawTypeOptions : []).map(function (option) {
      return String(option || "").trim();
    }).filter(Boolean)));
    const setOptions = Array.from(new Set((Array.isArray(rawSetOptions) ? rawSetOptions : []).map(function (option) {
      return String(option || "").trim();
    }).filter(Boolean)));

    let typeFilters = Array.isArray(rawTypeFilters) ? rawTypeFilters : [];
    let visibleTypeOptions = [];
    let activeTypeIndex = -1;
    let visibleSetOptions = [];
    let activeSetIndex = -1;

    function trimValue(value) {
      return String(value || "").trim();
    }

    function lowerValue(value) {
      return trimValue(value).toLowerCase();
    }

    function setExpanded(inputEl, expanded) {
      if (!inputEl) return;
      inputEl.setAttribute("aria-expanded", expanded ? "true" : "false");
    }

    function operatorSymbol(value) {
      switch (trimValue(value).toLowerCase()) {
      case "lt":
        return "<";
      case "gt":
        return ">";
      case "lte":
        return "<=";
      case "gte":
        return ">=";
      case "neq":
        return "!=";
      default:
        return "=";
      }
    }

    function colorModeLabel(value) {
      switch (trimValue(value).toLowerCase()) {
      case "exact":
        return "exact";
      case "at_most":
        return "at most";
      default:
        return "includes";
      }
    }

    function selectedOptionLabel(selectEl, fallback) {
      if (!selectEl || selectEl.selectedIndex < 0) return fallback || "";
      return trimValue(selectEl.options[selectEl.selectedIndex].textContent) || fallback || "";
    }

    function rankedMatches(options, query, excludeSet) {
      const raw = lowerValue(query);
      const excluded = excludeSet || new Set();
      const exact = [];
      const prefix = [];
      const contains = [];
      const all = [];

      options.forEach(function (option) {
        const value = trimValue(option);
        const lower = value.toLowerCase();
        if (!value || excluded.has(lower)) {
          return;
        }
        if (!raw) {
          all.push(value);
          return;
        }
        if (lower === raw) {
          exact.push(value);
          return;
        }
        if (lower.startsWith(raw)) {
          prefix.push(value);
          return;
        }
        if (lower.includes(raw)) {
          contains.push(value);
        }
      });

      return raw ? exact.concat(prefix, contains) : all;
    }

    function setTypeStatus(message) {
      if (!typeStatus) return;
      typeStatus.textContent = message || "";
    }

    function manaCostTokens() {
      if (!manaCostInput) return [];
      return String(manaCostInput.value || "").match(/\{[^{}]+\}/g) || [];
    }

    function announceManaCost(message) {
      if (!manaCostStatus) return;
      manaCostStatus.textContent = "";
      window.requestAnimationFrame(function () {
        manaCostStatus.textContent = message;
      });
    }

    function refreshManaCostControls() {
      const hasValue = Boolean(manaCostInput && trimValue(manaCostInput.value));
      if (manaCostUndo) manaCostUndo.disabled = !hasValue;
      if (manaCostClear) manaCostClear.disabled = !hasValue;
    }

    function setManaCostTokens(tokens, announcement) {
      if (!manaCostInput) return;
      manaCostInput.value = tokens.join("");
      refreshManaCostControls();
      manaCostInput.dispatchEvent(new Event("input", { bubbles: true }));
      if (announcement) announceManaCost(announcement);
    }

    function addManaSymbol(symbol) {
      if (!symbol) return;
      const tokens = manaCostTokens();
      if (symbol === "{1}") {
        const genericIndex = tokens.findIndex(function (token) {
          return /^\{\d+\}$/.test(token);
        });
        if (genericIndex >= 0) {
          const nextValue = Number(tokens[genericIndex].slice(1, -1)) + 1;
          tokens[genericIndex] = "{" + nextValue + "}";
        } else {
          tokens.unshift("{1}");
        }
      } else {
        tokens.push(symbol);
      }
      setManaCostTokens(tokens, "Mana cost is now " + tokens.join(""));
    }

    function undoManaSymbol() {
      const tokens = manaCostTokens();
      if (!tokens.length) return;
      const lastIndex = tokens.length - 1;
      const last = tokens[lastIndex];
      if (/^\{\d+\}$/.test(last) && Number(last.slice(1, -1)) > 1) {
        tokens[lastIndex] = "{" + (Number(last.slice(1, -1)) - 1) + "}";
      } else {
        tokens.pop();
      }
      setManaCostTokens(tokens, tokens.length ? "Mana cost is now " + tokens.join("") : "Mana cost cleared");
    }

    function hideTypeMenu() {
      visibleTypeOptions = [];
      activeTypeIndex = -1;
      if (typeMenu) {
        typeMenu.classList.add("hidden");
        typeMenu.innerHTML = "";
      }
      if (typeInput) {
        typeInput.removeAttribute("aria-activedescendant");
      }
      setExpanded(typeInput, false);
    }

    function hideSetMenu() {
      visibleSetOptions = [];
      activeSetIndex = -1;
      if (setMenu) {
        setMenu.classList.add("hidden");
        setMenu.innerHTML = "";
      }
      if (setInput) {
        setInput.removeAttribute("aria-activedescendant");
      }
      setExpanded(setInput, false);
    }

    function selectedTypeSet() {
      const out = new Set();
      typeFilters.forEach(function (filter) {
        out.add(lowerValue(filter.value));
      });
      return out;
    }

    function canonicalType(value) {
      const raw = lowerValue(value);
      if (!raw) return "";
      for (const option of typeOptions) {
        if (option.toLowerCase() === raw) {
          return option;
        }
      }
      return "";
    }

    function renderTypeMenu() {
      if (!typeMenu || !typeInput) return;

      typeMenu.innerHTML = "";
      if (!visibleTypeOptions.length) {
        const empty = document.createElement("div");
        empty.className = "mt-advanced-search-menu__empty";
        empty.textContent = trimValue(typeInput.value) ? "No matching card types." : "All suggested types are already selected.";
        typeMenu.appendChild(empty);
        typeMenu.classList.remove("hidden");
        typeInput.removeAttribute("aria-activedescendant");
        setExpanded(typeInput, true);
        return;
      }

      visibleTypeOptions.forEach(function (option, index) {
        const button = document.createElement("button");
        button.type = "button";
        button.setAttribute("role", "option");
        button.setAttribute("aria-selected", index === activeTypeIndex ? "true" : "false");
        button.id = "advanced-type-option-" + index;
        button.className = "mt-advanced-search-option" + (index === activeTypeIndex ? " is-active" : "");
        button.textContent = option;
        button.addEventListener("click", function (event) {
          event.preventDefault();
          addTypeFilter(option);
        });
        typeMenu.appendChild(button);
      });

      typeMenu.classList.remove("hidden");
      typeInput.setAttribute("aria-activedescendant", "advanced-type-option-" + activeTypeIndex);
      setExpanded(typeInput, true);
    }

    function refreshTypeMenu() {
      visibleTypeOptions = rankedMatches(typeOptions, typeInput ? typeInput.value : "", selectedTypeSet());
      if (!visibleTypeOptions.length) {
        activeTypeIndex = -1;
      } else if (activeTypeIndex < 0 || activeTypeIndex >= visibleTypeOptions.length) {
        activeTypeIndex = 0;
      }
      renderTypeMenu();
    }

    function renderTypeFilters() {
      if (!typeSelected || !typeHidden) return;

      typeSelected.innerHTML = "";
      typeHidden.innerHTML = "";

      typeFilters.forEach(function (filter, index) {
        const chip = document.createElement("div");
        chip.className = "mt-advanced-search-type-chip";

        const value = document.createElement("span");
        value.textContent = filter.value;
        chip.appendChild(value);

        const toggle = document.createElement("button");
        toggle.type = "button";
        toggle.textContent = filter.mode === "not" ? "NOT" : "IS";
        toggle.className = "mt-advanced-search-type-chip__mode " +
          (filter.mode === "not" ? "mt-advanced-search-type-chip__mode--exclude" : "mt-advanced-search-type-chip__mode--include");
        toggle.setAttribute(
          "aria-label",
          (filter.mode === "not" ? "Exclude " : "Include ") + filter.value + ". Select to toggle."
        );
        toggle.addEventListener("click", function () {
          typeFilters[index].mode = typeFilters[index].mode === "not" ? "is" : "not";
          renderTypeFilters();
          updateSearchSummary();
        });
        chip.appendChild(toggle);

        const remove = document.createElement("button");
        remove.type = "button";
        remove.textContent = "×";
        remove.className = "mt-advanced-search-type-chip__remove";
        remove.setAttribute("aria-label", "Remove " + filter.value + " type filter");
        remove.addEventListener("click", function () {
          typeFilters.splice(index, 1);
          renderTypeFilters();
          refreshTypeMenu();
          updateSearchSummary();
        });
        chip.appendChild(remove);

        typeSelected.appendChild(chip);

        const valueInput = document.createElement("input");
        valueInput.type = "hidden";
        valueInput.name = "type_value";
        valueInput.value = filter.value;
        typeHidden.appendChild(valueInput);

        const modeInput = document.createElement("input");
        modeInput.type = "hidden";
        modeInput.name = "type_mode";
        modeInput.value = filter.mode;
        typeHidden.appendChild(modeInput);
      });
    }

    function addTypeFilter(rawValue) {
      const value = canonicalType(rawValue);
      if (!value) {
        setTypeStatus("Choose a suggested type.");
        return false;
      }
      const exists = typeFilters.some(function (filter) {
        return lowerValue(filter.value) === lowerValue(value);
      });
      if (exists) {
        setTypeStatus("That type is already added.");
        return false;
      }
      typeFilters.push({ value: value, mode: "is" });
      if (typeInput) {
        typeInput.value = "";
        typeInput.focus();
      }
      setTypeStatus("");
      renderTypeFilters();
      refreshTypeMenu();
      updateSearchSummary();
      return true;
    }

    function commitPendingType() {
      if (!typeInput) return false;
      const raw = trimValue(typeInput.value);
      if (!raw) return false;

      const candidate = (visibleTypeOptions.length && activeTypeIndex >= 0 && activeTypeIndex < visibleTypeOptions.length)
        ? visibleTypeOptions[activeTypeIndex]
        : canonicalType(raw) || rankedMatches(typeOptions, raw, selectedTypeSet())[0] || "";

      if (!candidate) {
        setTypeStatus("Choose a suggested type.");
        return false;
      }
      return addTypeFilter(candidate);
    }

    function renderSetMenu() {
      if (!setMenu || !setInput) return;

      setMenu.innerHTML = "";
      if (!visibleSetOptions.length) {
        const empty = document.createElement("div");
        empty.className = "mt-advanced-search-menu__empty";
        empty.textContent = trimValue(setInput.value) ? "No matching sets." : "No set suggestions available.";
        setMenu.appendChild(empty);
        setMenu.classList.remove("hidden");
        setInput.removeAttribute("aria-activedescendant");
        setExpanded(setInput, true);
        return;
      }

      visibleSetOptions.forEach(function (option, index) {
        const button = document.createElement("button");
        button.type = "button";
        button.setAttribute("role", "option");
        button.setAttribute("aria-selected", index === activeSetIndex ? "true" : "false");
        button.id = "advanced-set-option-" + index;
        button.className = "mt-advanced-search-option" + (index === activeSetIndex ? " is-active" : "");
        button.textContent = option;
        button.addEventListener("click", function (event) {
          event.preventDefault();
          chooseSet(option);
        });
        setMenu.appendChild(button);
      });

      setMenu.classList.remove("hidden");
      setInput.setAttribute("aria-activedescendant", "advanced-set-option-" + activeSetIndex);
      setExpanded(setInput, true);
    }

    function refreshSetMenu() {
      visibleSetOptions = rankedMatches(setOptions, setInput ? setInput.value : "");
      if (!visibleSetOptions.length) {
        activeSetIndex = -1;
      } else if (activeSetIndex < 0 || activeSetIndex >= visibleSetOptions.length) {
        activeSetIndex = 0;
      }
      renderSetMenu();
    }

    function chooseSet(value) {
      if (!setInput) return;
      setInput.value = value;
      hideSetMenu();
      updateSearchSummary();
    }

    function statRows() {
      return statRowsRoot ? Array.from(statRowsRoot.querySelectorAll("[data-stat-row]")) : [];
    }

    function statRowHasValue(row) {
      const valueInput = row ? row.querySelector("[data-stat-value]") : null;
      return Boolean(trimValue(valueInput ? valueInput.value : ""));
    }

    function appendStatRow() {
      if (!statRowsRoot || !statRowTemplate || !statRowTemplate.content) return null;
      const fragment = statRowTemplate.content.cloneNode(true);
      const row = fragment.querySelector("[data-stat-row]");
      statRowsRoot.appendChild(fragment);
      return row;
    }

    function ensureTrailingStatRow() {
      if (!statRowsRoot) return;
      let rows = statRows();
      if (!rows.length) {
        appendStatRow();
        rows = statRows();
      }
      if (statRowHasValue(rows[rows.length - 1])) {
        appendStatRow();
      }
    }

    function resetStatRow(row) {
      if (!row) return;
      const statSelect = row.querySelector("[data-stat-select]");
      const statOperator = row.querySelector("[data-stat-operator]");
      const statValueInput = row.querySelector("[data-stat-value]");
      if (statSelect) statSelect.value = "mana_value";
      if (statOperator) statOperator.value = "eq";
      if (statValueInput) statValueInput.value = "";
    }

    function removeStatRow(row) {
      if (!row || !statRowsRoot || !statRowsRoot.contains(row)) return;
      if (statRows().length > 1) {
        row.remove();
      } else {
        resetStatRow(row);
      }
      ensureTrailingStatRow();
    }

    function priceRows() {
      return priceRowsRoot ? Array.from(priceRowsRoot.querySelectorAll("[data-price-row]")) : [];
    }

    function priceRowHasValue(row) {
      const valueInput = row ? row.querySelector("[data-price-value]") : null;
      return Boolean(trimValue(valueInput ? valueInput.value : ""));
    }

    function appendPriceRow() {
      if (!priceRowsRoot || !priceRowTemplate || !priceRowTemplate.content) return null;
      const fragment = priceRowTemplate.content.cloneNode(true);
      const row = fragment.querySelector("[data-price-row]");
      priceRowsRoot.appendChild(fragment);
      return row;
    }

    function ensureTrailingPriceRow() {
      if (!priceRowsRoot) return;
      let rows = priceRows();
      if (!rows.length) {
        appendPriceRow();
        rows = priceRows();
      }
      if (priceRowHasValue(rows[rows.length - 1])) {
        appendPriceRow();
      }
    }

    function resetPriceRow(row) {
      if (!row) return;
      const priceOperator = row.querySelector("[data-price-operator]");
      const priceValueInput = row.querySelector("[data-price-value]");
      if (priceOperator) priceOperator.value = "eq";
      if (priceValueInput) priceValueInput.value = "";
    }

    function removePriceRow(row) {
      if (!row || !priceRowsRoot || !priceRowsRoot.contains(row)) return;
      if (priceRows().length > 1) {
        row.remove();
      } else {
        resetPriceRow(row);
      }
      ensureTrailingPriceRow();
    }

    function numericRangeImpossible(constraints) {
      let lower = Number.NEGATIVE_INFINITY;
      let upper = Number.POSITIVE_INFINITY;
      let lowerInclusive = true;
      let upperInclusive = true;
      const excluded = new Set();

      constraints.forEach(function (constraint) {
        const operator = trimValue(constraint.operator).toLowerCase();
        const value = constraint.value;
        if (operator === "lt" || operator === "lte") {
          const inclusive = operator === "lte";
          if (value < upper) {
            upper = value;
            upperInclusive = inclusive;
          } else if (value === upper) {
            upperInclusive = upperInclusive && inclusive;
          }
          return;
        }
        if (operator === "gt" || operator === "gte") {
          const inclusive = operator === "gte";
          if (value > lower) {
            lower = value;
            lowerInclusive = inclusive;
          } else if (value === lower) {
            lowerInclusive = lowerInclusive && inclusive;
          }
          return;
        }
        if (operator === "neq") {
          excluded.add(value);
          return;
        }
        if (value > lower) {
          lower = value;
          lowerInclusive = true;
        }
        if (value < upper) {
          upper = value;
          upperInclusive = true;
        }
      });

      if (lower > upper) return true;
      if (lower !== upper) return false;
      return !lowerInclusive || !upperInclusive || excluded.has(lower);
    }

    function completedNumericConstraint(row, operatorSelector, valueSelector) {
      const operatorInput = row ? row.querySelector(operatorSelector) : null;
      const valueInput = row ? row.querySelector(valueSelector) : null;
      const rawValue = trimValue(valueInput ? valueInput.value : "");
      if (!rawValue) return null;
      const value = Number(rawValue);
      if (!Number.isFinite(value)) return null;
      return {
        operator: operatorInput ? operatorInput.value : "eq",
        value: value
      };
    }

    function searchHasImpossibleRange() {
      const statsByName = new Map();
      statRows().forEach(function (row) {
        const constraint = completedNumericConstraint(row, "[data-stat-operator]", "[data-stat-value]");
        if (!constraint) return;
        const statSelect = row.querySelector("[data-stat-select]");
        const statName = statSelect ? statSelect.value : "mana_value";
        if (!statsByName.has(statName)) statsByName.set(statName, []);
        statsByName.get(statName).push(constraint);
      });
      for (const constraints of statsByName.values()) {
        if (numericRangeImpossible(constraints)) return true;
      }

      const priceConstraints = priceRows().map(function (row) {
        return completedNumericConstraint(row, "[data-price-operator]", "[data-price-value]");
      }).filter(Boolean);
      return numericRangeImpossible(priceConstraints);
    }

    function updateRangeState() {
      const impossible = searchHasImpossibleRange();
      if (rangeError) {
        rangeError.hidden = !impossible;
        rangeError.classList.toggle("is-visible", impossible);
      }
      if (searchSubmit) {
        searchSubmit.disabled = impossible;
        searchSubmit.setAttribute("aria-disabled", impossible ? "true" : "false");
      }
      return impossible;
    }

    function renderSummaryChips(items) {
      if (!summaryChips) return;
      summaryChips.innerHTML = "";
      items.forEach(function (item, index) {
        const chip = document.createElement("button");
        chip.type = "button";
        chip.className = "mt-advanced-search-summary-chip";
        chip.setAttribute("data-search-summary-remove", "");
        chip.setAttribute("aria-label", "Remove filter: " + item.label);

        const label = document.createElement("span");
        label.className = "mt-advanced-search-summary-chip__label";
        label.textContent = item.label;
        chip.appendChild(label);

        const remove = document.createElement("span");
        remove.className = "mt-advanced-search-summary-chip__remove";
        remove.setAttribute("aria-hidden", "true");
        remove.textContent = "×";
        chip.appendChild(remove);

        chip.addEventListener("click", function () {
          if (typeof item.remove === "function") {
            item.remove();
          }
          updateSearchSummary();
          window.requestAnimationFrame(function () {
            const remaining = Array.from(summaryChips.querySelectorAll("[data-search-summary-remove]"));
            if (remaining.length) {
              remaining[Math.min(index, remaining.length - 1)].focus({ preventScroll: true });
              return;
            }
            if (summaryCount) {
              summaryCount.setAttribute("tabindex", "-1");
              summaryCount.focus({ preventScroll: true });
            }
          });
        });
        summaryChips.appendChild(chip);
      });
    }

    function updateSearchSummary() {
      const items = [];
      function addSummaryItem(label, remove) {
        items.push({ label: label, remove: remove });
      }

      const nameInput = form.querySelector('input[name="q"]');
      const textInput = form.querySelector('input[name="text"]');
      const typePartialInput = form.querySelector('input[name="type_partial"]');
      const colorModeInput = form.querySelector('select[name="color_mode"]');
      const layoutInput = form.querySelector('select[name="layout"]');
      const artistInput = form.querySelector('input[name="artist"]');

      const colorLabels = {
        W: "White",
        U: "Blue",
        B: "Black",
        R: "Red",
        G: "Green",
        C: "Colorless"
      };
      const rarityLabels = {
        common: "Common",
        uncommon: "Uncommon",
        rare: "Rare",
        mythic: "Mythic"
      };

      if (trimValue(nameInput ? nameInput.value : "")) {
        addSummaryItem("Name: " + trimValue(nameInput.value), function () {
          nameInput.value = "";
        });
      }
      if (trimValue(manaCostInput ? manaCostInput.value : "")) {
        addSummaryItem("Mana Cost: " + trimValue(manaCostInput.value), function () {
          manaCostInput.value = "";
          refreshManaCostControls();
          announceManaCost("Mana cost cleared");
        });
      }
      if (trimValue(textInput ? textInput.value : "")) {
        addSummaryItem("Rules text: " + trimValue(textInput.value), function () {
          textInput.value = "";
        });
      }

      typeFilters.forEach(function (filter) {
        const filterValue = trimValue(filter.value);
        addSummaryItem("Type " + (filter.mode === "not" ? "NOT " : "IS ") + filterValue, function () {
          const currentIndex = typeFilters.findIndex(function (candidate) {
            return lowerValue(candidate.value) === lowerValue(filterValue);
          });
          if (currentIndex >= 0) {
            typeFilters.splice(currentIndex, 1);
          }
          if (!typeFilters.length && typePartialInput) {
            typePartialInput.checked = false;
          }
          renderTypeFilters();
          refreshTypeMenu();
        });
      });
      if (typePartialInput && typePartialInput.checked && typeFilters.length) {
        addSummaryItem("Allow partial type matches", function () {
          typePartialInput.checked = false;
        });
      }

      colorInputs.filter(function (inputEl) {
        return inputEl.checked;
      }).forEach(function (inputEl) {
        const label = colorLabels[inputEl.value] || inputEl.value;
        addSummaryItem("Colors (" + colorModeLabel(colorModeInput ? colorModeInput.value : "") + "): " + label, function () {
          inputEl.checked = false;
          if (!colorInputs.some(function (candidate) { return candidate.checked; }) && colorModeInput) {
            colorModeInput.value = "includes";
          }
        });
      });

      rarityInputs.filter(function (inputEl) {
        return inputEl.checked;
      }).forEach(function (inputEl) {
        const label = rarityLabels[inputEl.value] || trimValue(inputEl.value);
        addSummaryItem("Rarity: " + label, function () {
          inputEl.checked = false;
          syncRaritySelection(inputEl);
        });
      });
      if (layoutInput && trimValue(layoutInput.value)) {
        addSummaryItem("Layout: " + selectedOptionLabel(layoutInput, trimValue(layoutInput.value)), function () {
          layoutInput.value = "";
        });
      }
      if (trimValue(setInput ? setInput.value : "")) {
        addSummaryItem("Set: " + trimValue(setInput.value), function () {
          setInput.value = "";
          hideSetMenu();
        });
      }
      if (trimValue(artistInput ? artistInput.value : "")) {
        addSummaryItem("Artist: " + trimValue(artistInput.value), function () {
          artistInput.value = "";
        });
      }
      statRows().forEach(function (row) {
        const statSelect = row.querySelector("[data-stat-select]");
        const statOperator = row.querySelector("[data-stat-operator]");
        const statValueInput = row.querySelector("[data-stat-value]");
        const value = trimValue(statValueInput ? statValueInput.value : "");
        if (!value) return;
        addSummaryItem(selectedOptionLabel(statSelect, "Mana Value") + " " + operatorSymbol(statOperator ? statOperator.value : "eq") + " " + value, function () {
          removeStatRow(row);
        });
      });
      priceRows().forEach(function (row) {
        const priceOperator = row.querySelector("[data-price-operator]");
        const priceValueInput = row.querySelector("[data-price-value]");
        const value = trimValue(priceValueInput ? priceValueInput.value : "");
        if (!value) return;
        addSummaryItem("Price " + operatorSymbol(priceOperator ? priceOperator.value : "eq") + " $" + value, function () {
          removePriceRow(row);
        });
      });

      if (summaryCount) {
        summaryCount.textContent = items.length
          ? items.length + " active filter" + (items.length === 1 ? "" : "s")
          : "No filters selected";
      }
      renderSummaryChips(items);
      updateRangeState();
    }

    if (typeInput) {
      typeInput.addEventListener("focus", function () {
        hideSetMenu();
        setTypeStatus("");
        refreshTypeMenu();
      });
      typeInput.addEventListener("click", function () {
        hideSetMenu();
        setTypeStatus("");
        refreshTypeMenu();
      });
      typeInput.addEventListener("input", function () {
        setTypeStatus("");
        activeTypeIndex = 0;
        refreshTypeMenu();
      });
      typeInput.addEventListener("keydown", function (event) {
        if (event.key === "Escape") {
          event.preventDefault();
          typeInput.value = "";
          setTypeStatus("");
          hideTypeMenu();
          return;
        }
        if (event.key === "ArrowDown") {
          event.preventDefault();
          if (!visibleTypeOptions.length) {
            refreshTypeMenu();
          }
          if (visibleTypeOptions.length) {
            activeTypeIndex = activeTypeIndex < visibleTypeOptions.length - 1 ? activeTypeIndex + 1 : 0;
            renderTypeMenu();
          }
          return;
        }
        if (event.key === "ArrowUp") {
          event.preventDefault();
          if (!visibleTypeOptions.length) {
            refreshTypeMenu();
          }
          if (visibleTypeOptions.length) {
            activeTypeIndex = activeTypeIndex > 0 ? activeTypeIndex - 1 : visibleTypeOptions.length - 1;
            renderTypeMenu();
          }
          return;
        }
        if (event.key !== "Enter") return;
        event.preventDefault();
        commitPendingType();
      });
    }

    if (setInput) {
      setInput.addEventListener("focus", function () {
        hideTypeMenu();
        refreshSetMenu();
      });
      setInput.addEventListener("click", function () {
        hideTypeMenu();
        refreshSetMenu();
      });
      setInput.addEventListener("input", function () {
        activeSetIndex = 0;
        refreshSetMenu();
        updateSearchSummary();
      });
      setInput.addEventListener("keydown", function (event) {
        if (event.key === "Escape") {
          event.preventDefault();
          hideSetMenu();
          return;
        }
        if (event.key === "ArrowDown") {
          event.preventDefault();
          if (!visibleSetOptions.length) {
            refreshSetMenu();
          }
          if (visibleSetOptions.length) {
            activeSetIndex = activeSetIndex < visibleSetOptions.length - 1 ? activeSetIndex + 1 : 0;
            renderSetMenu();
          }
          return;
        }
        if (event.key === "ArrowUp") {
          event.preventDefault();
          if (!visibleSetOptions.length) {
            refreshSetMenu();
          }
          if (visibleSetOptions.length) {
            activeSetIndex = activeSetIndex > 0 ? activeSetIndex - 1 : visibleSetOptions.length - 1;
            renderSetMenu();
          }
          return;
        }
        if (event.key === "Enter" && visibleSetOptions.length) {
          event.preventDefault();
          chooseSet(visibleSetOptions[activeSetIndex >= 0 ? activeSetIndex : 0]);
        }
      });
    }

    if (statRowsRoot) {
      statRowsRoot.addEventListener("input", function (event) {
        if (!event.target.matches("[data-stat-value]")) return;
        ensureTrailingStatRow();
        updateSearchSummary();
      });
      statRowsRoot.addEventListener("click", function (event) {
        const removeButton = event.target.closest("[data-remove-stat-filter]");
        if (!removeButton || !statRowsRoot.contains(removeButton)) return;
        const row = removeButton.closest("[data-stat-row]");
        if (!row) return;
        removeStatRow(row);
        updateSearchSummary();
      });
    }

    if (priceRowsRoot) {
      priceRowsRoot.addEventListener("input", function (event) {
        if (!event.target.matches("[data-price-value]")) return;
        ensureTrailingPriceRow();
        updateSearchSummary();
      });
      priceRowsRoot.addEventListener("click", function (event) {
        const removeButton = event.target.closest("[data-remove-price-filter]");
        if (!removeButton || !priceRowsRoot.contains(removeButton)) return;
        const row = removeButton.closest("[data-price-row]");
        if (!row) return;
        removePriceRow(row);
        updateSearchSummary();
      });
    }

    if (form) {
      form.addEventListener("submit", function (event) {
        if (updateRangeState()) {
          event.preventDefault();
          return;
        }
        if (!typeInput || !trimValue(typeInput.value)) {
          return;
        }
        event.preventDefault();
        if (commitPendingType()) {
          window.requestAnimationFrame(function () {
            if (typeof form.requestSubmit === "function") {
              form.requestSubmit();
            } else {
              form.submit();
            }
          });
        }
      });

      form.addEventListener("input", function () {
        updateSearchSummary();
      });
      form.addEventListener("change", function () {
        updateSearchSummary();
      });
    }

    document.addEventListener("click", function (event) {
      if (!typeRoot || !typeRoot.contains(event.target)) {
        hideTypeMenu();
      }
      if (!setRoot || !setRoot.contains(event.target)) {
        hideSetMenu();
      }
    });

    colorInputs.forEach(function (inputEl) {
      inputEl.addEventListener("change", function () {
        if (!colorlessInput) return;
        if (inputEl.value === "C" && inputEl.checked) {
          colorInputs.forEach(function (other) {
            if (other !== inputEl) other.checked = false;
          });
          return;
        }
        if (inputEl.value !== "C" && inputEl.checked) {
          colorlessInput.checked = false;
        }
      });
    });

    function syncRaritySelection(changedInput) {
      if (!rarityAnyInput) return;
      if (changedInput === rarityAnyInput && rarityAnyInput.checked) {
        rarityInputs.forEach(function (inputEl) {
          inputEl.checked = false;
        });
        return;
      }
      const hasSpecificRarity = rarityInputs.some(function (inputEl) {
        return inputEl.checked;
      });
      rarityAnyInput.checked = !hasSpecificRarity;
    }

    if (rarityAnyInput) {
      rarityAnyInput.addEventListener("change", function () {
        syncRaritySelection(rarityAnyInput);
      });
    }
    rarityInputs.forEach(function (inputEl) {
      inputEl.addEventListener("change", function () {
        syncRaritySelection(inputEl);
      });
    });

    manaSymbolButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        addManaSymbol(button.getAttribute("data-mana-symbol") || "");
      });
    });

    if (manaCostUndo) {
      manaCostUndo.addEventListener("click", undoManaSymbol);
    }

    if (manaCostClear) {
      manaCostClear.addEventListener("click", function () {
        setManaCostTokens([], "Mana cost cleared");
      });
    }

    renderTypeFilters();
    hideTypeMenu();
    hideSetMenu();
    refreshManaCostControls();
    ensureTrailingStatRow();
    ensureTrailingPriceRow();
    syncRaritySelection();
    updateSearchSummary();
  }

  function initializeCardSearchForms() {
    document.querySelectorAll("[data-card-search-form]").forEach(setupCardSearchForm);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initializeCardSearchForms, { once: true });
  } else {
    initializeCardSearchForms();
  }
})();
