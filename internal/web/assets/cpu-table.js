'use strict';
// Presentation only: every action still uses the current Forge prompt and IDs.
(() => {
  const $ = id => document.getElementById(id);
  const el = (tag, text = '', className = '') => { const n = document.createElement(tag); n.textContent = text; n.className = className; return n; };
  // Render only brace-delimited game symbols; all other engine prose stays text.
  function symbolText(tag, text = '', className = '') {
    const node = el(tag, '', className), raw = String(text || '');
    let start = 0;
    for (const match of raw.matchAll(/\{([A-Z0-9/∞]+)\}/gi)) {
      if (match.index > start) node.append(document.createTextNode(raw.slice(start, match.index)));
      const symbol = match[1].toUpperCase(), img = el('img', '', 'game-symbol');
      img.src = 'https://svgs.scryfall.io/card-symbols/' + encodeURIComponent(symbol.replaceAll('/', '')) + '.svg';
      img.alt = img.title = '{' + symbol + '}'; img.decoding = 'async';
      img.onerror = () => img.replaceWith(document.createTextNode(match[0]));
      node.append(img); start = match.index + match[0].length;
    }
    if (!start) node.textContent = raw;
    else if (start < raw.length) node.append(document.createTextNode(raw.slice(start)));
    return node;
  }
  // Consistent silhouettes stay recognizable without color or text labels.
  function icon(name, className = '') {
    const paths = {
      Library: ['M7 3h12v16H7z', 'M4 6v15h12', 'M10 7h6M10 10h6'],
      Graveyard: ['M6 20V9a6 6 0 0 1 12 0v11', 'M4 20h16M12 7v7M9 10h6'],
      Exile: ['M12 2 22 12 12 22 2 12Z', 'M8 16 16 8M10 8h6v6'],
      Command: ['M3 6 7 10 12 3 17 10 21 6 19 18H5Z', 'M6 21h12'],
      Permanents: ['M4 3 15 14M3 7l4-4M13 16l3-3M14 15l6 6', 'M20 3 9 14M17 3l4 4M8 13l3 3M9 15l-6 6'],
      Lands: ['M2 19 9 5l5 9 3-5 5 10Z', 'M6 11l3 2 3-2'],
      Hand: ['M8 7h10v14H8z', 'M5 18 2 6l9-3 1 2M18 8l3 1-2 9'],
      Poison: ['M11 1h2l1 4a8 8 0 0 1 0 15l-1 3h-2l-1-3a8 8 0 0 1 0-15Z M10 7a5 5 0 0 0 0 10Z M14 7v10a5 5 0 0 0 0-10Z'],
      Experience: ['M1 4h7l3 2v5l-2 1v9l-6-5-1-6Z M23 4h-7l-3 2v5l2 1v9l6-5 1-6Z'],
    };
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    for (const [key, value] of Object.entries({viewBox:'0 0 24 24', fill:'none', stroke:'currentColor', 'stroke-width':'1.6', 'stroke-linecap':'round', 'stroke-linejoin':'round', 'aria-hidden':'true', focusable:'false', class:`table-icon ${className}`})) svg.setAttribute(key, value);
    for (const d of paths[name] || paths.Library) { const path = document.createElementNS('http://www.w3.org/2000/svg', 'path'); path.setAttribute('d', d); svg.append(path); }
    if (['Poison','Experience'].includes(name)) {svg.setAttribute('fill','currentColor'); svg.setAttribute('stroke','none'); svg.setAttribute('fill-rule','evenodd');}
    return svg;
  }
  const imageCache = new Map(), imageQueue = [];
  let imageBusy = false, imageBackoff = 0, previewSelection = null, openZone = null, zonePrompt = '', currentView = null, currentSend = null, currentEnabled = false;
  const phaseNames = {
    UNTAP: 'Untap', UPKEEP: 'Upkeep', DRAW: 'Draw', MAIN1: 'First main', MAIN2: 'Second main',
    COMBAT_BEGIN: 'Beginning of combat', COMBAT_DECLARE_ATTACKERS: 'Declare attackers',
    COMBAT_DECLARE_BLOCKERS: 'Declare blockers', COMBAT_FIRST_STRIKE_DAMAGE: 'First-strike damage',
    COMBAT_DAMAGE: 'Combat damage', COMBAT_END: 'End of combat', END_OF_TURN: 'End step', CLEANUP: 'Cleanup',
  };
  function phaseName(phase) { return phaseNames[phase] || String(phase || 'Preparing').replaceAll('_', ' ').toLowerCase(); }
  const stopPhases = [['UNTAP','UN'],['UPKEEP','UP'],['DRAW','DR'],['MAIN1','M1'],['COMBAT_BEGIN','BC'],['COMBAT_DECLARE_ATTACKERS','DA'],['COMBAT_DECLARE_BLOCKERS','DB'],['COMBAT_FIRST_STRIKE_DAMAGE','FS'],['COMBAT_DAMAGE','CD'],['COMBAT_END','EC'],['MAIN2','M2'],['END_OF_TURN','ES'],['CLEANUP','CL']];
  let changeStop = null, stopPending = false, combatLinks = new Map(), cardNames = new Map(), layoutFrame = null;
  function phaseStops(player, s) {
    const column = el('div', '', 'phase-stops'); column.setAttribute('aria-label', `Priority stops on ${player.name === 'Human' ? 'your' : 'the CPU’s'} turn`);
    for (const [phase, short] of stopPhases) {
      const button = el('button', short, 'phase-stop'); button.type = 'button';
      button.dataset.stopSeat = player.name; button.dataset.stopPhase = phase;
      const available = phase !== 'UNTAP';
      button.title = `${phaseName(phase)}${available ? ' — toggle your priority stop' : ' — no priority in untap'}`;
      button.setAttribute('aria-label', `${player.name === 'Human' ? 'Your' : 'CPU'} turn: ${phaseName(phase)}`);
      button.onclick = async () => {
        if (stopPending || !changeStop) return;
        stopPending = true; updateStops(currentView?.phaseStops);
        try { await changeStop(player.name, phase, button.getAttribute('aria-pressed') !== 'true'); }
        finally { stopPending = false; updateStops(currentView?.phaseStops); }
      };
      if (s.turnPlayer === player.name && s.phase === phase) button.setAttribute('aria-current', 'step');
      column.append(button);
    }
    return column;
  }
  function updateStops(stops) {
    if (currentView && stops) currentView.phaseStops = stops;
    for (const button of document.querySelectorAll('[data-stop-phase]')) {
      const phases = stops?.[button.dataset.stopSeat];
      button.setAttribute('aria-pressed', String(button.dataset.stopPhase !== 'UNTAP' && (phases ? phases.includes(button.dataset.stopPhase) : true)));
      button.disabled = stopPending || !stops || button.dataset.stopPhase === 'UNTAP' || ['finished','failed','cancelled','expired'].includes(currentView?.status);
    }
  }
  function manaPool(player, s, enabled, send) {
    const pool = el('div', '', 'mana-pool'); pool.setAttribute('aria-label', `${player.name === 'Human' ? 'Your' : 'CPU'} floating mana`);
    const colors = [['white','W'],['blue','U'],['black','B'],['red','R'],['green','G'],['colorless','C']];
    const values = player.mana || (player.name === 'Human' ? s.mana : []) || [];
    for (const [color, symbol] of colors) {
      const mana = values.find(m => m.label === color || m.label?.toLowerCase() === symbol.toLowerCase());
      const amount = mana?.amount || 0, spendable = enabled && player.name === 'Human' && (s.mana || []).some(m => m.id === mana?.id && m.amount > 0);
      const chip = el(spendable ? 'button' : 'span', '', `mana-chip mana-${color}`);
      chip.setAttribute('aria-label', `${amount} ${color} mana${spendable ? ', click to spend' : ''}`); chip.title = `${color} mana: ${amount}${spendable ? ' · click to spend' : ''}`;
      chip.dataset.empty = String(amount === 0); chip.append(symbolText('span', '{' + symbol + '}'), el('strong', String(amount)));
      if (spendable) { chip.type = 'button'; chip.onclick = () => send('mana', {id:mana.id}); }
      pool.append(chip);
    }
    for (const [key, label, symbol] of [['poison','poison','Poison'], ['energy','energy','{E}'], ['experience','experience','Experience']]) {
      const amount = Number.isInteger(player.counters?.[key]) && player.counters[key] >= 0 ? player.counters[key] : 0;
      const chip = el('span', '', `mana-chip player-counter counter-${key}`);
      chip.dataset.empty = String(amount === 0); chip.dataset.counter = key;
      chip.setAttribute('aria-label', `${amount} ${label} counters`); chip.title = `${label} counters: ${amount}`;
      chip.append(key === 'energy' ? symbolText('span', symbol) : icon(symbol, 'counter-icon'), el('strong', String(amount))); pool.append(chip);
    }
    for (const commander of player.commanders || []) {
      const tax = Number.isInteger(commander.casts) && commander.casts >= 0 ? commander.casts * 2 : 0;
      const chip = el('span', '', 'mana-chip commander-tax'); chip.dataset.empty = String(tax === 0);
      chip.setAttribute('aria-label', `${commander.name} commander tax: ${tax} additional generic mana`);
      chip.title = `${commander.name}: +${tax} generic mana to cast from the command zone`;
      chip.append(el('span','TAX'),el('strong',String(tax))); pool.append(chip);
    }
    return pool;
  }
  function queueLayout() {
    if (typeof requestAnimationFrame !== 'function') return;
    if (layoutFrame !== null) cancelAnimationFrame(layoutFrame);
    layoutFrame = requestAnimationFrame(() => { layoutFrame = null; fitCards(); drawCombat(); });
  }
  function fitCards() {
    if (!$('game') || $('game').hidden || !matchMedia('(min-width:851px)').matches) return;
    for (const group of document.querySelectorAll('#players .battlefield-row > .cards, #players .zone-hand > .cards, #zoneBrowserCards')) {
      const slots = [...group.children].filter(n => n.classList.contains('card-slot') || n.classList.contains('land-pile') || n.classList.contains('zone-hidden'));
      if (!slots.length) continue;
      const width = group.clientWidth, height = group.clientHeight;
      if (!width || !height) continue;
      const hand = group.parentElement.classList.contains('zone-hand');
      if (hand) {
        const cardWidth = Math.max(18, Math.min(154, (height - 12) / 1.4, width / Math.min(slots.length, 5)));
        const step = slots.length === 1 ? cardWidth : Math.min(cardWidth + 5, (width - cardWidth) / (slots.length - 1));
        group.style.setProperty('--fit-width', cardWidth + 'px'); group.style.setProperty('--hand-step', step + 'px');
        slots.forEach((slot,i) => { slot.style.left = (i * step) + 'px'; });
      } else {
        const ratio = slots.some(s => s.classList.contains('is-tapped') || s.dataset.tapped === 'true') ? 1.4 : 1;
        const pileScale = 1 + Math.max(0,...slots.map(s => (Number(s.dataset.pileSize) || 1)-1)) * .16;
        let best = 1, columns = 1;
        for (let cols = 1; cols <= slots.length; cols++) {
          const rows = Math.ceil(slots.length / cols);
          const candidate = Math.min(150, (width - (cols-1)*5) / cols / ratio, ((height-(rows-1)*5)/rows-12)/(1.4*pileScale));
          if (candidate > best) { best = candidate; columns = cols; }
        }
        group.style.setProperty('--fit-width', Math.max(1, best) + 'px');
        group.style.gridTemplateColumns = `repeat(${columns}, ${best * ratio}px)`;
      }
    }
  }
  function connectionGeometry(a, b, horizontal = false) {
    let start, end, control1, control2;
    if (horizontal) {
      const right = b.left + b.width / 2 >= a.left + a.width / 2, direction = right ? 1 : -1;
      start = {x: right ? a.right + 3 : a.left - 3, y: a.top + a.height / 2};
      end = {x: right ? b.left - 4 : b.right + 4, y: b.top + b.height / 2};
      const bend = Math.max(18, Math.min(100, Math.abs(end.x - start.x) * .35));
      control1 = {x: start.x + direction * bend, y: start.y};
      control2 = {x: end.x - direction * bend, y: end.y};
    } else {
      const down = b.top + b.height / 2 >= a.top + a.height / 2;
      start = {x: a.left + a.width / 2, y: down ? a.bottom : a.top};
      end = {x: b.left + b.width / 2, y: down ? b.top : b.bottom};
      control1 = {x: start.x, y: (start.y + end.y) / 2};
      control2 = {x: end.x, y: (start.y + end.y) / 2};
    }
    const dx = end.x - control2.x, dy = end.y - control2.y, length = Math.hypot(dx, dy) || 1;
    const ux = dx / length, uy = dy / length, back = {x: end.x - ux * 8, y: end.y - uy * 8};
    return {
      start, end,
      line: `M${start.x},${start.y} C${control1.x},${control1.y} ${control2.x},${control2.y} ${end.x},${end.y}`,
      arrow: `M${end.x},${end.y} L${back.x - uy * 4},${back.y + ux * 4} L${back.x + uy * 4},${back.y - ux * 4} Z`,
    };
  }
  function drawCombat() {
    const svg = $('combatArrows'); if (!svg?.getBoundingClientRect) return;
    svg.replaceChildren();
    const box = svg.getBoundingClientRect(); if (!box.width || !box.height) return;
    svg.setAttribute('viewBox', `0 0 ${box.width} ${box.height}`);
    const nodes = new Map([...document.querySelectorAll('#players [data-card-id], #zoneBrowserCards [data-card-id]')].map(n => [Number(n.dataset.cardId), n]));
    const playerNodes = new Map([...document.querySelectorAll('#players [data-player-id]')].map(n => [Number(n.dataset.playerId), n]));
    const connect = (from, to, kind) => {
      if (!from || !to) return;
      const a = from.getBoundingClientRect(), b = to.getBoundingClientRect();
      if (!a.width || !a.height || !b.width || !b.height) return;
      const bounds = from.closest('.rail-panel')?.getBoundingClientRect();
      if (bounds && (a.bottom <= bounds.top || a.top >= bounds.bottom)) return;
      const relative = r => ({left: r.left-box.left, right: r.right-box.left, top: r.top-box.top, bottom: r.bottom-box.top, width: r.width, height: r.height});
      const geometry = connectionGeometry(relative(a),relative(b),kind === 'target');
      const path = document.createElementNS('http://www.w3.org/2000/svg','path');
      path.setAttribute('d', geometry.line);
      path.setAttribute('class',`combat-link ${kind}`); svg.append(path);
      const arrow = document.createElementNS('http://www.w3.org/2000/svg','path');
      arrow.setAttribute('d',geometry.arrow); arrow.setAttribute('class',`combat-link ${kind} arrowhead`); svg.append(arrow);
    };
    for (const link of $('zoneBrowser').hidden ? currentView?.combat || [] : []) {
      const attacker = nodes.get(link.attacker);
      connect(attacker, link.defender?.type === 'player' ? playerNodes.get(link.defender.id) : nodes.get(link.defender?.id), 'attack');
      for (const id of link.blockers || []) connect(nodes.get(id),attacker,'block');
      for (const id of link.plannedBlockers || []) if (!(link.blockers || []).includes(id)) connect(nodes.get(id),attacker,'planned');
    }
    const stackNodes = new Map([...document.querySelectorAll('#stack [data-stack-index]')].map(n => [Number(n.dataset.stackIndex),n]));
    for (const [index,item] of (currentView?.stack || []).entries()) for (const target of item.targets || []) {
      const destination = target.type === 'player' ? playerNodes.get(target.id) : target.type === 'card' ? nodes.get(target.id) : null;
      if ($('zoneBrowser').hidden || $('zoneBrowserCards').contains(destination)) connect(stackNodes.get(index), destination, 'target');
    }
  }
  function phaseIndex(phase) {
    if (['UNTAP', 'UPKEEP', 'DRAW'].includes(phase)) return 0;
    if (phase === 'MAIN1') return 1;
    if (String(phase).startsWith('COMBAT')) return 2;
    if (phase === 'MAIN2') return 3;
    if (['END_OF_TURN', 'CLEANUP'].includes(phase)) return 4;
    return -1;
  }
  function guidance(s) {
    const kind = s.prompt?.kind || '';
    if (kind.includes('Mulligan')) return ['Your opening hand', 'Keep this hand or take a mulligan.'];
    if (kind.includes('PayMana')) return ['Pay the cost', 'Auto-pay, or select mana sources and spend floating mana beside your life total.'];
    if (kind.includes('Attack')) return ['Declare attackers', 'Select your attackers on the battlefield, then confirm.'];
    if (kind.includes('Block')) return ['Declare blockers', 'Choose the attacking creature and your blocker as requested below.'];
    if (kind.includes('PassPriority')) return [s.turnPlayer === 'Human' ? 'Your move' : 'Your response', 'Select a card to act, or pass priority to continue.'];
    if (kind === 'getAbilityToPlay') return ['Choose an action', 'Select the ability you want to use.'];
    if (kind === 'assignCombatDamage' || kind === 'assignGenericAmount') return ['Assign the amount', 'Distribute the total among the offered targets.'];
    if (s.prompt?.ordering) return ['Choose the order', 'Arrange the offered items in the order requested below.'];
    if (s.status === 'choice') return ['Make your choice', 'Select one of the offered options below.'];
    return ['Your decision', 'Follow the current instruction, then continue.'];
  }
  function safeImageURL(value) {
    try { const u = new URL(value); return u.protocol === 'https:' && !u.username && !u.password ? u.href : ''; } catch (_) { return ''; }
  }
  function resolveArtwork(card) {
    // Only identities already present in the authorized view are looked up.
    if (!card.name || card.name === 'Face-down card' || (card.id === undefined && !card.previewOnly)) return Promise.resolve('');
    const key = `${card.printing || card.name}:${!!card.backFace}:${card.token && !card.copy ? (card.tokenName || card.name) + (card.tokenPT || card.pt || '') + (card.tokenType || card.type || '') : ''}`;
    if (imageCache.has(key)) return imageCache.get(key);
    const task = new Promise(resolve => imageQueue.push({card, resolve}));
    imageCache.set(key, task);
    if (imageCache.size > 256) imageCache.delete(imageCache.keys().next().value);
    pumpArtwork();
    return task;
  }
  async function pumpArtwork() {
    if (imageBusy || !imageQueue.length) return;
    imageBusy = true;
    const {card, resolve} = imageQueue.shift();
    let url = '';
    try {
      if (Date.now() >= imageBackoff) {
        const token = card.token && !card.copy;
        const query = '/cards/resolve?q=' + encodeURIComponent(token ? (card.tokenName || card.name).replace(/ Token$/i, '') : card.name) + '&exact=1' + (card.printing ? '&printing=' + encodeURIComponent(card.printing) : '') + (token ? '&token=1&pt=' + encodeURIComponent(card.tokenPT || card.pt || '') + '&type=' + encodeURIComponent(card.tokenType || card.type || '') : '');
        const response = await fetch(query);
        if (response.status === 429) imageBackoff = Date.now() + Math.max(1000, (Number(response.headers.get('Retry-After')) || 60) * 1000);
        if (response.ok) {
          const data = await response.json();
          const face = (data.faces || []).find(f => f.name?.toLocaleLowerCase() === card.name.toLocaleLowerCase());
          // Never replace the visible transformed face with unrelated front-face art.
          const candidate = face?.image_uri || face?.image_uris?.normal || (!card.backFace && data.image_uris?.normal);
          url = safeImageURL(candidate);
        }
      }
    } catch (_) { /* Artwork is optional; the live Forge card remains usable. */ }
    resolve(url);
    // Leave room in the shared request budget for gameplay and normal site use.
    setTimeout(() => { imageBusy = false; pumpArtwork(); }, 650);
  }
  function imageFor(card) {
    // Hidden lazy images never enter the loading viewport; the queue already throttles lookups.
    const img = el('img', '', 'card-art'); img.alt = ''; img.loading = 'eager'; img.decoding = 'async'; img.referrerPolicy = 'no-referrer'; img.hidden = true;
    resolveArtwork(card).then(url => {
      if (!url || img.isConnected === false) return;
      img.onload = () => { img.hidden = false; }; img.onerror = () => { img.hidden = true; }; img.src = url;
    });
    return img;
  }
  function facts(card) {
    return [card.type, card.token ? 'Token' : '', card.copy ? 'Copy — live characteristics below' : '', card.backFace ? 'Transformed' : '',
      card.summoningSick ? 'Summoning sickness' : '', card.damage ? `${card.damage} damage marked` : '',
      card.attachedTo ? `Attached to ${card.attachedTo}` : '', ...(card.counters || []).map(c => `${c.count} ${c.name}`)].filter(Boolean);
  }
  function cardRules(card) {
    // Compare prose independently of Forge's newline/markup formatting. Keep
    // genuinely different live ability text, but never repeat the same clause.
    const clean = text => String(text || '').replace(/<\/?span\b[^>]*>/gi, '').replace(/\r/g, '').trim();
    const key = text => clean(text).replace(/\s+/g, ' ').toLowerCase();
    // Forge's live ability text is the desktop's rules display. Oracle is a
    // fallback, not a second copy (and may describe pre-modification rules).
    const text = clean(card.abilities) || clean(card.rules), parts = [], keys = [];
    for (const part of text.split(/\n\s*\n/).map(clean).filter(Boolean)) {
      const normalized = key(part);
      if (keys.includes(normalized)) continue;
      keys.push(normalized); parts.push(part);
    }
    return parts.join('\n\n');
  }
  function preview(card, selection = {kind: 'card', id: card.id, name: card.name}) {
    previewSelection = selection;
    const detail = $('cardDetail');
    detail.replaceChildren(el('h3', card.name), symbolText('p', [card.mana, card.pt].filter(Boolean).join(' · '), 'preview-mana'),
      el('p', facts(card).join(' · '), 'preview-facts'),
      symbolText('pre', cardRules(card), 'preview-rules'));
    const art = el('div', '', 'preview-card');
    art.append(el('strong', card.name), card.name === 'Face-down card' ? cardBack() : imageFor(card));
    $('cardPicture').replaceChildren(art);
  }
  function clearPreview() {
    previewSelection = null;
    $('cardDetail').replaceChildren(el('p', 'Hover over a card to see its details.', 'preview-hint'));
    const placeholder = el('span', 'ManaTomb', 'preview-placeholder');
    $('cardPicture').replaceChildren(placeholder);
  }
  function closeZoneBrowser(restoreFocus = false) {
    const previous = openZone; openZone = null;
    $('zoneBrowser').hidden = true; $('zoneBrowserCards').replaceChildren(); queueLayout();
    for (const button of document.querySelectorAll('[data-zone-player]')) {
      button.setAttribute('aria-expanded', 'false');
      if (restoreFocus && previous && button.dataset.zonePlayer === String(previous.player) && button.dataset.zoneName === previous.zone) button.focus();
    }
  }
  function renderZoneBrowser() {
    if (!openZone || !currentView) return;
    const player = currentView.players?.find(p => p.id === openZone.player);
    const zone = player?.zones?.find(z => z.zone === openZone.zone);
    if (!zone) { closeZoneBrowser(); return; }
    $('zoneBrowser').hidden = false; $('combatArrows').replaceChildren();
    $('zoneBrowser').dataset.zoneName = zone.zone;
    $('zoneBrowserTitle').textContent = `${player.name === 'Human' ? 'Your' : 'Opponent’s'} ${zone.zone.toLowerCase()} · ${zone.count}`;
    const cards = zone.cards || [], hidden = Math.max(0, zone.count - cards.length);
    $('zoneBrowserHint').textContent = hidden ? `${hidden} card${hidden === 1 ? ' is' : 's are'} hidden. Cards appear here when revealed by the game.` : cards.length ? 'Hover to preview. Select a card when the current decision asks for one.' : 'There are no cards in this zone.';
    const gallery = $('zoneBrowserCards'); gallery.replaceChildren();
    cards.forEach(card => gallery.append(cardNode(card, currentEnabled, currentSend)));
    if (hidden) {
      const back = el('div', '', 'zone-hidden'); back.append(cardBack(), el('strong', String(hidden)), el('small', 'Hidden cards')); gallery.append(back);
    }
    for (const button of document.querySelectorAll('[data-zone-player]')) button.setAttribute('aria-expanded', String(button.dataset.zonePlayer === String(openZone.player) && button.dataset.zoneName === openZone.zone));
    queueLayout();
  }
  function reset() { $('combatArrows').replaceChildren(); clearPreview(); closeZoneBrowser(); currentView = null; currentSend = null; zonePrompt = ''; }
  function cardBack() {
    const back = el('img', '', 'card-back'); back.src = 'https://cards.scryfall.io/back.png'; back.alt = 'Magic: The Gathering card back'; back.decoding = 'async'; back.referrerPolicy = 'no-referrer'; return back;
  }
  function displayedStats(card) {
    const match = /^(-?\d+)\/(-?\d+)$/.exec(card.pt || '');
    return match && Number.isFinite(card.damage) && card.damage > 0 ? `${match[1]}/${Number(match[2])-card.damage}` : card.pt;
  }
  function cardNode(card, enabled, send) {
    const wrap = el('div', '', 'card-slot' + (card.tapped ? ' is-tapped' : ''));
    const c = el('button', '', 'card' + (card.tapped ? ' tapped' : '') + (card.selected ? ' selected' : '') + (card.attacking ? ' attacking' : '') + (card.blocking ? ' blocking' : ''));
    enabled = enabled && card.selectable !== false;
    if (card.id !== undefined) c.dataset.cardId = String(card.id);
    c.type = 'button'; c.setAttribute('aria-disabled', String(!enabled || card.id === undefined));
    const blocks = combatLinks.has(card.id) ? 'Blocks ' + [...new Set(combatLinks.get(card.id))].map(id => cardNames.get(id) || 'attacking creature').join(', ') : '';
    c.setAttribute('aria-label', [card.name, card.reference, card.tapped ? 'tapped' : '', card.pt, card.damage ? `${card.damage} damage marked` : '', card.selected ? 'selected' : '', card.attacking ? 'attacking' : '', blocks || (card.blocking ? 'blocking' : '')].filter(Boolean).join(', '));
    c.onclick = () => { if (enabled && card.id !== undefined) send('card', {id: card.id}); };
    wrap.onmouseenter = () => preview(card); c.onfocus = () => preview(card);
    const fallback = el('span', '', 'card-fallback');
    fallback.append(el('strong', card.name), symbolText('small', card.mana || ''), el('span', card.type || 'Hidden card'), symbolText('span', card.rules || '', 'card-fallback-rules'));
    c.append(fallback, card.name === 'Face-down card' ? cardBack() : imageFor(card));
    const ribbon = el('span', '', 'card-ribbon');
    if (card.pt) { const shown = displayedStats(card), stats = el('strong', shown, shown !== card.pt ? 'stats-damaged' : card.statsModified ? 'stats-modified' : ''); stats.title = shown !== card.pt ? `${card.pt} power/toughness · ${card.damage} damage marked · ${shown.split('/')[1]} toughness remaining before lethal damage` : 'Current power and toughness, including effects and counters'; ribbon.append(stats); }
    if (card.damage) ribbon.append(el('span', `${card.damage} dmg`));
    if ((card.counters || []).length) ribbon.append(el('span', card.counters.map(x => `${x.count} ${x.name}`).join(' · ')));
    if (card.keywords?.length) ribbon.append(el('span', card.keywords.join(' · '), 'card-keywords'));
    if (ribbon.children.length) c.append(ribbon);
    if (card.selected) c.append(el('span', '✓', 'selection-badge'));
    if (blocks) { c.title = blocks; c.append(el('span', '↗ Block', 'combat-badge')); }
    else if (card.attacking) c.append(el('span', '↑ Attack', 'combat-badge'));
    wrap.append(c);
    const labels = [card.attacking ? 'Attacking' : '', card.blocking ? 'Blocking' : '', card.tapped ? 'Tapped' : '', card.summoningSick ? 'New' : '', card.token ? 'Token' : '', card.copy ? 'Copy' : ''].filter(Boolean);
    if (labels.length) wrap.append(el('small', labels.join(' · '), 'card-state'));
    return wrap;
  }
  function landNodes(cards, enabled, send) {
    const groups = new Map(), result = [];
    for (const card of cards) {
      if (!/\bBasic\b/.test(card.type || '')) {result.push(cardNode(card,enabled,send));continue;}
      // Keep different printings, tap states and live characteristics separate.
      const key = JSON.stringify([card.name,card.printing,card.type,!!card.tapped,card.rules,card.abilities,card.counters,card.keywords]);
      let group = groups.get(key);
      if (!group || group.children.length >= 5) {
        group = el('div','','land-pile'); group.dataset.tapped = String(!!card.tapped); groups.set(key,group); result.push(group);
      }
      const slot = cardNode(card,enabled,send); slot.style?.setProperty('--pile-index',String(group.children.length)); group.append(slot); group.dataset.pileSize = String(group.children.length);
    }
    for (const group of result.filter(node => node.className === 'land-pile')) {
      group.style?.setProperty('--pile-size',group.dataset.pileSize);
      group.setAttribute('role','group'); group.setAttribute('aria-label',`${group.dataset.pileSize} matching basic lands, ${group.dataset.tapped === 'true' ? 'tapped' : 'untapped'}`);
      if (Number(group.dataset.pileSize) > 1) {const count=el('span',group.dataset.pileSize,'land-pile-count');count.setAttribute('aria-hidden','true');group.append(count);}
    }
    return result;
  }
  function zoneNode(zone, player, enabled, send) {
    if (zone.zone !== 'Battlefield' && (zone.zone !== 'Hand' || player.name === 'CPU')) {
      const button = el('button', '', 'zone-shortcut'); button.type = 'button';
      button.dataset.zonePlayer = String(player.id); button.dataset.zoneName = zone.zone;
      button.setAttribute('aria-controls', 'zoneBrowser'); button.setAttribute('aria-expanded', 'false');
      button.setAttribute('aria-label', `${player.name === 'Human' ? 'Your' : 'Opponent’s'} ${zone.zone}, ${zone.count} card${zone.count === 1 ? '' : 's'}`);
      button.title = `${zone.zone} · ${zone.count} — click to browse`;
      button.append(icon(zone.zone, 'zone-icon'), el('span', zone.zone, 'zone-shortcut-label'), el('strong', String(zone.count), 'zone-count'));
      button.onclick = () => {
        if (openZone?.player === player.id && openZone?.zone === zone.zone) closeZoneBrowser();
        else { openZone = {player: player.id, zone: zone.zone}; renderZoneBrowser(); $('closeZoneBrowser').focus(); }
      };
      return button;
    }
    const wrap = el('section', '', `zone zone-${zone.zone.toLowerCase()}`);
    const heading = el('h3', '', 'zone-heading');
    heading.append(icon(zone.zone), el('span', `${zone.zone} · ${zone.count}`)); wrap.append(heading);
    const gallery = el('div', '', 'cards');
    const cards = zone.cards || [];
    if (zone.zone === 'Battlefield') {
      const nonlands = cards.filter(c => !/\bLand\b/.test(c.type || ''));
      const lands = cards.filter(c => /\bLand\b/.test(c.type || ''));
      const lanes = player.name === 'CPU' ? [['Lands', lands], ['Permanents', nonlands]] : [['Permanents', nonlands], ['Lands', lands]];
      for (const [label, list] of lanes) {
        const row = el('div', '', 'battlefield-row'); row.dataset.lane = label;
        if (!list.length) continue;
        row.setAttribute('aria-label', label);
        const rowCards = el('div', '', 'cards'); rowCards.append(...(label === 'Lands' ? landNodes(list,enabled,send) : list.map(c=>cardNode(c,enabled,send))));

        row.append(rowCards); gallery.append(row);
      }
      gallery.dataset.lanes = nonlands.length && lands.length ? 'both' : 'single';
      if (!cards.length) {
        const empty = el('div', '', 'battlefield-empty'); empty.append(el('span', 'Empty battlefield')); gallery.append(empty);
      }
    } else {
      cards.forEach(c => gallery.append(cardNode(c, enabled, send)));
      if (!cards.length && zone.count > 0) {
        const backs = el('div', '', 'card-backs'); backs.setAttribute('aria-hidden', 'true');
        for (let i = 0; i < Math.min(zone.zone === 'Hand' ? 7 : 3, zone.count); i++) backs.append(cardBack());
        gallery.append(backs, el('span', `${zone.count} hidden card${zone.count === 1 ? '' : 's'}`, 'hidden-count'));
      } else if (!cards.length) gallery.append(el('span', 'Empty', 'empty-zone'));
    }
    wrap.append(gallery); return wrap;
  }
  function render(s, send, {working = false, setPhaseStop = null} = {}) {
    currentView = s; currentSend = send; if (setPhaseStop) changeStop = setPhaseStop;
    cardNames = new Map((s.players || []).flatMap(p => (p.zones || []).flatMap(z => z.cards || [])).map(c => [c.id,c.name]));
    combatLinks = new Map();
    for (const link of s.combat || []) for (const id of [...(link.blockers || []), ...(link.plannedBlockers || [])]) { const targets = combatLinks.get(id) || []; targets.push(link.attacker); combatLinks.set(id, targets); } currentEnabled = !working && s.status === 'input';
    const selected = previewSelection;
    if (selected?.kind === 'card') {
      const card = (s.players || []).flatMap(p => (p.zones || []).flatMap(z => z.cards || [])).find(c => c.id === selected.id && c.name === selected.name);
      if (card) preview(card); else clearPreview();
    } else if (selected?.kind === 'stack') {
      const item = (s.stack || []).find(i => i.source === selected.name && i.text === selected.text);
      if (item) preview({...item.card, name: item.source, rules: item.text, abilities: '', previewOnly: true}, selected); else clearPreview();
    }
    if (!previewSelection) {
      const human = s.players?.find(player => player.name === 'Human');
      const visible = (human?.zones || []).flatMap(zone => zone.cards || []).filter(card => card.name && card.name !== 'Face-down card');
      const initial = visible.find(card => human.commanders?.some(commander => commander.name === card.name)) || visible[0];
      if (initial) preview(initial);
    }
    const seats = [...(s.players || [])].sort((a, b) => (a.name === 'CPU' ? 0 : 1) - (b.name === 'CPU' ? 0 : 1));
    const enabled = !working && s.status === 'input';
    $('players').replaceChildren();
    for (const player of seats) {
      const seat = el('section', '', 'seat ' + (player.name === 'Human' ? 'seat-human' : 'seat-cpu'));
      const life = el('button', '', 'player-life'); life.dataset.playerId = String(player.id); life.disabled = !enabled; life.setAttribute('aria-label', `${player.name}, ${player.life} life`); life.onclick = () => send('player', {id: player.id});
      life.append(el('span', player.name === 'Human' ? 'YOU' : 'CPU', 'seat-label'), el('strong', String(player.life)), el('span', 'life'));
      const header = el('header', '', 'seat-header'); header.append(life);
      const identity = el('div', '', 'seat-identity'); identity.append(el('h2', player.name === 'Human' ? 'Your battlefield' : 'Opponent’s battlefield'));
      const commanders = (player.commanders || []).map(c => c.name).join(' + '); if (commanders) identity.append(el('small', commanders));
      header.append(identity);
      seat.dataset.active = String(s.turnPlayer === player.name);
      if (s.turnPlayer === player.name) header.append(el('span', 'Active turn', 'seat-active'));
      const hiddenHand = (player.zones || []).find(z => z.zone === 'Hand');
      header.append(manaPool(player, s, enabled, send));
      seat.append(header);
      const table = el('div', '', 'seat-table'), reserves = el('div', '', 'reserves');
      table.append(reserves, phaseStops(player, s));
      if (player.name === 'CPU' && hiddenHand) reserves.append(zoneNode(hiddenHand, player, enabled, send));
      for (const zone of player.zones || []) {
        if (zone.zone === 'Battlefield') table.append(zoneNode(zone, player, enabled, send));
        else if (zone.zone !== 'Hand') reserves.append(zoneNode(zone, player, enabled, send));
      }
      seat.append(table);
      const commanderDamage = (player.commanders || []).filter(c => c.damageToHuman);
      if (commanderDamage.length) seat.append(el('small', commanderDamage.map(c => `${c.name}: ${c.damageToHuman} commander damage to you`).join(' / '), 'commander-stats'));
      $('players').append(seat);
    }
    const human = seats.find(player => player.name === 'Human');
    const hand = human?.zones?.find(zone => zone.zone === 'Hand');
    if (hand) $('players').append(zoneNode(hand, human, enabled, send));
    $('boardNotice').hidden = !working;
    $('boardNotice').textContent = working ? 'Resolving… Last confirmed board shown. Controls resume at your next decision.' : '';
    renderPhase(s, working);
    const heading = el('h2', s.stack?.length ? `The stack · ${s.stack.length}` : 'The stack');
    $('stack').replaceChildren(heading);
    if (!s.stack?.length) $('stack').append(el('p', 'Nothing waiting to resolve.', 'empty-zone'));
    else for (const [index,item] of s.stack.entries()) {
      const row = el('div', '', 'stack-item'); row.append(el('strong', item.source), symbolText('p', item.text || ''));
      row.dataset.stackIndex = String(index);
      if (item.source && item.source !== 'Hidden source') {
        row.tabIndex = 0; row.onmouseenter = row.onfocus = () => preview({...item.card, name: item.source, rules: item.text, abilities: '', previewOnly: true}, {kind: 'stack', name: item.source, text: item.text});
      }
      $('stack').append(row);
    }
    // The worker identifies zones using the actual offered cards, not prose.
    // A closed browser stays closed for this decision, including poll refreshes.
    if (!working && s.prompt?.id && zonePrompt !== s.prompt.id) {
      zonePrompt = s.prompt.id;
      const focus = (s.prompt.focusZones || []).find(f => (s.players || []).some(p => p.id === f.player && (p.zones || []).some(z => z.zone === f.zone && !['Hand', 'Battlefield'].includes(z.zone) && z.cards?.length)));
      if (focus) openZone = {player: focus.player, zone: focus.zone};
    }
    renderZoneBrowser(); updateStops(s.phaseStops); queueLayout();
  }
  function renderPhase(s, working) {
    $('gameStatus').hidden = false;
    $('turnOwner').textContent = s.turnPlayer === 'Human' ? 'Your turn' : s.turnPlayer === 'CPU' ? 'Opponent’s turn' : 'Preparing the table';
    $('turnNumber').textContent = s.turn ? `Turn ${s.turn}` : '';
    $('priorityBadge').textContent = working ? 'Resolving…' : s.priorityPlayer === 'Human' ? 'You have priority' : s.status === 'choice' ? 'Your choice' : '';
    const active = phaseIndex(s.phase);
    $('phases').replaceChildren(...['Beginning', 'Main 1', 'Combat', 'Main 2', 'Ending'].map((label, i) => {
      const item = el('li', '', i === active ? 'phase current' : i < active ? 'phase passed' : 'phase');
      item.append(el('span', String(i + 1), 'phase-dot'), el('span', label));
      if (i === active) { item.setAttribute('aria-current', 'step'); item.append(el('small', phaseName(s.phase))); }
      return item;
    }));
  }
  function waitingBoard(previous) {
    // Old engines do not identify scope-only reveals. Fail closed for those cards.
    return {...previous, status: 'working', prompt: null, stack: [], players: (previous.players || []).map(p => ({...p,
      zones: (p.zones || []).map(z => ({...z, cards: z.zone === 'Library' || (z.zone === 'Hand' && p.name !== 'Human') ? [] : (z.cards || []).filter(c => c.temporaryReveal === false || (c.name === 'Face-down card'))})),
    }))};
  }
  if (typeof ResizeObserver !== 'undefined') { const resize = new ResizeObserver(queueLayout); resize.observe($('game')); resize.observe($('players')); }
  $('game').addEventListener?.('scroll', queueLayout, true);
  $('closeZoneBrowser').onclick = () => closeZoneBrowser(true);
  $('cpuPage').onkeydown = event => { if (event.key === 'Escape') closeZoneBrowser(true); };
  globalThis.ManatombCPUTable = {symbolText, cardRules, render, renderPhase, phaseName, guidance, waitingBoard, reset, updateStops, relayout: queueLayout, connectionGeometry};
})();
