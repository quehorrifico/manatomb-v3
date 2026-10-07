'use strict';
// Presentation only: every action still uses the current Forge prompt and IDs.
(() => {
  const $ = id => document.getElementById(id);
  const el = (tag, text = '', className = '') => { const n = document.createElement(tag); n.textContent = text; n.className = className; return n; };
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
    };
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    for (const [key, value] of Object.entries({viewBox:'0 0 24 24', fill:'none', stroke:'currentColor', 'stroke-width':'1.6', 'stroke-linecap':'round', 'stroke-linejoin':'round', 'aria-hidden':'true', focusable:'false', class:`table-icon ${className}`})) svg.setAttribute(key, value);
    for (const d of paths[name] || paths.Library) { const path = document.createElementNS('http://www.w3.org/2000/svg', 'path'); path.setAttribute('d', d); svg.append(path); }
    return svg;
  }
  const imageCache = new Map(), imageQueue = [];
  let imageBusy = false, imageBackoff = 0, previewSelection = null, openZone = null, currentView = null, currentSend = null, currentEnabled = false;
  const phaseNames = {
    UNTAP: 'Untap', UPKEEP: 'Upkeep', DRAW: 'Draw', MAIN1: 'First main', MAIN2: 'Second main',
    COMBAT_BEGIN: 'Beginning of combat', COMBAT_DECLARE_ATTACKERS: 'Declare attackers',
    COMBAT_DECLARE_BLOCKERS: 'Declare blockers', COMBAT_FIRST_STRIKE_DAMAGE: 'First-strike damage',
    COMBAT_DAMAGE: 'Combat damage', COMBAT_END: 'End of combat', END_OF_TURN: 'End step', CLEANUP: 'Cleanup',
  };
  function phaseName(phase) { return phaseNames[phase] || String(phase || 'Preparing').replaceAll('_', ' ').toLowerCase(); }
  const stopPhases = [['UNTAP','UN'],['UPKEEP','UP'],['DRAW','DR'],['MAIN1','M1'],['COMBAT_BEGIN','BC'],['COMBAT_DECLARE_ATTACKERS','AT'],['COMBAT_DECLARE_BLOCKERS','BL'],['COMBAT_FIRST_STRIKE_DAMAGE','FS'],['COMBAT_DAMAGE','DM'],['COMBAT_END','EC'],['MAIN2','M2'],['END_OF_TURN','EN'],['CLEANUP','CL']];
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
      chip.dataset.empty = String(amount === 0); chip.append(el('span', symbol), el('strong', String(amount)));
      if (spendable) { chip.type = 'button'; chip.onclick = () => send('mana', {id:mana.id}); }
      pool.append(chip);
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
      const slots = [...group.children].filter(n => n.classList.contains('card-slot') || n.classList.contains('zone-hidden'));
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
        const ratio = slots.some(s => s.classList.contains('is-tapped')) ? 1.4 : 1;
        let best = 1, columns = 1;
        for (let cols = 1; cols <= slots.length; cols++) {
          const rows = Math.ceil(slots.length / cols);
          const candidate = Math.min(150, (width - (cols-1)*5) / cols / ratio, ((height-(rows-1)*5)/rows-12)/1.4);
          if (candidate > best) { best = candidate; columns = cols; }
        }
        group.style.setProperty('--fit-width', Math.max(1, best) + 'px');
        group.style.gridTemplateColumns = `repeat(${columns}, ${best * ratio}px)`;
      }
    }
  }
  function drawCombat() {
    const svg = $('combatArrows'); if (!svg?.getBoundingClientRect) return;
    svg.replaceChildren(); if (!$('zoneBrowser').hidden) return;
    const box = svg.getBoundingClientRect(); if (!box.width || !box.height) return;
    svg.setAttribute('viewBox', `0 0 ${box.width} ${box.height}`);
    const nodes = new Map([...document.querySelectorAll('#players [data-card-id]')].map(n => [Number(n.dataset.cardId), n]));
    const playerNodes = new Map([...document.querySelectorAll('#players [data-player-id]')].map(n => [Number(n.dataset.playerId), n]));
    const connect = (from, to, kind) => {
      if (!from || !to) return;
      const a = from.getBoundingClientRect(), b = to.getBoundingClientRect();
      const down = a.top < b.top;
      const x1 = a.left + a.width/2-box.left, y1 = (down ? a.bottom : a.top)-box.top;
      const x2 = b.left + b.width/2-box.left, y2 = (down ? b.top : b.bottom)-box.top;
      const path = document.createElementNS('http://www.w3.org/2000/svg','path');
      path.setAttribute('d', `M${x1},${y1} C${x1},${(y1+y2)/2} ${x2},${(y1+y2)/2} ${x2},${y2}`);
      path.setAttribute('class',`combat-link ${kind}`); svg.append(path);
      const arrow = document.createElementNS('http://www.w3.org/2000/svg','path');
      arrow.setAttribute('d',`M${x2-4},${y2+(down?-7:7)} L${x2},${y2} L${x2+4},${y2+(down?-7:7)}`); arrow.setAttribute('class',`combat-link ${kind}`); svg.append(arrow);
    };
    for (const link of currentView?.combat || []) {
      const attacker = nodes.get(link.attacker);
      connect(attacker, link.defender?.type === 'player' ? playerNodes.get(link.defender.id) : nodes.get(link.defender?.id), 'attack');
      for (const id of link.blockers || []) connect(nodes.get(id),attacker,'block');
      for (const id of link.plannedBlockers || []) if (!(link.blockers || []).includes(id)) connect(nodes.get(id),attacker,'planned');
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
    const key = `${card.name}:${!!card.backFace}`;
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
        const response = await fetch('/cards/resolve?q=' + encodeURIComponent(card.name) + '&exact=1');
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
  function preview(card, selection = {kind: 'card', id: card.id, name: card.name}) {
    previewSelection = selection;
    const detail = $('cardDetail');
    detail.replaceChildren(el('h3', card.name), el('p', [card.mana, card.pt].filter(Boolean).join(' · '), 'preview-mana'),
      el('p', facts(card).join(' · '), 'preview-facts'),
      el('pre', [card.rules, card.abilities !== card.rules ? card.abilities : ''].filter(Boolean).join('\n\n'), 'preview-rules'));
    const art = el('div', '', 'preview-card');
    art.append(el('strong', card.name), imageFor(card));
    $('cardPicture').replaceChildren(art);
  }
  function clearPreview() {
    previewSelection = null;
    $('cardDetail').replaceChildren(el('p', 'Hover over a card to see its details.', 'preview-hint'));
    const placeholder = el('span', 'M', 'preview-placeholder');
    placeholder.append(el('span', 'Explore the table'), el('small', 'Hover or focus a card in any visible zone.'));
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
      const back = el('div', '', 'zone-hidden'); back.append(el('span', 'M', 'card-back'), el('strong', String(hidden)), el('small', 'Hidden cards')); gallery.append(back);
    }
    for (const button of document.querySelectorAll('[data-zone-player]')) button.setAttribute('aria-expanded', String(button.dataset.zonePlayer === String(openZone.player) && button.dataset.zoneName === openZone.zone));
    queueLayout();
  }
  function reset() { $('combatArrows').replaceChildren(); clearPreview(); closeZoneBrowser(); currentView = null; currentSend = null; }
  function inspect(card) {
    const dialog = $('cardInspector'), content = $('inspectorContent');
    const title = el('h2', card.name); title.id = 'inspectorTitle';
    const art = el('div', '', 'inspector-art'); art.append(imageFor(card), el('strong', card.name));
    const text = el('div', '', 'inspector-text');
    text.append(title, el('p', [card.mana, card.pt].filter(Boolean).join(' · ')), el('p', facts(card).join(' · ')),
      el('pre', [card.rules, card.abilities !== card.rules ? card.abilities : ''].filter(Boolean).join('\n\n')),
      el('small', 'Current characteristics above come from the game. Printed art may differ for tokens and copies.'));
    content.replaceChildren(art, text);
    if (!dialog.open) dialog.showModal();
  }
  function closeInspector() { if ($('cardInspector').open) $('cardInspector').close(); }
  function cardNode(card, enabled, send) {
    const wrap = el('div', '', 'card-slot' + (card.tapped ? ' is-tapped' : ''));
    const c = el('button', '', 'card' + (card.tapped ? ' tapped' : '') + (card.selected ? ' selected' : '') + (card.attacking ? ' attacking' : '') + (card.blocking ? ' blocking' : ''));
    enabled = enabled && card.selectable !== false;
    if (card.id !== undefined) c.dataset.cardId = String(card.id);
    c.type = 'button'; c.setAttribute('aria-disabled', String(!enabled || card.id === undefined));
    const blocks = combatLinks.has(card.id) ? 'Blocks ' + [...new Set(combatLinks.get(card.id))].map(id => cardNames.get(id) || 'attacking creature').join(', ') : '';
    c.setAttribute('aria-label', [card.name, card.reference, card.tapped ? 'tapped' : '', card.pt, card.selected ? 'selected' : '', card.attacking ? 'attacking' : '', blocks || (card.blocking ? 'blocking' : '')].filter(Boolean).join(', '));
    c.onclick = () => { if (enabled && card.id !== undefined) send('card', {id: card.id}); };
    wrap.onmouseenter = () => preview(card); c.onfocus = () => preview(card);
    const fallback = el('span', '', 'card-fallback');
    fallback.append(el('strong', card.name), el('small', card.mana || ''), el('span', card.type || 'Hidden card'), el('span', card.rules || '', 'card-fallback-rules'));
    c.append(fallback, imageFor(card));
    const ribbon = el('span', '', 'card-ribbon');
    if (card.pt) ribbon.append(el('strong', card.pt));
    if (card.damage) ribbon.append(el('span', `${card.damage} dmg`));
    if ((card.counters || []).length) ribbon.append(el('span', card.counters.map(x => `${x.count} ${x.name}`).join(' · ')));
    if (ribbon.children.length) c.append(ribbon);
    if (card.selected) c.append(el('span', '✓', 'selection-badge'));
    if (blocks) { c.title = blocks; c.append(el('span', '↗ Block', 'combat-badge')); }
    else if (card.attacking) c.append(el('span', '↑ Attack', 'combat-badge'));
    wrap.append(c);
    const labels = [card.attacking ? 'Attacking' : '', card.blocking ? 'Blocking' : '', card.tapped ? 'Tapped' : '', card.summoningSick ? 'New' : '', card.token ? 'Token' : '', card.copy ? 'Copy' : ''].filter(Boolean);
    if (labels.length) wrap.append(el('small', labels.join(' · '), 'card-state'));
    if (card.id !== undefined && card.name !== 'Face-down card') {
      const info = el('button', '↗', 'inspect-card'); info.type = 'button'; info.setAttribute('aria-label', `Inspect ${card.name}`); info.title = `Inspect ${card.name}`;
      info.onfocus = () => preview(card); info.onclick = () => { preview(card); inspect(card); }; wrap.append(info);
    }
    return wrap;
  }
  function zoneNode(zone, player, enabled, send) {
    if (zone.zone !== 'Battlefield' && zone.zone !== 'Hand') {
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
      for (const [label, list] of [['Permanents', nonlands], ['Lands', lands]]) {
        const row = el('div', '', 'battlefield-row'); row.dataset.lane = label;
        const labelNode = el('span', '', 'row-label'); labelNode.append(icon(label), el('span', label)); row.append(labelNode);
        const rowCards = el('div', '', 'cards'); list.forEach(c => rowCards.append(cardNode(c, enabled, send)));
        if (!list.length) {
          const empty = el('div', '', 'battlefield-empty'); empty.append(icon(label), el('span', `No ${label.toLowerCase()}`)); rowCards.append(empty);
        }
        row.append(rowCards); gallery.append(row);
      }
    } else {
      cards.forEach(c => gallery.append(cardNode(c, enabled, send)));
      if (!cards.length && zone.count > 0) {
        const backs = el('div', '', 'card-backs'); backs.setAttribute('aria-hidden', 'true');
        for (let i = 0; i < Math.min(zone.zone === 'Hand' ? 7 : 3, zone.count); i++) backs.append(el('span', 'M', 'card-back'));
        gallery.append(backs, el('span', `${zone.count} hidden card${zone.count === 1 ? '' : 's'}`, 'hidden-count'));
      } else if (!cards.length) gallery.append(el('span', 'Empty', 'empty-zone'));
    }
    wrap.append(gallery); return wrap;
  }
  function render(s, send, {working = false, setPhaseStop = null} = {}) {
    closeInspector();
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
      if (item) preview({name: item.source, rules: item.text, previewOnly: true}, selected); else clearPreview();
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
      if (player.name === 'CPU' && !hiddenHand?.cards?.length) header.append(el('span', `${hiddenHand?.count || 0} cards in hand`, 'hand-count'));
      header.append(manaPool(player, s, enabled, send));
      seat.append(header);
      const table = el('div', '', 'seat-table'), reserves = el('div', '', 'reserves');
      table.append(phaseStops(player, s));
      for (const zone of player.zones || []) {
        if (zone.zone === 'Battlefield') table.append(zoneNode(zone, player, enabled, send));
        else if (zone.zone !== 'Hand') reserves.append(zoneNode(zone, player, enabled, send));
      }
      table.append(reserves); seat.append(table);
      if (hiddenHand && (player.name === 'Human' || hiddenHand.cards?.length)) seat.append(zoneNode(hiddenHand, player, enabled, send));
      if ((player.commanders || []).some(c => c.casts || c.damageToHuman)) seat.append(el('small', player.commanders.map(c => `${c.name}: cast ${c.casts}× · ${c.damageToHuman} commander damage to you`).join(' / '), 'commander-stats'));
      $('players').append(seat);
    }
    $('boardNotice').hidden = !working;
    $('boardNotice').textContent = working ? 'Resolving… Last confirmed board shown. Controls resume at your next decision.' : '';
    renderPhase(s, working);
    const heading = el('h2', 'The stack'); heading.append(el('span', String(s.stack?.length || 0), 'stack-count'));
    $('stack').replaceChildren(heading);
    if (!s.stack?.length) $('stack').append(el('p', 'Nothing waiting to resolve.', 'empty-zone'));
    else for (const item of s.stack) {
      const row = el('div', '', 'stack-item'); row.append(el('strong', item.source), el('p', item.text || ''));
      if (item.source && item.source !== 'Hidden source') {
        row.tabIndex = 0; row.onmouseenter = row.onfocus = () => preview({name: item.source, rules: item.text, previewOnly: true}, {kind: 'stack', name: item.source, text: item.text});
      }
      $('stack').append(row);
    }
    renderZoneBrowser(); updateStops(s.phaseStops); queueLayout();
  }
  function renderPhase(s, working) {
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
  $('closeInspector').onclick = closeInspector;
  $('closeZoneBrowser').onclick = () => closeZoneBrowser(true);
  $('cpuPage').onkeydown = event => { if (event.key === 'Escape' && !$('cardInspector').open) closeZoneBrowser(true); };
  globalThis.ManatombCPUTable = {render, renderPhase, phaseName, guidance, waitingBoard, closeInspector, reset, updateStops};
})();
