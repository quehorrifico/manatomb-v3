'use strict';
(() => {
  const $ = id => document.getElementById(id);
  const desktopUnavailable = () => typeof matchMedia === 'function' && matchMedia('(max-width:850px), (pointer:coarse) and (max-width:1024px)').matches;
  // A durable desktop notice is shown by CSS. Do not create or poll a game on
  // an unsupported screen; resizing back restores the existing desktop view.
  if (desktopUnavailable()) {
    matchMedia('(max-width:850px), (pointer:coarse) and (max-width:1024px)').addEventListener?.('change', () => {
      if (!desktopUnavailable()) location.reload();
    });
    return;
  }
  const element = (tag, text) => globalThis.ManatombCPUTable.symbolText(tag, text);
  let session = null, state = {}, revision = '', pending = false, timer, owner = null, returnFocus = false;
  let activitySession = '', activitySignature = '';
  let savedStops = {}, restoredStopsSession = '';
  const stopPhases = new Set(['UPKEEP','DRAW','MAIN1','COMBAT_BEGIN','COMBAT_DECLARE_ATTACKERS','COMBAT_DECLARE_BLOCKERS','COMBAT_FIRST_STRIKE_DAMAGE','COMBAT_DAMAGE','COMBAT_END','MAIN2','END_OF_TURN','CLEANUP']);
  const table = globalThis.ManatombCPUTable;
  let lastBoard = null, refreshInFlight = null, refreshAgain = false, refreshGeneration = 0;
  let seen = null, awaitingRevision = '', fastUntil = 0;
  const pendingStops = new Map();
  const deckNames = {human:'Your Commander deck', cpu:'Custom CPU deck'};
  const deckPrintings = {human: new Map(), cpu: new Map()};
  let editorDeck = null, editorDeckOption = null, editorHandoff = false, decklistGeneration = 0;
  function loadEditorDeck(deck) {
    editorDeck = deck;
    if (!editorDeckOption) {
      editorDeckOption = element('option', ''); editorDeckOption.value = 'editor';
      $('humanSaved').append(editorDeckOption);
    }
    editorDeckOption.textContent = deck.name || 'Commander deck';
    $('humanSaved').value = 'editor'; fillDeck('human', deck);
    $('humanDecklistSummary').textContent = 'View or edit decklist';
    $('humanDecklist').open = false;
  }
  function selectPanel(name, focus = false) {
    for (const tab of ['stack', 'log', 'options']) {
      $(tab + 'Tab').setAttribute('aria-selected', String(tab === name));
      $(tab + 'Tab').tabIndex = tab === name ? 0 : -1;
      $(tab + 'Panel').hidden = tab !== name;
    }
    if (focus) $(name + 'Tab').focus();
    table.relayout?.();
  }
  for (const [index, name] of ['stack', 'log', 'options'].entries()) {
    $(name + 'Tab').onclick = () => selectPanel(name);
    $(name + 'Tab').onkeydown = event => {
      const tabs = ['stack', 'log', 'options'];
      const next = event.key === 'ArrowRight' ? (index + 1) % 3 : event.key === 'ArrowLeft' ? (index + 2) % 3 : event.key === 'Home' ? 0 : event.key === 'End' ? 2 : -1;
      if (next >= 0) { event.preventDefault(); selectPanel(tabs[next], true); }
    };
  }
  selectPanel('stack');
  function reconcileStops(next) {
    const stops = next.state?.phaseStops;
    if (!stops) return;
    const visible = {...stops};
    for (const [key, change] of pendingStops) {
      if (change.session !== next.id || terminal(next.status) || stops[change.seat]?.includes(change.phase) === change.enabled || Date.now() >= change.until) { pendingStops.delete(key); continue; }
      visible[change.seat] = change.enabled ? [...new Set([...(visible[change.seat] || []), change.phase])] : (visible[change.seat] || []).filter(phase => phase !== change.phase);
    }
    next.state = {...next.state, phaseStops: visible};
  }
  function schedule(next) {
    clearTimeout(timer);
    const delay = document.hidden ? 5000 : awaitingRevision && Date.now() < fastUntil ? 150 : pendingStops.size ? 300 : next.status === 'working' ? 700 : next.status === 'starting' ? 1000 : 2000;
    timer = setTimeout(refresh, delay);
  }
  function clearBoard() {
    pendingStops.clear();
    lastBoard = null; table.reset(); $('game').hidden = true; $('gameStatus').hidden = true;
    for (const id of ['concede', 'confirmConcede', 'requestConcede', 'confirmConcedeRequest']) $(id).hidden = true;
    selectPanel('stack');
    $('players').replaceChildren(); $('stack').replaceChildren(); $('phases').replaceChildren();
    $('priorityBadge').textContent = ''; $('turnOwner').textContent = ''; $('turnNumber').textContent = '';
    $('prompt').hidden = true; $('prompt').replaceChildren(); delete $('prompt').dataset.prompt;
    $('cpuPage').dataset.playing = 'false';
  }
  function observeActivity(next) {
    if (activitySession !== next.id) {activitySession = next.id; activitySignature = '';}
    // The adapter emits typed, visibility-checked events. Do not infer actions
    // from snapshots or import arbitrary engine log/diagnostic strings.
    const entries = (next.state?.log || []).filter(e => Number.isSafeInteger(e.id) && e.id > 0 && Number.isInteger(e.turn) && e.turn >= 0 && ['Human','CPU','None'].includes(e.player) && typeof e.text === 'string' && e.text.length <= 700).slice(-200);
    const signature = JSON.stringify(entries);
    if (signature === activitySignature) return;
    activitySignature = signature;
    const rows = []; let group = '';
    for (const entry of entries) {
      const key = `${entry.turn}:${entry.player}`;
      if (group !== key) {
        group = key; const heading = element('li', entry.turn ? `Turn ${entry.turn} · ${entry.player === 'Human' ? 'You' : entry.player === 'CPU' ? 'Opponent' : 'Table'}` : 'Opening hands'); heading.className = 'log-turn'; rows.push(heading);
      }
      rows.push(element('li', entry.text));
    }
    $('activity').replaceChildren(...(rows.length ? rows : [element('li', 'Game actions will appear here as play progresses.')]));
  }
  function readStops() {
    if (owner == null) return;
    try {
      const raw = JSON.parse(localStorage.getItem(`manatomb.cpu.phaseStops.${owner}`) || '{}');
      savedStops = {};
      for (const seat of ['Human','CPU']) for (const [phase, enabled] of Object.entries(raw?.[seat] || {})) {
        if (stopPhases.has(phase) && typeof enabled === 'boolean') (savedStops[seat] ||= {})[phase] = enabled;
      }
    } catch (_) { savedStops = {}; }
  }
  async function restoreStops(next) {
    if (!next.state?.phaseStops || !next.state.session || terminal(next.status)) return;
    await configuration;
    const key = `${next.id}:${next.state.session}`;
    if (key === restoredStopsSession) return;
    for (const seat of ['Human','CPU']) for (const [phase, enabled] of Object.entries(savedStops[seat] || {})) {
      if (next.state.phaseStops[seat]?.includes(phase) === enabled) continue;
      await api(`/sessions/${next.id}/actions`, 'POST', {request: crypto.randomUUID(), session: next.state.session, revision: 0, prompt: '', action: 'setPhaseStop', seat, phase, enabled});
      pendingStops.set(`${seat}:${phase}`, {session:next.id, seat, phase, enabled, until:Date.now()+5000});
    }
    restoredStopsSession = key;
  }
  const terminal = status => ['finished', 'failed', 'cancelled', 'expired'].includes(status);
  async function api(path, method = 'GET', body) {
    const response = await fetch('/api/cpu' + path, {method, headers: {'Content-Type': 'application/json'}, body: body === undefined ? undefined : JSON.stringify(body), cache: 'no-store'});
    const json = response.headers.get('Content-Type')?.includes('application/json');
    const data = json ? await response.json() : {};
    if (!response.ok || !json) {
      const error = new Error(data.error || (response.status === 429 ? 'Too many requests. Waiting before reconnecting…' : 'The server returned an unexpected response. Reconnect before retrying.'));
      error.status = response.status;
      error.retryAfter = Math.max(1000, (Number(response.headers.get('Retry-After')) || 5) * 1000);
      throw error;
    }
    return data;
  }
  function fillDeck(seat, deck) { deckNames[seat] = deck.name || (seat === 'human' ? 'Your Commander deck' : 'Custom CPU deck'); deckPrintings[seat] = new Map([...deck.commanders, ...deck.main].filter(c => c.printing).map(c => [c.name, c.printing])); $(seat + 'Commanders').value = deck.commanders.map(c => c.name).join('\n'); $(seat + 'Main').value = deck.main.map(c => `${c.quantity} ${c.name}`).join('\n'); }
  function deck(seat) {
    const commanders = $(seat + 'Commanders').value.split('\n').map(s => s.trim()).filter(Boolean).map(name => ({name, quantity: 1}));
    const main = $(seat + 'Main').value.split('\n').map(s => s.trim()).filter(Boolean).map(line => {
      const match = /^(\d+)\s+(.+)$/.exec(line); if (!match) throw new Error('Use quantity and card name on every main-deck line: ' + line);
      return {name: match[2], quantity: Number(match[1])};
    });
    for (const card of [...commanders, ...main]) { const printing = deckPrintings[seat].get(card.name); if (printing) card.printing = printing; }
    return {name: deckNames[seat], commanders, main};
  }
  function refresh() {
    clearTimeout(timer);
    if (refreshInFlight) { refreshAgain = true; return refreshInFlight; }
    const generation = refreshGeneration;
    const work = (async () => {
      try {
        const next = await api('/sessions' + (session ? '/' + session.id : ''));
        if (generation !== refreshGeneration) { refreshAgain = true; return; }
        const worker = next.state?.session;
        if (seen && next.id === seen.id && worker === seen.worker && Number(next.state?.revision) < seen.revision) { schedule(next); return; }
        if (Number.isFinite(next.state?.revision)) seen = {id: next.id, worker, revision: next.state.revision};
        await restoreStops(next);
        if (generation !== refreshGeneration) { refreshAgain = true; return; }
        reconcileStops(next);
        const wasPlaying = $('cpuPage').dataset.playing === 'true';
        $('signinAgain').hidden = true; session = next; observeActivity(next);
        $('gameResult').hidden = !terminal(next.status);
        if (terminal(next.status)) {
          clearBoard(); state = {}; revision = ''; awaitingRevision = ''; fastUntil = 0;
          $('setup').hidden = false;
          for (const id of ['cancel', 'confirmCancel', 'requestConcede', 'confirmConcedeRequest']) $(id).hidden = true;
          $('expiry').textContent = ''; $('concedePending').textContent = ''; $('error').textContent = ''; $('status').textContent = '';
          const result = next.state?.result;
          $('resultTitle').textContent = next.status === 'finished' ? (result === 'Human' ? 'You win' : result === 'CPU' ? 'CPU wins' : result === 'Draw' ? 'Game drawn' : 'Game complete') : next.status === 'expired' ? 'Game expired' : next.status === 'cancelled' ? 'Game ended' : 'Game interrupted';
          $('resultDetail').textContent = next.error || 'Your decklists are ready below. Start a fresh game whenever you like.';
          if (wasPlaying) $('gameResult').scrollIntoView?.({block: 'start'});
          return;
        }
        $('cpuPage').dataset.playing = 'true';
        $('game').hidden = false;
        $('concede').hidden = !next.state?.canConcede;
        if (!next.state?.canConcede || `${next.id}:${next.state?.revision}` !== revision) $('confirmConcede').hidden = true;
        $('requestConcede').hidden = !!(!next.state?.canRequestConcede || next.state?.canConcede || next.state?.concedePending);
        $('concedePending').textContent = next.state?.concedePending ? 'Concession queued for the next priority. Required choices still need your reply.' : '';
        if (next.state?.concedePending) $('confirmConcedeRequest').hidden = true;
        $('setup').hidden = true; $('cancel').hidden = false;
        $('status').textContent = next.status === 'starting' ? 'Shuffling up — loading the engine and checking your decks…' : next.status === 'working' ? 'Resolving actions…' : '';
        const nearLimit = new Date(next.expiresAt).getTime() - Date.now() < 60000;
        $('expiry').textContent = `${nearLimit ? 'Approaching session limit. ' : ''}Session ends by ${new Date(next.expiresAt).toLocaleTimeString()}. Reconnect within ${next.reconnectSeconds / 60} minutes after leaving.`;
        if (next.error) $('error').textContent = next.error;
        if (next.state?.prompt) {
          const key = `${next.id}:${next.state.revision}`;
          if (awaitingRevision && key !== awaitingRevision) { awaitingRevision = ''; fastUntil = 0; }
          if (key !== revision) { revision = key; render(next.state); }
          if (awaitingRevision) $('status').textContent = 'Sending your choice…';
        } else {
          $('prompt').hidden = false; $('prompt').replaceChildren(element('h2', 'Your response'), element('p', next.status === 'starting' ? 'Preparing your game…' : 'Resolving actions… Controls return at your next decision.')); delete $('prompt').dataset.prompt;
          $('confirmCancel').hidden = true;
          state = next.state || {};
          if (lastBoard) table.render(table.waitingBoard(lastBoard), send, {working: true});
          else {
            $('players').replaceChildren(element('p', 'Preparing the battlefield…'));
            table.renderPhase(next.state || {}, true);
          }
        }
        table.updateStops(next.state?.phaseStops);
        schedule(next);
      } catch (error) {
        if (generation !== refreshGeneration) { refreshAgain = true; return; }
        if (error.status === 401) {
          session = null; state = {}; revision = ''; seen = null; awaitingRevision = '';
          $('gameResult').hidden = true; $('setup').hidden = true; $('status').textContent = 'Sign in again to resume your game.'; $('error').textContent = ''; $('signinAgain').hidden = false;
          $('prompt').hidden = true; $('prompt').replaceChildren(); delete $('prompt').dataset.prompt; clearBoard(); $('expiry').textContent = ''; $('concedePending').textContent = '';
          for (const id of ['cancel', 'confirmCancel', 'requestConcede', 'confirmConcedeRequest']) $(id).hidden = true;
        } else if (error.status === 404) {
          session = null; state = {}; revision = ''; seen = null; awaitingRevision = '';
          $('gameResult').hidden = true; $('setup').hidden = false; $('status').textContent = ''; $('error').textContent = ''; $('prompt').hidden = true; $('prompt').replaceChildren();
          clearBoard(); $('cancel').hidden = true; $('confirmCancel').hidden = true;
        } else { $('error').textContent = error.message; timer = setTimeout(refresh, error.status === 429 ? error.retryAfter : 5000); }
      }
    })();
    refreshInFlight = work.finally(() => {
      refreshInFlight = null;
      if (refreshAgain) { refreshAgain = false; return refresh(); }
    });
    return refreshInFlight;
  }
  async function send(action, extra = {}) {
    if (pending || awaitingRevision || !session || !state.prompt || terminal(session.status)) return;
    returnFocus=!!($('prompt').contains?.(document.activeElement)||$('players').contains?.(document.activeElement)||$('zoneBrowser').contains?.(document.activeElement));
    pending = true; awaitingRevision = `${session.id}:${state.revision}`; fastUntil = Date.now() + 2500; refreshGeneration++; clearTimeout(timer); $('error').textContent = ''; $('prompt').setAttribute('aria-busy','true');
    document.querySelectorAll('#prompt button, #players .card, #zoneBrowser .card, #players .player-life').forEach(button => { button.disabled=true; });
    const body = {request: crypto.randomUUID(), session: state.session, revision: state.revision, prompt: state.prompt.id, action, ...extra};
    let rejected = '';
    try { await api(`/sessions/${session.id}/actions`, 'POST', body); }
    catch (error) { rejected = error.message; revision=''; awaitingRevision=''; fastUntil=0; }
    finally { pending = false; $('prompt').setAttribute('aria-busy','false'); await refresh(); if (rejected) $('error').textContent = rejected; }
  }
  async function setPhaseStop(seat, phase, enabled) {
    if (!session || terminal(session.status)) return;
    const current = session;
    try {
      await api(`/sessions/${current.id}/actions`, 'POST', {request: crypto.randomUUID(), session: current.state.session, revision: 0, prompt: '', action: 'setPhaseStop', seat, phase, enabled});
      if (owner != null && stopPhases.has(phase)) {
        (savedStops[seat] ||= {})[phase] = enabled;
        try { localStorage.setItem(`manatomb.cpu.phaseStops.${owner}`, JSON.stringify(savedStops)); } catch (_) {}
      }
      // Acknowledged preferences can precede the supervisor's next cached view.
      if (session?.id === current.id) pendingStops.set(`${seat}:${phase}`, {session:current.id, seat, phase, enabled, until:Date.now()+5000});
      await refresh();
    } catch (error) { $('error').textContent = error.message; }
  }
  function button(text, action, extra, disabled = false) { const b = element('button', text); b.disabled = disabled; b.onclick = () => send(action, extra); return b; }
  function render(s) {
    const moveFocus=returnFocus||$('prompt').contains?.(document.activeElement)||$('players').contains?.(document.activeElement)||$('zoneBrowser').contains?.(document.activeElement);
    state = s; const p = s.prompt; $('prompt').dataset.prompt = p.id; $('prompt').hidden = false;
    const text = element('p', p.kind === 'orderSimultaneousAbilities' ? 'Choose which ability resolves next. Move abilities up or down, then confirm. Later responses may resolve before these.' : p.text); text.className = 'decision-text';
    $('prompt').replaceChildren(element('h2', 'Your response'), text);
    $('error').textContent = s.actionError || '';
    if (s.status === 'choice' && p.min === 0 && p.max === 0 && !p.valueType) {
      // Forge's read-only reveals are acknowledgements, never selectable choices.
      $('prompt').replaceChildren(element('h2', 'Your response'), text);
      const list = element('ul', ''); list.className = 'reveal-list';
      for (const option of p.options || []) list.append(element('li', option.label));
      const next = button('Continue', 'reply', {selected: []}); next.className = 'primary-action';
      $('prompt').append(list, next);
    } else if (s.status === 'choice' && ['integer','text','amounts'].includes(p.valueType)) {
      const inputs = [];
      if (p.valueType === 'amounts') {
        $('prompt').append(element('p', `Total to assign: ${p.total}`));
        p.options.forEach((option, i) => {const label=element('label', option.label + ' ');const input=element('input','');input.type='number';input.min=p.minimumEach;input.max=p.limits[i];input.step='1';input.setAttribute('aria-label',option.label);label.append(input);inputs.push(input);$('prompt').append(label);});
      } else {
        const input=element('input','');input.type=p.valueType === 'integer' || p.numeric ? 'number' : 'text';input.setAttribute('aria-label','Your answer');input.maxLength=200;
        if(p.valueType==='integer'){input.min=p.lower;input.max=p.upper;input.step='1';$('prompt').append(element('p',`Allowed range: ${p.lower}–${p.upper}`));}
        inputs.push(input);$('prompt').append(input);
      }
      const submit=element('button','Submit answer');submit.onclick=()=>{if(inputs.some(input=>input.value===''||!input.checkValidity())){$('error').textContent='Enter an answer within the offered limits.';return;}send('reply',p.valueType==='amounts'?{amounts:inputs.map(i=>Number(i.value))}:p.valueType==='integer'?{value:Number(inputs[0].value)}:{text:inputs[0].value});};$('prompt').append(submit);
    } else if (s.status === 'choice' && p.ordering) {
      const ordered = [...p.options], list = element('ol', ''); list.className = 'ordering-list';
      // Repeated damage triggers share the same ability prose. Keep that prose
      // available once, and put each event's distinguishing context in the list.
      const contexts = p.options.map(option => { const at = option.label.lastIndexOf(' · Damage from '); return at < 0 ? null : {prefix:option.label.slice(0,at), label:option.label.slice(at+3)}; });
      const sharedAbility = contexts.length > 1 && contexts.every(context => context && context.prefix === contexts[0]?.prefix) ? contexts[0].prefix : '';
      if (sharedAbility) {
        const details = element('details',''); details.className = 'ordering-ability';
        details.append(element('summary',sharedAbility.split(' — ')[0] + ' · ability text'),element('p',sharedAbility));$('prompt').append(details);
      }
      const draw = () => {
        list.replaceChildren(...ordered.map((option, i) => {
          const row = element('li', ''); row.append(element('span', `${i + 1}. ${sharedAbility ? option.label.slice(sharedAbility.length+3) : option.label}`));
          for (const [delta, label] of [[-1, 'Move up'], [1, 'Move down']]) {
            const move = element('button', delta === -1 ? '↑' : '↓'); move.setAttribute('aria-label', `${label}: ${option.label}`);
            move.disabled = i + delta < 0 || i + delta >= ordered.length;
            move.onclick = () => { [ordered[i], ordered[i + delta]] = [ordered[i + delta], ordered[i]]; draw(); }; row.append(move);
          }
          return row;
        }));
      };
      draw(); const submit = element('button', 'Use this order'); submit.className = 'primary-action';
      submit.onclick = () => send('reply', {selected: ordered.map(o => o.id)}); $('prompt').append(list, submit);
    } else if (s.status === 'choice' && p.max === 1 && p.options.length <= 100) {
      const choices = element('div', ''); choices.className = 'choice-actions';
      for (const o of p.options) choices.append(button(o.label, 'reply', {selected: [o.id]}));
      $('prompt').append(choices);
      if (p.min === 0) $('prompt').append(button('Choose none', 'reply', {selected: []}));
    } else if (s.status === 'choice') {
      const choices=element('div','');
      const showOptions=options=>{choices.replaceChildren();for(const o of options){
        const label=element('label','');const input=document.createElement('input');input.type=p.max===1?'radio':'checkbox';input.name='choice';input.value=o.id;input.disabled=p.max===0;
        label.append(input,document.createTextNode(' '+o.label));choices.append(label);
      }};
      if(p.max===1 && p.options.length>100){
        // Catalog choices can contain tens of thousands of authorized names.
        // Filter presentation only; return the exact offered ID, never free-form rules data.
        const filter=element('input','');filter.type='search';filter.setAttribute('aria-label','Filter offered choices');filter.placeholder='Type a card name or choice';
        const count=element('p','');count.setAttribute('role','status');
        const update=()=>{const term=filter.value.trim().toLocaleLowerCase();const matches=p.options.filter(o=>o.label.toLocaleLowerCase().includes(term));showOptions(matches.slice(0,60));count.textContent=`Showing ${Math.min(matches.length,60)} of ${matches.length} matching choices. Refine the text to find a name, then select it explicitly.`;};
        filter.oninput=update;$('prompt').append(filter,count);update();
      }else showOptions(p.options);
      $('prompt').append(choices);
      const submit = element('button', 'Submit choice'); submit.onclick = () => send('reply', {selected: [...document.querySelectorAll('input[name=choice]:checked')].map(x => Number(x.value))}); $('prompt').append(submit, element('small', `Choose ${p.min}–${p.max}.`)); if(p.min===0 && p.max>0)$('prompt').append(button('Choose none','reply',{selected:[]}));
    } else {
      const okLabel = p.kind?.includes('PayMana') && p.ok === 'Auto' ? 'Auto-pay mana' : p.ok === 'OK' && p.kind === 'InputPassPriority' ? 'Pass priority' : p.ok || 'Continue';
      const ok = button(okLabel, 'ok', {}, !p.okEnabled); ok.className = 'primary-action';
      $('prompt').append(ok);
      if (p.cancelEnabled) $('prompt').append(button(p.cancel || 'Cancel', 'cancel'));
    }
    if (s.mana?.length) $('prompt').append(element('small', 'Click a mana symbol beside your life total to spend floating mana.'));
    renderBoard(s);
    if(moveFocus) {$('prompt').focus();returnFocus=false;}
  }
  function renderBoard(s) {
    lastBoard = s; table.render(s, send, {setPhaseStop});
  }
  const opponentDecks = ['feline-ferocity', 'open-hostility', 'arcane-wizardry', 'draconic-domination', 'vampiric-bloodlust'];
  function hideDefaultList() {
    decklistGeneration++;
    $('defaultList').hidden = true;
    $('inspectDefault').textContent = 'View decklist';
    $('inspectDefault').setAttribute('aria-expanded', 'false');
  }
  $('default').onchange = $('randomOpponent').onchange = () => {
    const random = $('randomOpponent').checked;
    $('default').disabled = random;
    $('customCPU').hidden = random || !!$('default').value;
    $('inspectDefault').hidden = random || !$('default').value;
    hideDefaultList();
  };
  hideDefaultList();
  $('inspectDefault').onclick = async () => {
    if (!$('defaultList').hidden) { hideDefaultList(); return; }
    const selected = $('default').value;
    if (!selected || $('randomOpponent').checked) return;
    const generation = ++decklistGeneration;
    $('defaultList').hidden = false; $('defaultList').textContent = 'Loading decklist…';
    $('inspectDefault').textContent = 'Hide decklist'; $('inspectDefault').setAttribute('aria-expanded', 'true');
    try {
      const d = await api('/defaults/' + encodeURIComponent(selected));
      if (generation !== decklistGeneration || $('default').value !== selected || $('randomOpponent').checked) return;
      $('defaultList').textContent = [d.name,'Commander(s)',...d.commanders.map(c=>`${c.quantity} ${c.name}`),'Main deck',...d.main.map(c=>`${c.quantity} ${c.name}`)].join('\n');
    } catch (error) {
      if (generation !== decklistGeneration) return;
      hideDefaultList(); $('error').textContent = error.message;
    }
  };
  $('diagnostics').onclick = () => {
    // Explicit allowlist: no raw views, deck lists, hand contents, prompt text or credentials.
    const s=session?.state||{};const data={format:'manatomb-diagnostics/v1',protocol:session?.protocol||null,forge:session?.forge||null,build:session?.build||null,sessionReference:session?.id||null,status:session?.status||'no-session',turn:s.turn||0,phase:s.phase||null,promptKind:s.prompt?.kind||null,failureCode:s.failureCode||null,result:s.result||null,conceded:!!s.conceded};
    const text=JSON.stringify(data,null,2)+'\n';$('diagnosticPreview').textContent=text;$('diagnosticPreview').hidden=false;
    const url=URL.createObjectURL(new Blob([text],{type:'application/json'}));const link=element('a','');link.href=url;link.download='manatomb-session-diagnostics.json';document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
  };
  $('start').onclick = async () => {
    if (desktopUnavailable()) return;
    $('start').disabled = true; $('error').textContent = ''; $('gameResult').hidden = true;
    try {
      await configuration;
      const randomOpponent = !!$('randomOpponent').checked, selectedDefault = $('default').value;
      const opponent = randomOpponent ? opponentDecks[Math.floor(Math.random() * opponentDecks.length)] : selectedDefault;
      const human = deck('human');
      if ($('humanSaved').value === 'editor' && editorDeck) human.name = editorDeck.name;
      if (!human.commanders.length || !human.main.length) {
        $('humanDecklist').open = true;
        throw new Error('Choose a saved deck or paste your commander and main deck.');
      }
      const body = {request: crypto.randomUUID(), human, cpu: opponent ? {name: '', commanders: [], main: []} : deck('cpu'), default: opponent};
      try{sessionStorage.setItem('manatomb.cpu.setup',JSON.stringify({owner,body,randomOpponent,selectedDefault,fromEditor:$('humanSaved').value==='editor'}));}catch(_){}
      refreshGeneration++; session = await api('/sessions', 'POST', body); revision = ''; seen = null; lastBoard = null; awaitingRevision = ''; await refresh();
    }
    catch (error) { $('error').textContent = error.message; } finally { $('start').disabled = false; }
  };
  $('concede').onclick = () => { if (state.canConcede && !pending && !awaitingRevision) $('confirmConcede').hidden = !$('confirmConcede').hidden; };
  $('confirmConcede').onclick = () => { $('confirmConcede').hidden = true; if (state.canConcede) return send('concede'); };
  $('requestConcede').onclick=()=>{$('confirmConcedeRequest').hidden=!$('confirmConcedeRequest').hidden;};
  $('confirmConcedeRequest').onclick=async()=>{
    if(!session||terminal(session.status)||!session.state?.session)return;
    $('confirmConcedeRequest').disabled=true;
    try{await api(`/sessions/${session.id}/actions`,'POST',{request:crypto.randomUUID(),session:session.state.session,revision:0,prompt:'',action:'requestConcede'});$('confirmConcedeRequest').hidden=true;await refresh();}catch(error){$('error').textContent=error.message;}finally{$('confirmConcedeRequest').disabled=false;}
  };
  $('openHelp').onclick = () => $('playGuide').showModal();
  $('closeHelp').onclick = () => $('playGuide').close();
  $('cancel').onclick = () => { $('confirmCancel').hidden = !$('confirmCancel').hidden; };
  $('confirmCancel').onclick = async () => { if (!session) return; $('confirmCancel').hidden = true; try { await api('/sessions/' + session.id, 'DELETE'); await refresh(); } catch (error) { $('error').textContent = error.message; } };
  $('reconnect').onclick = () => { refreshGeneration++; session = null; revision = ''; seen = null; awaitingRevision = ''; refresh(); };
  for (const seat of ['human', 'cpu']) $(seat + 'Saved').onchange = async () => {
    if (seat === 'human') $('humanDecklistSummary').textContent = $(seat + 'Saved').value ? 'View or edit decklist' : 'Paste or edit a decklist';
    if (seat === 'human' && $('humanSaved').value === 'editor' && editorDeck) { loadEditorDeck(editorDeck); return; }
    if (!$(seat + 'Saved').value) { deckPrintings[seat].clear(); deckNames[seat] = seat === 'human' ? 'Your Commander deck' : 'Custom CPU deck'; return; }
    try { fillDeck(seat, await api('/decks/' + $(seat + 'Saved').value)); } catch (error) { $('error').textContent = error.message; }
  };
  api('/decks').then(async decks => { for (const d of decks) for (const seat of ['human', 'cpu']) { const option = element('option', d.name); option.value = d.id; $(seat + 'Saved').append(option); } await configuration; const selected = new URLSearchParams(location.search).get('deck'); if (!editorHandoff && selected && decks.some(d => String(d.id) === selected)) { $('humanSaved').value = selected; await $('humanSaved').onchange(); } }).catch(error => { $('error').textContent = error.message; });
  const configuration = api('/config').then(config=>{
    owner=config.owner; readStops();
    try {const previous=JSON.parse(sessionStorage.getItem('manatomb.cpu.setup')||'null');if(previous?.owner===owner && previous.body){if(previous.fromEditor && !new URLSearchParams(location.search).has('deck'))loadEditorDeck(previous.body.human);else fillDeck('human',previous.body.human);fillDeck('cpu',previous.body.cpu);$('default').value=previous.selectedDefault ?? previous.body.default ?? '';$('randomOpponent').checked=previous.randomOpponent===true;$('default').onchange();}}catch(_){}
    let seed;try{seed=JSON.parse(sessionStorage.getItem('manatomb.cpu.start')||'null');}catch(_){}
    if(seed && seed.owner===config.owner){
      editorHandoff = true;
      loadEditorDeck(seed.deck);
      // Retain this handoff across reloads before the first game starts.
      try{sessionStorage.setItem('manatomb.cpu.setup',JSON.stringify({owner,body:{human:seed.deck,cpu:deck('cpu'),default:$('default').value},randomOpponent:!!$('randomOpponent').checked,selectedDefault:$('default').value,fromEditor:true}));sessionStorage.removeItem('manatomb.cpu.start');}catch(_){}
    }
  }).catch(()=>{});
  refresh();
})();
