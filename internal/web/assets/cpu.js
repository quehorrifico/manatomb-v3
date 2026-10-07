'use strict';
(() => {
  const $ = id => document.getElementById(id);
  const element = (tag, text) => { const e = document.createElement(tag); e.textContent = text; return e; };
  let session = null, state = {}, revision = '', pending = false, timer, owner = null, returnFocus = false;
  let activitySession = '', activitySignature = '', activity = [];
  const table = globalThis.ManatombCPUTable;
  let lastBoard = null, refreshInFlight = null, refreshAgain = false, refreshGeneration = 0;
  let seen = null, awaitingRevision = '', fastUntil = 0;
  const pendingStops = new Map();
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
    lastBoard = null; table.closeInspector(); table.reset(); $('game').hidden = true;
    $('players').replaceChildren(); $('stack').replaceChildren(); $('cpuPage').dataset.playing = 'false';
  }
  function observeActivity(next) {
    if (activitySession !== next.id) {activitySession=next.id;activitySignature='';activity=[];}
    const s=next.state||{};
    // Only fixed seat labels and public scalar facts. Never retain prompt/card text.
    const phase=/^[A-Z][A-Z0-9_]{0,31}$/.test(s.phase||'')?s.phase:'';
    const life=(s.players||[]).filter(p=>['Human','CPU'].includes(p.name)&&Number.isFinite(p.life)).map(p=>`${p.name}: ${p.life} life`).join('; ');
    const text=[['starting','input','choice','finished','failed','cancelled','expired'].includes(next.status)?next.status:'Playing',Number.isInteger(s.turn)?`turn ${s.turn}`:'',phase,life,['Human','CPU','Draw'].includes(s.result)?`Result: ${s.result}`:''].filter(Boolean).join(' · ');
    if(text!==activitySignature){activitySignature=text;activity.push(text);activity=activity.slice(-50);$('activity').replaceChildren(...activity.map(text=>element('li',text)));}
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
  function fillDeck(seat, deck) { $(seat + 'Commanders').value = deck.commanders.map(c => c.name).join('\n'); $(seat + 'Main').value = deck.main.map(c => `${c.quantity} ${c.name}`).join('\n'); }
  function deck(seat) {
    const commanders = $(seat + 'Commanders').value.split('\n').map(s => s.trim()).filter(Boolean).map(name => ({name, quantity: 1}));
    const main = $(seat + 'Main').value.split('\n').map(s => s.trim()).filter(Boolean).map(line => {
      const match = /^(\d+)\s+(.+)$/.exec(line); if (!match) throw new Error('Use quantity and card name on every main-deck line: ' + line);
      return {name: match[2], quantity: Number(match[1])};
    });
    return {name: seat === 'human' ? 'Your Commander deck' : 'Custom CPU deck', commanders, main};
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
        reconcileStops(next);
        $('signinAgain').hidden = true; session = next; observeActivity(next);
        $('cpuPage').dataset.playing = String(!terminal(next.status));
        $('game').hidden = false;
        $('requestConcede').hidden = !!(terminal(next.status) || !next.state?.canRequestConcede || next.state?.canConcede || next.state?.concedePending);
        $('concedePending').textContent = !terminal(next.status) && next.state?.concedePending ? 'Concession queued for the next priority. Required choices still need your reply.' : '';
        if (terminal(next.status) || next.state?.concedePending) $('confirmConcedeRequest').hidden = true;
        $('setup').hidden = !terminal(next.status); $('cancel').hidden = terminal(next.status);
        $('status').textContent = next.status === 'starting' ? 'Shuffling up — loading the engine and checking your decks…' : next.status === 'working' ? 'Resolving actions…' : terminal(next.status) ? (next.state?.result ? `Game finished · ${next.state.result === 'Human' ? 'You win' : next.state.result === 'CPU' ? 'CPU wins' : next.state.result}${next.state.conceded ? ' · conceded' : ''}` : `Game ${next.status}. You can start a new game below.`) : '';
        const nearLimit = new Date(next.expiresAt).getTime() - Date.now() < 60000;
        $('expiry').textContent = terminal(next.status) ? '' : `${nearLimit ? 'Approaching session limit. ' : ''}Session ends by ${new Date(next.expiresAt).toLocaleTimeString()}. Reconnect within ${next.reconnectSeconds / 60} minutes after leaving.`;
        if (next.error) $('error').textContent = next.error;
        if (next.state?.prompt && !terminal(next.status)) {
          const key = `${next.id}:${next.state.revision}`;
          if (awaitingRevision && key !== awaitingRevision) { awaitingRevision = ''; fastUntil = 0; }
          if (key !== revision) { revision = key; render(next.state); }
          if (awaitingRevision) $('status').textContent = 'Sending your choice…';
        } else {
          table.closeInspector(); $('prompt').hidden = true; $('prompt').replaceChildren(); delete $('prompt').dataset.prompt;
          $('confirmCancel').hidden = true;
          state = next.state || {};
          if (terminal(next.status)) {
            awaitingRevision = ''; fastUntil = 0;
            if (next.state?.players) renderBoard({...next.state, status: next.status});
            else if (lastBoard) renderBoard({...table.waitingBoard(lastBoard), status: next.status});
          } else if (lastBoard) table.render(table.waitingBoard(lastBoard), send, {working: true});
          else {
            $('players').replaceChildren(element('p', 'Preparing the battlefield…'));
            table.renderPhase(next.state || {}, true);
          }
        }
        table.updateStops(next.state?.phaseStops);
        if (!terminal(next.status)) schedule(next);
      } catch (error) {
        if (generation !== refreshGeneration) { refreshAgain = true; return; }
        if (error.status === 401) {
          session = null; state = {}; revision = ''; seen = null; awaitingRevision = '';
          $('setup').hidden = true; $('status').textContent = 'Sign in again to resume your game.'; $('error').textContent = ''; $('signinAgain').hidden = false;
          $('prompt').hidden = true; $('prompt').replaceChildren(); delete $('prompt').dataset.prompt; clearBoard(); $('expiry').textContent = ''; $('concedePending').textContent = '';
          for (const id of ['cancel', 'confirmCancel', 'requestConcede', 'confirmConcedeRequest']) $(id).hidden = true;
        } else if (error.status === 404) {
          session = null; state = {}; revision = ''; seen = null; awaitingRevision = '';
          $('setup').hidden = false; $('status').textContent = 'Choose your deck and take a seat.'; $('error').textContent = ''; $('prompt').hidden = true; $('prompt').replaceChildren();
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
      // Acknowledged preferences can precede the supervisor's next cached view.
      if (session?.id === current.id) pendingStops.set(`${seat}:${phase}`, {session:current.id, seat, phase, enabled, until:Date.now()+5000});
      await refresh();
    } catch (error) { $('error').textContent = error.message; }
  }
  function button(text, action, extra, disabled = false) { const b = element('button', text); b.disabled = disabled; b.onclick = () => send(action, extra); return b; }
  function render(s) {
    const moveFocus=returnFocus||$('prompt').contains?.(document.activeElement)||$('players').contains?.(document.activeElement)||$('zoneBrowser').contains?.(document.activeElement);
    state = s; const p = s.prompt; $('prompt').dataset.prompt = p.id; $('prompt').hidden = false;
    const [title, guide] = table.guidance(s);
    const kicker = element('p', s.phase ? table.phaseName(s.phase) : 'At the table'); kicker.className = 'decision-kicker';
    const hint = element('p', guide); hint.className = 'decision-guide';
    const text = element('p', p.kind === 'orderSimultaneousAbilities' ? 'Choose which ability resolves next. Move abilities up or down, then confirm. Later responses may resolve before these.' : p.text); text.className = 'decision-text';
    $('prompt').replaceChildren(kicker, element('h2', title), hint, text);
    $('error').textContent = s.actionError || '';
    if (s.status === 'choice' && ['integer','text','amounts'].includes(p.valueType)) {
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
    if(s.canConcede){const concede=element('button','Concede');concede.onclick=()=>{const confirm=button('Confirm concede (Forge records a loss)','concede');concede.replaceWith(confirm);};$('prompt').append(concede);}
    renderBoard(s);
    if(moveFocus) {$('prompt').focus();returnFocus=false;}
  }
  function renderBoard(s) {
    lastBoard = s; table.render(s, send, {setPhaseStop});
  }
  const defaultStyles = {'feline-ferocity':'Cats and equipment; creature combat.','open-hostility':'Four-color creature pressure and attack rewards.','arcane-wizardry':'Wizards, enter-the-battlefield effects and graveyard value.','draconic-domination':'Five-color ramp into large Dragons.','vampiric-bloodlust':'Vampires, token production and sacrifice value.'};
  $('default').onchange = () => { $('defaultStyle').textContent=defaultStyles[$('default').value]||'Paste a custom list or choose one of your saved Commander decks.'; $('customCPU').hidden = !!$('default').value; $('inspectDefault').hidden = !$('default').value; $('defaultList').hidden = true; };
  $('inspectDefault').onclick = async () => { const selected=$('default').value; if(!selected)return; try {const d=await api('/defaults/'+encodeURIComponent(selected));if($('default').value!==selected)return;$('defaultList').textContent=[d.name,'Commander(s)',...d.commanders.map(c=>`${c.quantity} ${c.name}`),'Main deck',...d.main.map(c=>`${c.quantity} ${c.name}`)].join('\n');$('defaultList').hidden=false;}catch(error){$('error').textContent=error.message;} };
  $('diagnostics').onclick = () => {
    // Explicit allowlist: no raw views, deck lists, hand contents, prompt text or credentials.
    const s=session?.state||{};const data={format:'manatomb-diagnostics/v1',protocol:session?.protocol||null,forge:session?.forge||null,build:session?.build||null,sessionReference:session?.id||null,status:session?.status||'no-session',turn:s.turn||0,phase:s.phase||null,promptKind:s.prompt?.kind||null,failureCode:s.failureCode||null,result:s.result||null,conceded:!!s.conceded};
    const text=JSON.stringify(data,null,2)+'\n';$('diagnosticPreview').textContent=text;$('diagnosticPreview').hidden=false;
    const url=URL.createObjectURL(new Blob([text],{type:'application/json'}));const link=element('a','');link.href=url;link.download='manatomb-session-diagnostics.json';document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
  };
  $('start').onclick = async () => {
    $('start').disabled = true; $('error').textContent = '';
    try { const body = {request: crypto.randomUUID(), human: deck('human'), cpu: $('default').value ? {name: '', commanders: [], main: []} : deck('cpu'), default: $('default').value}; await configuration; try{sessionStorage.setItem('manatomb.cpu.setup',JSON.stringify({owner,body}));}catch(_){} refreshGeneration++; session = await api('/sessions', 'POST', body); revision = ''; seen = null; lastBoard = null; awaitingRevision = ''; await refresh(); }
    catch (error) { $('error').textContent = error.message; } finally { $('start').disabled = false; }
  };
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
  for (const seat of ['human', 'cpu']) $(seat + 'Saved').onchange = async () => { if (!$(seat + 'Saved').value) return; try { fillDeck(seat, await api('/decks/' + $(seat + 'Saved').value)); } catch (error) { $('error').textContent = error.message; } };
  api('/decks').then(decks => { for (const d of decks) for (const seat of ['human', 'cpu']) { const option = element('option', d.name); option.value = d.id; $(seat + 'Saved').append(option); } const selected = new URLSearchParams(location.search).get('deck'); if (selected && decks.some(d => String(d.id) === selected)) { $('humanSaved').value = selected; $('humanSaved').onchange(); } }).catch(error => { $('error').textContent = error.message; });
  const configuration = api('/config').then(config=>{
    owner=config.owner;
    try {const previous=JSON.parse(sessionStorage.getItem('manatomb.cpu.setup')||'null');if(previous?.owner===owner && previous.body){fillDeck('human',previous.body.human);fillDeck('cpu',previous.body.cpu);$('default').value=previous.body.default||'';$('default').onchange();}}catch(_){}
    try {const previous=JSON.parse(sessionStorage.getItem('manatomb.cpu.return')||'null');if(previous?.owner===owner && /^\/decks\//.test(previous.path) && !/[\\\r\n]/.test(previous.path)){$('returnDeck').href=previous.path;$('returnDeck').hidden=false;}}catch(_){}
    let seed;try{seed=JSON.parse(sessionStorage.getItem('manatomb.cpu.start')||'null');}catch(_){}
    if(seed && seed.owner===config.owner){fillDeck('human',seed.deck);sessionStorage.removeItem('manatomb.cpu.start');}
  }).catch(()=>{});
  refresh();
})();
