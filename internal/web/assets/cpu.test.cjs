const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

function page(storage = new Map(), mobile = false, setupStorage = new Map(), account = {}) {
  class Element {
    constructor(tag = 'div', text = '') { this.tag = tag; this.textContent = text; this.children = []; this.dataset = {}; this.value = ''; }
    append(...items) { this.children.push(...items); }
    replaceChildren(...items) { this.children = items; }
    setAttribute(key, value) { this[key] = value; }
    getAttribute(key) { return this[key] ?? null; }
    click() {}
    remove() {}
    contains(node) {return node===this||this.children.some(c=>c.contains?.(node));}
    focus() {sandbox.document.activeElement=this;}
    set disabled(value) {this._disabled=value;if(value&&sandbox.document.activeElement===this)sandbox.document.activeElement=sandbox.document.body;}
    get disabled() {return this._disabled;}
  }
  const nodes = new Map();
  const get = id => { if (!nodes.has(id)) nodes.set(id, new Element()); return nodes.get(id); };
  let view = {id: 'game-one', protocol:'manatomb-forge/v1', forge:'pinned-engine', build:'runtime-hash', status: 'input', expiresAt: new Date().toISOString(), reconnectSeconds: 300, state: {
    session: 'worker-one', revision: 4, status: 'input', prompt: {id: 'prompt-one', text: 'Priority', ok: 'OK', okEnabled: true, cancel: 'Cancel'},
    players: [{id: 1, name: 'Human', life: 40, zones: []}],
  }};
  let failure = '', timer, downloaded, expired=false, randomValue=0; const posts=[], urls=[]; let savedDeck = account.deck || null, defaultWait = null;
  const sandbox = {
    document: {body: new Element(), getElementById: get, createElement: tag => new Element(tag), createElementNS: (_, tag) => new Element(tag), createTextNode: text => new Element('text', text), querySelectorAll: selector => selector === '[data-stop-phase]' ? descendants(get('players')).filter(e=>e.dataset.stopPhase) : selector.includes('#prompt button') ? get('prompt').children.filter(e=>e.tag==='button') : []},
    fetch: async (url, options = {}) => {
      urls.push(url);
      if(options.method==='POST')posts.push(JSON.parse(options.body));
      if (url.includes('/defaults/') && defaultWait) await defaultWait;
      let body = url.endsWith('/config') ? {owner: 1} : url.endsWith('/decks') ? (account.decks || []) : url.includes('/decks/') ? savedDeck : url.includes('/defaults/') ? {name:'Feline Ferocity',commanders:[{name:'Arahbo, Roar of the World',quantity:1}],main:[{name:'Forest',quantity:99}]} : view;
      const reject = expired || options.method === 'POST' && failure;
      if (reject) body = {error: failure};
      return {ok: !reject, status: expired ? 401 : reject ? 409 : 200, headers: {get: () => 'application/json'}, json: async () => body};
    },
    URL: {createObjectURL: blob => {downloaded = blob.parts.join(''); return 'blob:test';}, revokeObjectURL() {}}, Blob: class {constructor(parts){this.parts=parts;}},
    matchMedia: () => ({matches:mobile}), crypto: {randomUUID: () => 'request-one'}, setTimeout: (callback, delay) => { if (callback.name === 'refresh') { timer = callback; sandbox.pollDelay = delay; } }, clearTimeout() {},
    localStorage: {getItem: key => storage.get(key) || null, setItem: (key,value) => storage.set(key,value)},
    sessionStorage: {getItem: key => setupStorage.get(key) || null, setItem: (key,value) => setupStorage.set(key,value), removeItem:key=>setupStorage.delete(key)}, location: {search: account.search || ''}, URLSearchParams,
    Math: Object.assign(Object.create(Math), {random:()=>randomValue}),
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(__dirname + '/cpu-table.js', 'utf8'), sandbox);
  vm.runInContext(fs.readFileSync(__dirname + '/cpu.js', 'utf8'), sandbox);
  return {get, posts, urls, deferDefault:promise=>defaultWait=promise, random:value=>randomValue=value, savedDeck:deck=>savedDeck=deck, pollDelay:()=>sandbox.pollDelay, table:sandbox.ManatombCPUTable, expire:()=>{expired=true;timer=null;}, nextPoll:()=>timer, updateView:patch=>Object.assign(view,patch), focused:()=>sandbox.document.activeElement, update: patch => Object.assign(view.state,patch), exported: () => downloaded, poison: () => {view.state.players=[{name:'Private hand name',zones:[]}];view.state.prompt.text='Hidden rule data';view.state.secret='private-token';view.error='private deck validation text';}, settle: () => new Promise(resolve => setImmediate(resolve)), refresh: () => timer(), savedRefresh:()=>timer, reject: message => {failure = message;}, end: status => {view = {...view, status};}};
}

test('a rejected reply remains visible after refreshing the unchanged prompt', async () => {
  const p = page(); await p.settle();
  p.reject('Assignment does not satisfy Forge combat damage order/lethal constraints');
  await p.get('prompt').children.find(e => e.tag === 'button' && e.textContent === 'OK').onclick();
  assert.match(p.get('error').textContent, /Forge combat damage/);
  assert.equal(p.get('prompt').dataset.prompt, 'prompt-one');
  assert.equal(p.get('prompt').children.find(e => e.tag === 'button').disabled, false);
});

test('terminal snapshots remove the battlefield and show a result above retained setup', async () => {
  for (const status of ['finished', 'failed', 'expired', 'cancelled']) {
    const p = page(); await p.settle(); p.get('humanMain').value = '99 Plains'; p.end(status); await p.refresh();
    assert.equal(p.get('prompt').hidden, true);
    assert.equal(p.get('prompt').dataset.prompt, undefined);
    assert.equal(p.get('prompt').children.length, 0);
    assert.equal(p.get('players').children.length, 0);
    assert.equal(p.get('game').hidden, true);
    assert.equal(p.get('gameResult').hidden, false);
    assert.equal(p.get('cpuPage').dataset.playing, 'false');
    assert.equal(p.get('phases').children.length, 0);
    assert.equal(p.get('setup').hidden, false);
    assert.equal(p.get('humanMain').value, '99 Plains');
    assert.equal(p.get('cancel').hidden, true);
    assert.equal(p.get('expiry').textContent, '');
    assert.equal(p.get('error').textContent, '');
    p.updateView({status: 'input'}); p.get('reconnect').onclick(); await p.settle();
    assert.equal(p.get('gameResult').hidden, true);
    assert.equal(p.get('setup').hidden, true);
    assert.equal(p.get('game').hidden, false);
  }
});

test('downloaded diagnostics exclude private state, prompt prose and arbitrary errors', async () => {
  const p = page(); await p.settle(); p.poison(); await p.refresh();
  p.get('diagnostics').onclick();
  const exported = p.exported();
  assert.equal(JSON.parse(exported).format, 'manatomb-diagnostics/v1');
  assert.equal(JSON.parse(exported).forge,'pinned-engine');
  assert.equal(JSON.parse(exported).build,'runtime-hash');
  assert.equal(p.get('diagnosticPreview').textContent,exported);
  assert.equal(p.get('diagnosticPreview').hidden,false);
  assert.equal(JSON.parse(exported).sessionReference,'game-one');
  for(const privateText of ['Private hand name','Hidden rule data','private-token','private deck validation text','players','zones']) assert.equal(exported.includes(privateText),false,privateText);
});


test('typed public events replace status snapshots, stay bounded and survive polls', async () => {
  const p=page();await p.settle();p.poison();
  const log=Array.from({length:230},(_,i)=>({id:i+1,turn:5,player:'Human',text:`You cast spell ${i+1}, targeting Opponent.`}));
  p.update({log});await p.refresh();
  assert.equal(p.get('activity').children.length,201); // One turn heading plus 200 events.
  await p.refresh();assert.equal(p.get('activity').children.length,201);
  const text=textOf(p.get('activity'));
  assert.match(text,/Turn 5 · You/);assert.match(text,/cast spell 230/);assert.doesNotMatch(text,/cast spell 30,/);
  for(const value of ['Private hand name','Hidden rule data','private-token','private deck validation text'])assert.equal(text.includes(value),false);
  p.updateView({status:'working',state:{session:'worker-one',revision:5,status:'working',log}});await p.refresh();
  assert.match(textOf(p.get('activity')),/cast spell 230/);
});

test('a keyboard reply restores focus after disabling the submitted control',async()=>{
 const p=page();await p.settle();
 const control=p.get('prompt').children.find(e=>e.tag==='button'&&e.textContent==='OK');
 control.focus();p.reject('stale reply');await control.onclick();
 // A rejection republishes the current controls; focus belongs to that decision.
 assert.equal(p.focused(),p.get('prompt'));
});


test('queued concession is explicit session intent while the engine works',async()=>{
 const p=page();await p.settle();
 p.updateView({status:'working',state:{session:'worker-one',revision:5,canRequestConcede:true}});await p.refresh();
 assert.equal(p.get('requestConcede').hidden,false);
 assert.equal(p.posts.length,0);
 p.get('requestConcede').onclick();assert.equal(p.posts.length,0);
 await p.get('confirmConcedeRequest').onclick();
 assert.equal(p.posts.length,1);assert.equal(p.posts[0].action,'requestConcede');
 assert.equal(p.posts[0].session,'worker-one');assert.equal(p.posts[0].revision,0);assert.equal(p.posts[0].prompt,'');
});


test('authorization expiry clears private controls and stops polling until sign-in',async()=>{
 const p=page();await p.settle();const refresh=p.savedRefresh();p.expire();await refresh();
 assert.equal(p.get('signinAgain').hidden,false);assert.equal(p.get('players').children.length,0);
 assert.equal(p.get('prompt').children.length,0);assert.equal(p.get('setup').hidden,true);
 assert.equal(p.nextPoll(),null);
 for(const id of ['cancel','requestConcede','confirmConcedeRequest'])assert.equal(p.get(id).hidden,true);
});


test('large single-choice catalogs render a bounded searchable set with original IDs',async()=>{
 const p=page();await p.settle();
 const options=Array.from({length:33640},(_,i)=>({id:i,label:i===33123?'Vampire Noble':`Catalog option ${i}`}));
 p.update({revision:9,status:'choice',prompt:{id:'names',kind:'one',text:'Choose a nonland card name.',min:1,max:1,options}});await p.refresh();
 const children=p.get('prompt').children;const filter=children.find(e=>e.tag==='input'&&e['aria-label']==='Filter offered choices');
 const choices=children.find(e=>e.tag==='div');assert.equal(choices.children.length,60);
 filter.value='Vampire Noble';filter.oninput();assert.equal(choices.children.length,1);
 const radio=choices.children[0].children[0];assert.equal(radio.value,33123);assert.equal(radio.checked,undefined);assert.equal(p.posts.length,0);
 filter.value='not an offered name';filter.oninput();assert.equal(choices.children.length,0);assert.equal(p.posts.length,0);
});


test('ordinary single choices use one explicit click and never auto-select', async () => {
  const p=page(); await p.settle();
  p.update({revision:5,status:'choice',prompt:{id:'ability',kind:'getAbilityToPlay',text:'Choose an ability',min:0,max:1,options:[{id:17,label:'Add green mana'}]}}); await p.refresh();
  assert.equal(p.posts.length,0);
  const actions=p.get('prompt').children.find(e=>e.className==='choice-actions');
  assert.equal(actions.children.length,1);
  await actions.children[0].onclick();
  assert.equal(p.posts.length,1);
  assert.equal(p.posts[0].action,'reply');
  assert.deepEqual(p.posts[0].selected,[17]);
  await actions.children[0].onclick();
  assert.equal(p.posts.length,1,'a second click cannot submit the pending choice again');
});

test('retained working boards strip temporary reveals and private-zone views', async () => {
  const p=page(); await p.settle();
  const publicCard={name:'Forest',id:1,temporaryReveal:false};
  const revealed={name:'Private name',id:2,temporaryReveal:true};
  const legacy={name:'Unclassified legacy reveal',id:3};
  const filtered=p.table.waitingBoard({players:[{name:'Human',zones:[{zone:'Hand',cards:[publicCard]}]},{name:'CPU',zones:[{zone:'Battlefield',cards:[publicCard,revealed,legacy]},{zone:'Hand',cards:[revealed]},{zone:'Library',cards:[revealed]}]}]});
  assert.equal(filtered.status,'working');
  assert.equal(filtered.players[0].zones[0].cards.length,1);
  assert.deepEqual(JSON.parse(JSON.stringify(filtered.players[1].zones[0].cards)),[publicCard]);
  assert.equal(filtered.players[1].zones[1].cards.length,0);
  assert.equal(filtered.players[1].zones[2].cards.length,0);
});

test('working state retains a noninteractive tabletop and phase context',async()=>{
  const p=page();await p.settle();
  p.update({turn:3,phase:'MAIN1',turnPlayer:'Human',priorityPlayer:'Human'});await p.refresh();
  p.updateView({status:'working',state:{session:'worker-one',revision:5}});await p.refresh();
  assert.equal(p.get('players').children.length,1);
  assert.equal(p.get('players').children[0].children[0].children[0].disabled,true);
  assert.equal(p.get('boardNotice').hidden,false);
  assert.equal(p.get('turnOwner').textContent,'Your turn');
  assert.equal(p.get('prompt').hidden,false);
  assert.match(textOf(p.get('prompt')),/Your response.*Resolving actions/);
});

test('older snapshot revisions cannot restore stale controls',async()=>{
  const p=page();await p.settle();
  p.update({revision:8,prompt:{id:'current',kind:'InputPassPriority',text:'Current',ok:'OK',okEnabled:true}});await p.refresh();
  p.update({revision:7,prompt:{id:'obsolete',text:'Old'}});await p.refresh();
  assert.equal(p.get('prompt').dataset.prompt,'current');
  assert.equal(p.pollDelay(),2000,'idle human polling leaves budget for actions and artwork');
});

function descendants(node) { return [node, ...node.children.flatMap(descendants)]; }
function textOf(node) { return descendants(node).map(n => n.textContent).join(' '); }
function tableState(cards, zone = 'Hand') {
  return {status:'input', players:[{id:1, name:'Human', life:40, zones:[{zone, count:cards.length, cards}]}], stack:[]};
}

test('hover and keyboard previews do not submit actions, including during a choice', async () => {
  const p=page();await p.settle();const sent=[];
  const card={id:4,name:'Island',type:'Basic Land — Island',temporaryReveal:false};
  p.table.render({...tableState([card]),status:'choice'},(...args)=>sent.push(args));
  const slot=descendants(p.get('players')).find(n=>n.className==='card-slot');
  slot.onmouseenter();assert.match(textOf(p.get('cardDetail')),/Island/);
  const control=slot.children[0];control.onfocus();control.onclick();
  assert.equal(sent.length,0);assert.equal(control['aria-disabled'],'true');
  assert.equal(descendants(p.get('cardPicture')).find(n=>n.tag==='img').loading,'eager');
  p.table.render(tableState([card]),(...args)=>sent.push(args));
  descendants(p.get('players')).find(n=>n.className==='card').onclick();
  assert.equal(sent.length,1);assert.equal(sent[0][0],'card');assert.equal(sent[0][1].id,4);
});

test('library browser shows counts without hidden identities and refreshes revealed cards',async()=>{
  const p=page();await p.settle();const sent=[];
  const state=tableState([], 'Library');state.players[0].zones[0].count=92;
  p.table.render(state,(...args)=>sent.push(args));
  descendants(p.get('players')).find(n=>n.dataset.zoneName==='Library').onclick();
  assert.equal(p.get('zoneBrowser').hidden,false);assert.match(p.get('zoneBrowserTitle').textContent,/Your library · 92/);
  assert.match(p.get('zoneBrowserHint').textContent,/92 cards are hidden/);
  assert.equal(descendants(p.get('zoneBrowserCards')).filter(n=>n.className==='card').length,0);
  const revealed={id:9,name:'Cultivate',temporaryReveal:true};
  state.players[0].zones[0].cards=[revealed];p.table.render(state,(...args)=>sent.push(args));
  descendants(p.get('zoneBrowserCards')).find(n=>n.className==='card-slot').onmouseenter();
  assert.match(textOf(p.get('cardDetail')),/Cultivate/);assert.equal(sent.length,0);
  p.table.render(p.table.waitingBoard(state),()=>{}, {working:true});
  assert.doesNotMatch(textOf(p.get('cardDetail')),/Cultivate/);
  assert.doesNotMatch(textOf(p.get('zoneBrowserCards')),/Cultivate/);
  p.get('closeZoneBrowser').onclick();assert.equal(p.get('zoneBrowser').hidden,true);
});

test('preview follows only authorized current cards and is cleared on session loss',async()=>{
  const p=page();await p.settle();
  const state=tableState([{id:2,name:'Forest',temporaryReveal:false}]);
  p.table.render(state,()=>{});descendants(p.get('players')).find(n=>n.className==='card-slot').onmouseenter();
  p.table.render(p.table.waitingBoard(state),()=>{},{working:true});assert.match(textOf(p.get('cardDetail')),/Forest/);
  p.table.render(tableState([{id:2,name:'Face-down card'}]),()=>{});
  assert.doesNotMatch(textOf(p.get('cardDetail')),/Forest/);
  p.table.render(state,()=>{});descendants(p.get('players')).find(n=>n.className==='card-slot').onmouseenter();
  const refresh=p.savedRefresh();p.expire();await refresh();assert.doesNotMatch(textOf(p.get('cardDetail')),/Forest/);
  assert.equal(p.get('zoneBrowser').hidden,true);
});

test('public stack sources preview without card actions and clear after resolution',async()=>{
  const p=page();await p.settle();const state=tableState([]);
  state.stack=[{source:'Counterspell',text:'Counter target spell.'}];p.table.render(state,()=>{});
  p.get('stack').children[1].onfocus();assert.match(textOf(p.get('cardDetail')),/Counterspell/);
  state.stack=[];p.table.render(state,()=>{});assert.doesNotMatch(textOf(p.get('cardDetail')),/Counterspell/);
  state.stack=[{source:'Hidden source',text:''}];p.table.render(state,()=>{});
  assert.equal(p.get('stack').children[1].onfocus,undefined);
});

test('floating mana stays visible outside payment and only offered human mana can be spent',async()=>{
  const p=page();await p.settle();const sent=[],state=tableState([]);
  state.players[0].mana=[{id:16,label:'green',amount:3}];
  state.players.push({id:2,name:'CPU',life:40,mana:[{id:16,label:'green',amount:2}],zones:[]});
  p.table.render(state,(...args)=>sent.push(args));
  let chips=descendants(p.get('players')).filter(n=>n.className?.startsWith('mana-chip'));
  assert.equal(chips.length,18);assert.equal(chips.filter(n=>n.tag==='button').length,0);
  assert.ok(chips.some(n=>n['aria-label']==='3 green mana'));
  state.mana=[{id:16,label:'green',amount:3}];p.table.render(state,(...args)=>sent.push(args));
  chips=descendants(p.get('players')).filter(n=>n.className?.startsWith('mana-chip'));
  const spend=chips.filter(n=>n.tag==='button');assert.equal(spend.length,1);
  spend[0].onclick();assert.equal(sent[0][0],'mana');assert.equal(sent[0][1].id,16);
  p.table.render(state,()=>{}, {working:true});
  assert.equal(descendants(p.get('players')).filter(n=>n.className?.startsWith('mana-chip')&&n.tag==='button').length,0);
});

test('phase-stop intent preserves the prompt and independently refreshes both seats',async()=>{
  const p=page();await p.settle();
  p.update({revision:5,phaseStops:{Human:['MAIN1'],CPU:['DRAW']},players:[{id:1,name:'Human',life:40,zones:[]},{id:2,name:'CPU',life:40,zones:[]}]});await p.refresh();
  const stops=()=>descendants(p.get('players')).filter(n=>n.dataset.stopPhase);
  assert.equal(stops().length,26);
  assert.ok(stops().filter(n=>n.dataset.stopPhase==='UNTAP').every(n=>n.disabled));
  const cpuDraw=stops().find(n=>n.dataset.stopSeat==='CPU'&&n.dataset.stopPhase==='DRAW');
  assert.equal(cpuDraw['aria-pressed'],'true');await cpuDraw.onclick();
  const action=p.posts.at(-1);assert.equal(action.action,'setPhaseStop');assert.equal(action.seat,'CPU');assert.equal(action.phase,'DRAW');assert.equal(action.enabled,false);assert.equal(action.revision,0);assert.equal(action.prompt,'');
  assert.equal(cpuDraw['aria-pressed'],'false','acknowledged setting is not reverted by a stale cached view');
  assert.equal(p.pollDelay(),300,'briefly poll for preference confirmation');
  const prompt=p.get('prompt').children[0];
  p.update({phaseStops:{Human:['MAIN1'],CPU:[]}});await p.refresh();
  assert.equal(cpuDraw['aria-pressed'],'false');assert.equal(p.get('prompt').children[0],prompt);
  assert.equal(stops().find(n=>n.dataset.stopSeat==='Human'&&n.dataset.stopPhase==='MAIN1')['aria-pressed'],'true');
});

test('selected blockers announce their attacker and retain an explicit selection marker',async()=>{
  const p=page();await p.settle();const state=tableState([{id:11,name:'Bear',selected:true,blocking:true},{id:12,name:'Dragon',attacking:true}],'Battlefield');
  state.combat=[{attacker:12,defender:{type:'player',id:1},blockers:[11],plannedBlockers:[11]}];p.table.render(state,()=>{});
  const blocker=descendants(p.get('players')).find(n=>n.dataset.cardId==='11');
  assert.match(blocker['aria-label'],/selected, Blocks Dragon/);assert.equal(blocker.title,'Blocks Dragon');
  assert.ok(blocker.children.some(n=>n.className==='selection-badge'));
});

test('auto-pay uses the current Forge OK action and honors its availability',async()=>{
  const p=page();await p.settle();
  p.update({revision:5,prompt:{id:'pay-mana',kind:'InputPayManaOfCostPayment',ok:'Auto',okEnabled:true}});await p.refresh();
  const pay=p.get('prompt').children.find(n=>n.textContent==='Auto-pay mana');assert.equal(pay.disabled,false);await pay.onclick();
  assert.equal(p.posts.at(-1).action,'ok');assert.equal(p.posts.at(-1).prompt,'pay-mana');assert.equal(p.posts.at(-1).revision,5);
  p.update({revision:6,prompt:{id:'pay-mana-2',kind:'InputPayManaOfCostPayment',ok:'Auto',okEnabled:false}});await p.refresh();
  assert.equal(p.get('prompt').children.find(n=>n.textContent==='Auto-pay mana').disabled,true);
});

test('simultaneous triggers from the same source retain the explicitly chosen order',async()=>{
  const p=page();await p.settle();
  p.updateView({status:'choice'});
  p.update({revision:5,status:'choice',prompt:{id:'gnawbone-triggers',kind:'orderSimultaneousAbilities',ordering:true,min:2,max:2,text:'Choose which trigger resolves next.',options:[
    {id:0,label:'Old Gnawbone — Create that many Treasure tokens. · Damage from Raph & Mikey, Troublemakers to CPU: 7'},
    {id:1,label:'Old Gnawbone — Create that many Treasure tokens. · Damage from Old Gnawbone to CPU: 7'},
  ]}});await p.refresh();
  const list=descendants(p.get('prompt')).find(n=>n.className==='ordering-list');
  assert.equal(p.posts.length,0,'ordering is never submitted automatically');
  list.children[1].children.find(n=>n['aria-label']?.startsWith('Move up:')).onclick();
  assert.match(textOf(list.children[0]),/Damage from Old Gnawbone to CPU: 7/);
  assert.doesNotMatch(textOf(list),/Create that many/);
  assert.match(textOf(p.get('prompt')),/Create that many Treasure tokens/);
  await p.get('prompt').children.find(n=>n.textContent==='Use this order').onclick();
  assert.equal(p.posts.at(-1).action,'reply');assert.equal(p.posts.at(-1).prompt,'gnawbone-triggers');
  assert.deepEqual(p.posts.at(-1).selected,[1,0]);
});


test('read-only Forge notices show a Continue acknowledgement without disabled choices', async () => {
  const p=page();await p.settle();
  p.update({status:'choice',revision:5,prompt:{id:'notice',kind:'reveal',text:'Cards revealed by the game',min:0,max:0,options:[{id:0,label:'Divine Reckoning'},{id:1,label:'Abundance'}]}});
  await p.refresh();
  assert.match(textOf(p.get('prompt')),/Your response.*Cards revealed.*Divine Reckoning.*Abundance/);
  assert.equal(descendants(p.get('prompt')).filter(n=>n.tag==='input').length,0);
  assert.doesNotMatch(textOf(p.get('prompt')),/Submit choice|Choose 0/);
  await descendants(p.get('prompt')).find(n=>n.tag==='button'&&n.textContent==='Continue').onclick();
  assert.equal(p.posts[0].action,'reply');assert.deepEqual(p.posts[0].selected,[]);
});

test('the two fields and player hand are separate table rows with hover-only inspection',async()=>{
  const p=page();await p.settle();const sent=[];
  const card={id:7,name:'Forest',type:'Basic Land — Forest'};
  p.table.render({status:'input',players:[{id:1,name:'Human',life:40,zones:[{zone:'Hand',count:1,cards:[card]},{zone:'Battlefield',count:0,cards:[]}]},{id:2,name:'CPU',life:40,zones:[{zone:'Battlefield',count:0,cards:[]}]}]},(...a)=>sent.push(a));
  const rows=p.get('players').children;
  assert.match(rows[0].className,/seat-cpu/);assert.match(rows[1].className,/seat-human/);assert.match(rows[2].className,/zone-hand/);
  assert.equal(descendants(p.get('players')).some(n=>n.className==='inspect-card'),false);
  const slot=descendants(rows[2]).find(n=>n.className==='card-slot');slot.onmouseenter();slot.children[0].onfocus();
  assert.match(textOf(p.get('cardDetail')),/Forest/);assert.equal(sent.length,0);
});


test('concession lives in Game options and confirms against the current decision',async()=>{
  const p=page();await p.settle();
  p.update({revision:5,canConcede:true});await p.refresh();
  assert.equal(p.get('concede').hidden,false);
  assert.equal(descendants(p.get('prompt')).some(n=>n.textContent==='Concede'),false);
  p.get('concede').onclick();assert.equal(p.get('confirmConcede').hidden,false);
  assert.equal(p.posts.length,0);
  p.update({revision:6,prompt:{id:'new-priority',ok:'OK',okEnabled:true}});await p.refresh();
  assert.equal(p.get('confirmConcede').hidden,true);
  p.get('concede').onclick();await p.get('confirmConcede').onclick();
  assert.equal(p.posts.at(-1).action,'concede');assert.equal(p.posts.at(-1).revision,6);
  assert.equal(p.posts.at(-1).prompt,'new-priority');
  p.end('finished');await p.refresh();
  assert.equal(p.get('gameStatus').hidden,true);assert.equal(p.get('concede').hidden,true);
});

test('mana symbols render safely in previews, stack text and floating mana',async()=>{
  const p=page();await p.settle();const sent=[];
  const card={id:7,name:'Seething Song',type:'Instant',mana:'{2}{R}',rules:'Add {R}{R}{R}{R}{R}. <script>private</script>'};
  p.table.render({status:'input',stack:[{source:'Seething Song',text:'Add {R}{R}.'}],players:[{id:1,name:'Human',life:40,mana:[{id:16,label:'red',amount:3}],zones:[{zone:'Hand',count:1,cards:[card]}]}]},(...a)=>sent.push(a));
  descendants(p.get('players')).find(n=>n.className==='card-slot').onmouseenter();
  const symbols=descendants(p.get('cardDetail')).filter(n=>n.className==='game-symbol');
  assert.equal(symbols.length,7);assert.equal(symbols[0].alt,'{2}');
  assert.equal(symbols[1].src,'https://svgs.scryfall.io/card-symbols/R.svg');
  assert.match(textOf(p.get('cardDetail')),/<script>private<\/script>/);
  assert.equal(descendants(p.get('cardDetail')).some(n=>n.tag==='script'),false);
  assert.equal(descendants(p.get('stack')).filter(n=>n.className==='game-symbol').length,2);
  assert.equal(descendants(p.get('players')).filter(n=>n.className==='mana-chip mana-red')[0].children[0].children[0].alt,'{R}');
  const hybrid=p.table.symbolText('p','{R/G} {W/P} {T}');
  assert.deepEqual(hybrid.children.filter(n=>n.tag==='img').map(n=>n.src.split('/').at(-1)),['RG.svg','WP.svg','T.svg']);
  assert.equal(sent.length,0);
});

test('each battlefield keeps its cards in one zone with lands after nonlands',async()=>{
  const p=page();await p.settle();const sent=[];
  p.table.render({status:'input',players:[{id:1,name:'Human',life:40,zones:[{zone:'Battlefield',count:2,cards:[{id:8,name:'Forest',type:'Basic Land — Forest'},{id:7,name:'Grizzly Bears',type:'Creature — Bear'}]}]}]},(...a)=>sent.push(a));
  const battlefield=descendants(p.get('players')).find(n=>n.className==='zone zone-battlefield');
  const rows=battlefield.children[1].children;
  assert.equal(rows[0].dataset.lane,'Permanents');assert.equal(rows[1].dataset.lane,'Lands');
  assert.equal(descendants(battlefield).some(n=>n.className==='row-label'||n.className==='battlefield-empty'),false);
  descendants(rows[1]).find(n=>n.dataset.cardId==='8').onclick();
  assert.equal(sent[0][0],'card');assert.equal(sent[0][1].id,8);
});


test('rail tabs use complete panels, keyboard navigation and retain the decision', async()=>{
 const p=page();await p.settle();
 p.get('logTab').onclick();assert.equal(p.get('stackPanel').hidden,true);assert.equal(p.get('logPanel').hidden,false);
 assert.equal(p.get('logTab').getAttribute('aria-selected'),'true');
 p.get('logTab').onkeydown({key:'ArrowRight',preventDefault(){}});assert.equal(p.get('optionsPanel').hidden,false);assert.equal(p.focused(),p.get('optionsTab'));
 p.update({revision:5});await p.refresh();assert.equal(p.get('optionsPanel').hidden,false);
 p.get('stackTab').onclick();assert.equal(p.get('stackPanel').hidden,false);assert.equal(p.get('prompt').dataset.prompt,'prompt-one');assert.equal(p.posts.length,0);
});

test('opponent lands occupy the outer edge above their other permanents', async()=>{
 const p=page();await p.settle();p.table.render({status:'input',players:[{id:2,name:'CPU',life:40,zones:[{zone:'Battlefield',count:2,cards:[{id:1,name:'Forest',type:'Basic Land'},{id:2,name:'Bear',type:'Creature'}]}]}]},()=>{});
 const zone=descendants(p.get('players')).find(n=>n.className==='zone zone-battlefield');
 assert.deepEqual(zone.children[1].children.map(n=>n.dataset.lane),['Lands','Permanents']);
});

test('preview uses the current live rules once instead of appending Oracle text', async()=>{
 const p=page();await p.settle();
 assert.equal(p.table.cardRules({rules:'Trample\n\nPay 3 life: Add {R}.',abilities:'Trample\r\n\r\nPay 3 life: Add {R}.'}),'Trample\n\nPay 3 life: Add {R}.');
 assert.equal(p.table.cardRules({rules:'Add {R}.',abilities:'Add {G}.\n\nGranted ability: Draw a card.'}),'Add {G}.\n\nGranted ability: Draw a card.');
 assert.equal(p.table.cardRules({abilities:'Defender\n\nDefender'}),'Defender');
});

test('offered library cards open their zone once, and never restore a scoped reveal during work', async()=>{
 const p=page();await p.settle();const zones=[{zone:'Library',count:90,cards:[{id:7,name:'Forest',type:'Basic Land',temporaryReveal:true}]}];
 const view={status:'choice',prompt:{id:'search',focusZones:[{player:1,zone:'Library'}]},players:[{id:1,name:'Human',life:40,zones}]};
 p.table.render(view,()=>{});assert.equal(p.get('zoneBrowser').hidden,false);assert.match(p.get('zoneBrowserTitle').textContent,/library/);
 p.get('closeZoneBrowser').onclick();p.table.render(view,()=>{});assert.equal(p.get('zoneBrowser').hidden,true);
 p.table.render({...view,prompt:{id:'search-next',focusZones:[{player:1,zone:'Library'}]}},()=>{});assert.equal(p.get('zoneBrowser').hidden,false);
 p.table.render(p.table.waitingBoard(view),()=>{},{working:true});assert.doesNotMatch(textOf(p.get('zoneBrowserCards')),/Forest/);
 p.table.reset();assert.equal(p.get('zoneBrowser').hidden,true);
 p.table.render({...view,prompt:{id:'hidden',focusZones:[{player:1,zone:'Library'}]},players:[{id:1,name:'Human',zones:[{zone:'Library',count:90,cards:[]}]}]},()=>{});assert.equal(p.get('zoneBrowser').hidden,true);
});

test('Egg token artwork lookup removes Forge suffix and preserves token characteristics', async()=>{
 const p=page();await p.settle();p.table.render({status:'input',players:[{id:1,name:'Human',zones:[{zone:'Battlefield',count:1,cards:[{id:7,name:'Egg Token',token:true,type:'Creature — Egg',pt:'1/2',tokenPT:'0/1'}]}]}]},()=>{});await p.settle();
 const query=p.urls.find(url=>url.startsWith('/cards/resolve'));assert.match(query,/q=Egg&/);assert.match(query,/token=1/);assert.match(query,/pt=0%2F1/);assert.match(textOf(p.get('players')),/Egg Token/);
});

test('saved printings survive deck selection, submission and artwork lookup', async()=>{
 const p=page();await p.settle();const printing='4e5c0438-e691-4c5f-9d04-1861f9588a09';
 p.savedDeck({commanders:[{name:'Isamaru, Hound of Konda',quantity:1,printing}],main:[{name:'Plains',quantity:99,printing}]});
 p.get('humanSaved').value='9';await p.get('humanSaved').onchange();await p.get('start').onclick();assert.equal(p.posts.at(-1).human.main[0].printing,printing);assert.equal(p.posts.at(-1).human.commanders[0].printing,printing);
 p.table.render({status:'input',players:[{id:1,name:'Human',zones:[{zone:'Hand',count:1,cards:[{id:7,name:'Plains',printing}]}]}]},()=>{});await p.settle();assert.ok(p.urls.some(url=>url.includes('printing='+printing)));
 p.get('humanSaved').value='';await p.get('humanSaved').onchange();await p.get('start').onclick();assert.equal(p.posts.at(-1).human.main[0].printing,undefined);
});


test('phase-stop preferences are scoped by owner and restored in a new worker', async () => {
  const storage = new Map(), p=page(storage);await p.settle();
  const all=['DRAW','MAIN1','END_OF_TURN'];
  p.update({revision:5,phaseStops:{Human:all,CPU:all}});await p.refresh();
  await descendants(p.get('players')).find(n=>n.dataset.stopSeat==='Human'&&n.dataset.stopPhase==='DRAW').onclick();
  assert.equal(JSON.parse(storage.get('manatomb.cpu.phaseStops.1')).Human.DRAW,false);
  assert.equal(p.posts.at(-1).action,'setPhaseStop');
  p.updateView({id:'second-game',state:{session:'worker-two',revision:1,status:'input',prompt:{id:'second-prompt',text:'Priority',ok:'OK',okEnabled:true},phaseStops:{Human:all,CPU:all},players:[]}});await p.refresh();
  const restore=p.posts.at(-1);assert.equal(restore.session,'worker-two');assert.equal(restore.seat,'Human');assert.equal(restore.phase,'DRAW');assert.equal(restore.enabled,false);
  const count=p.posts.length;await p.refresh();assert.equal(p.posts.length,count);
  assert.equal(p.get('prompt').dataset.prompt,'second-prompt');
  const other=page(new Map([['manatomb.cpu.phaseStops.99',JSON.stringify({Human:{DRAW:false}})]]));await other.settle();other.update({phaseStops:{Human:all,CPU:all}});await other.refresh();assert.equal(other.posts.length,0);
});

test('opponent hand has a reserve shortcut above graveyard with authorized reveals only', async () => {
  const p=page();await p.settle();
  const revealed={id:8,name:'Forest',type:'Basic Land — Forest',temporaryReveal:true};
  p.table.render({status:'input',players:[{id:2,name:'CPU',life:40,zones:[{zone:'Hand',count:4,cards:[revealed]},{zone:'Battlefield',count:0,cards:[]},{zone:'Graveyard',count:0,cards:[]}]}]},()=>{});
  const reserves=descendants(p.get('players')).find(n=>n.className==='reserves');
  assert.equal(reserves.children[0].dataset.zoneName,'Hand');assert.equal(reserves.children[1].dataset.zoneName,'Graveyard');
  reserves.children[0].onclick();assert.match(textOf(p.get('zoneBrowserCards')),/Forest/);
  assert.match(textOf(descendants(p.get('zoneBrowserCards')).find(n=>n.className==='zone-hidden')),/3.*Hidden cards/);
  assert.equal(descendants(p.get('players')).some(n=>n.className?.includes('zone-hand')),false);
});

test('player counters, modified stats and renamed phase labels appear on the board', async () => {
  const p=page();await p.settle();
  p.table.render({status:'input',phaseStops:{Human:['MAIN1']},players:[{id:1,name:'Human',life:40,counters:{poison:3,energy:2,experience:5},zones:[{zone:'Battlefield',count:1,cards:[{id:1,name:'Grizzly Bears',type:'Creature — Bear',pt:'4/4',statsModified:true,counters:[{name:'+1/+1',count:2}],keywords:['Trample'],temporaryReveal:false}]}]}]},()=>{});
  const nodes=descendants(p.get('players'));
  for(const [key,value] of [['poison',3],['energy',2],['experience',5]])assert.match(textOf(nodes.find(n=>n.dataset.counter===key)),new RegExp(String(value)));
  assert.match(textOf(nodes.find(n=>n.className==='card-ribbon')),/4\/4.*2 \+1\/\+1.*Trample/);
  assert.ok(nodes.some(n=>n.className==='stats-modified'));
  for(const [phase,label] of [['COMBAT_DECLARE_ATTACKERS','DA'],['COMBAT_DECLARE_BLOCKERS','DB'],['COMBAT_DAMAGE','CD'],['END_OF_TURN','ES']])assert.equal(nodes.find(n=>n.dataset.stopPhase===phase).textContent,label);
});

test('unsupported mobile screens never start polling or create a game', async()=>{
 const p=page(new Map(),true);await p.settle();assert.equal(p.urls.length,0);assert.equal(p.get('start').onclick,undefined);
});

test('hidden cards use the real card back without identity lookups; loading shows ManaTomb',async()=>{
 const p=page();await p.settle();p.table.reset();assert.equal(textOf(p.get('cardPicture')).trim(),'ManaTomb');
 p.table.render({status:'input',players:[{id:2,name:'CPU',life:40,zones:[{zone:'Hand',count:4,cards:[]},{zone:'Battlefield',count:1,cards:[{id:9,name:'Face-down card',type:'Creature',pt:'2/2'}]}]}]},()=>{});
 descendants(p.get('players')).find(n=>n.dataset.zoneName==='Hand').onclick();
 const backs=[...descendants(p.get('players')),...descendants(p.get('zoneBrowserCards'))].filter(n=>n.className==='card-back');
 assert.equal(backs.length,2);for(const back of backs){assert.equal(back.tag,'img');assert.equal(back.src,'https://cards.scryfall.io/back.png');}
 assert.equal(p.urls.some(url=>url.startsWith('/cards/resolve')),false);
});

test('matching basic lands form at most five-card piles while retaining every action ID',async()=>{
 const p=page();await p.settle();const sent=[];
 const cards=Array.from({length:7},(_,i)=>({id:i+1,name:'Forest',type:'Basic Land — Forest',printing:'same',temporaryReveal:false}));
 cards.push({...cards[0],id:8,tapped:true},{...cards[0],id:9,printing:'other'},{id:10,name:'Temple of the False God',type:'Land'});
 p.table.render({status:'input',players:[{id:1,name:'Human',life:40,zones:[{zone:'Battlefield',count:10,cards}]}]},(...args)=>sent.push(args));
 const nodes=descendants(p.get('players')),piles=nodes.filter(n=>n.className==='land-pile');
 assert.deepEqual(piles.map(n=>n.dataset.pileSize),['5','2','1','1']);assert.equal(piles[2].dataset.tapped,'true');
 for(const id of [1,2,3,4,5,6,7,8,9,10])nodes.find(n=>n.dataset.cardId===String(id)).onclick();
 assert.deepEqual(sent.map(row=>row[1].id),[1,2,3,4,5,6,7,8,9,10]);
});

test('damage shows remaining toughness in red without modifying actual preview stats',async()=>{
 const p=page();await p.settle();
 p.table.render({status:'input',players:[{id:1,name:'Human',life:40,zones:[{zone:'Battlefield',count:2,cards:[{id:1,name:'Grizzly Bears',type:'Creature — Bear',pt:'4/4',damage:2,statsModified:true},{id:2,name:'Colossal Dreadmaw',type:'Creature — Dinosaur',pt:'6/6',damage:7}]}]}]},()=>{});
 const nodes=descendants(p.get('players')),stats=nodes.filter(n=>n.className==='stats-damaged');
 assert.deepEqual(stats.map(n=>n.textContent),['4/2','6/-1']);assert.match(stats[0].title,/4\/4.*2 damage marked/);
 nodes.find(n=>n.className==='card-slot').onmouseenter();assert.match(textOf(p.get('cardDetail')),/4\/4/);assert.doesNotMatch(textOf(p.get('cardDetail')),/4\/2/);
});

test('commander tax is separate per commander and per seat; custom counter icons retain labels',async()=>{
 const p=page();await p.settle();
 p.table.render({status:'input',players:[{id:1,name:'Human',life:40,commanders:[{name:'Partner A',casts:1},{name:'Partner B',casts:3}],zones:[]},{id:2,name:'CPU',life:40,commanders:[{name:'Arahbo',casts:2}],zones:[]}]},()=>{});
 const nodes=descendants(p.get('players')),taxes=nodes.filter(n=>n.className==='mana-chip commander-tax');
 assert.deepEqual(taxes.map(n=>textOf(n).trim()),['TAX 4','TAX 2','TAX 6']);assert.match(taxes[2].title,/Partner B.*\+6/);
 for(const key of ['poison','experience']){const chip=nodes.find(n=>n.dataset.counter===key);assert.match(chip['aria-label'],new RegExp(key));assert.ok(chip.children.some(n=>n.tag==='svg'));}
});

test('stack target curves leave the item edge and meet the target edge with a correctly aligned arrow',async()=>{
 const p=page();await p.settle();
 const source={left:20,right:160,top:80,bottom:140,width:140,height:60};
 for(const top of [20,240]){
  const target={left:500,right:600,top,bottom:top+140,width:100,height:140};
  const g=p.table.connectionGeometry(source,target,true);
  assert.equal(g.start.x,163);assert.equal(g.start.y,110);
  assert.equal(g.end.x,496);assert.equal(g.end.y,top+70);
  assert.match(g.line,/^M163,110 C263,110 396,/);
  assert.match(g.arrow,new RegExp(`^M496,${top+70} L488,${top+74} L488,${top+66} Z$`));
 }
});

test('combat arrowheads point up for upward links and down for downward links',async()=>{
 const p=page();await p.settle();
 const upper={left:100,right:200,top:20,bottom:160,width:100,height:140},lower={...upper,top:300,bottom:440};
 const down=p.table.connectionGeometry(upper,lower),up=p.table.connectionGeometry(lower,upper);
 assert.equal(down.start.y,160);assert.equal(down.end.y,300);assert.equal(up.start.y,300);assert.equal(up.end.y,160);
 assert.match(down.arrow,/^M150,300 L146,292 L154,292 Z$/);
 assert.match(up.arrow,/^M150,160 L154,168 L146,168 Z$/);
});

test('random opponents use supported prebuilt decks and restore the manual selection', async () => {
  const p=page();await p.settle();
  p.get('humanCommanders').value='Atla Palani, Nest Tender';p.get('humanMain').value='99 Forest';
  p.get('cpuCommanders').value='Arahbo, Roar of the World';p.get('cpuMain').value='99 Plains';
  p.get('default').value='';p.get('default').onchange();assert.equal(p.get('customCPU').hidden,false);
  p.get('randomOpponent').checked=true;p.get('randomOpponent').onchange();
  assert.equal(p.get('default').disabled,true);assert.equal(p.get('customCPU').hidden,true);assert.equal(p.get('inspectDefault').hidden,true);
  for (const [random,expected] of [[0,'feline-ferocity'],[.2,'open-hostility'],[.4,'arcane-wizardry'],[.6,'draconic-domination'],[.99,'vampiric-bloodlust']]) {
    p.random(random);await p.get('start').onclick();const body=p.posts.at(-1);
    assert.equal(body.default,expected);assert.equal(body.cpu.main.length,0);assert.equal(body.human.commanders[0].name,'Atla Palani, Nest Tender');
  }
  p.get('randomOpponent').checked=false;p.get('randomOpponent').onchange();
  assert.equal(p.get('default').disabled,false);assert.equal(p.get('default').value,'');assert.equal(p.get('customCPU').hidden,false);
  await p.get('start').onclick();assert.equal(p.posts.at(-1).default,'');assert.equal(p.posts.at(-1).cpu.commanders[0].name,'Arahbo, Roar of the World');
  p.get('default').value='open-hostility';p.get('default').onchange();await p.get('start').onclick();assert.equal(p.posts.at(-1).default,'open-hostility');
});

test('random opponent preference survives reload without replacing the manual deck choice',async()=>{
  const setupStorage=new Map(),p=page(new Map(),false,setupStorage);await p.settle();
  p.get('humanCommanders').value='Atla Palani, Nest Tender';p.get('humanMain').value='99 Forest';
  p.get('default').value='draconic-domination';p.get('randomOpponent').checked=true;p.get('randomOpponent').onchange();
  await p.get('start').onclick();
  const next=page(new Map(),false,setupStorage);await next.settle();
  assert.equal(next.get('randomOpponent').checked,true);assert.equal(next.get('default').disabled,true);assert.equal(next.get('default').value,'draconic-domination');
  next.get('randomOpponent').checked=false;next.get('randomOpponent').onchange();await next.get('start').onclick();assert.equal(next.posts.at(-1).default,'draconic-domination');
});

test('starting without a deck opens the decklist inputs without creating a session',async()=>{
  const p=page();await p.settle();await p.get('start').onclick();
  assert.equal(p.posts.length,0);assert.equal(p.get('humanDecklist').open,true);assert.match(p.get('error').textContent,/Choose a saved deck or paste/);
});

test('opponent decklist toggles open and closed and a late response never reopens it',async()=>{
  const p=page();await p.settle();p.get('default').value='feline-ferocity';
  await p.get('inspectDefault').onclick();assert.equal(p.get('defaultList').hidden,false);assert.match(p.get('defaultList').textContent,/99 Forest/);assert.equal(p.get('inspectDefault')['aria-expanded'],'true');
  await p.get('inspectDefault').onclick();assert.equal(p.get('defaultList').hidden,true);assert.equal(p.get('inspectDefault').textContent,'View decklist');assert.equal(p.get('inspectDefault')['aria-expanded'],'false');
  let resolve;p.deferDefault(new Promise(r=>resolve=r));const loading=p.get('inspectDefault').onclick();
  await p.get('inspectDefault').onclick();resolve();await loading;assert.equal(p.get('defaultList').hidden,true);
  let resolveAgain;p.deferDefault(new Promise(r=>resolveAgain=r));const changing=p.get('inspectDefault').onclick();
  p.get('default').value='open-hostility';p.get('default').onchange();resolveAgain();await changing;assert.equal(p.get('defaultList').hidden,true);
});

test('editor handoff is selected, overrides an older setup, retains printings and survives reload',async()=>{
  const setupStorage=new Map();const printing='4e5c0438-e691-4c5f-9d04-1861f9588a09';
  const draft={name:'Unsaved egg deck',commanderName:'Atla Palani, Nest Tender',cards:{'Atla Palani, Nest Tender':1,Forest:99},cardMeta:{Forest:{printID:printing}}};
  // Use the real launcher, then the real receiving page against the same storage.
  const launcher={window:{},sessionStorage:{setItem:(key,value)=>setupStorage.set(key,value)},location:{href:'http://local.test/decks/9',assign:destination=>assert.equal(destination,'/cpu')},URL,alert:()=>assert.fail('unexpected launch failure')};
  vm.runInNewContext(fs.readFileSync(__dirname+'/cpu-launch.js','utf8'),launcher);
  setupStorage.set('manatomb.cpu.setup',JSON.stringify({owner:1,body:{human:{commanders:[{name:'Old commander',quantity:1}],main:[{name:'Plains',quantity:99}]},cpu:{commanders:[],main:[]},default:'feline-ferocity'}}));
  launcher.window.ManaTombCPU.launch(draft,1);
  const p=page(new Map(),false,setupStorage);await p.settle();
  assert.equal(p.get('humanSaved').value,'editor');assert.equal(p.get('humanSaved').children.find(o=>o.value==='editor').textContent,'Unsaved egg deck');
  assert.equal(p.get('humanCommanders').value,'Atla Palani, Nest Tender');assert.equal(p.get('humanMain').value,'99 Forest');assert.equal(setupStorage.has('manatomb.cpu.start'),false);assert.equal(p.posts.length,0);
  const next=page(new Map(),false,setupStorage);await next.settle();assert.equal(next.get('humanSaved').value,'editor');assert.equal(next.get('humanMain').value,'99 Forest');
  await next.get('start').onclick();const body=next.posts.at(-1);assert.equal(body.human.name,'Unsaved egg deck');assert.equal(body.human.main[0].printing,printing);
  next.get('humanSaved').value='';await next.get('humanSaved').onchange();assert.match(next.get('humanDecklistSummary').textContent,/Paste/);
  next.get('humanSaved').value='editor';await next.get('humanSaved').onchange();assert.equal(next.urls.includes('/api/cpu/decks/editor'),false);assert.equal(next.get('humanMain').value,'99 Forest');
});

test('editor handoffs belonging to another account are never loaded',async()=>{
  const storage=new Map([['manatomb.cpu.start',JSON.stringify({owner:999,deck:{name:'Private deck',commanders:[{name:'Private commander'}],main:[{name:'Private card',quantity:99}]}})]]);
  const p=page(new Map(),false,storage);await p.settle();assert.equal(p.get('humanCommanders').value,'');assert.equal(p.get('humanMain').value,'');assert.notEqual(p.get('humanSaved').value,'editor');
});

test('account deck launch loads its saved name and printings without an editor option',async()=>{
 const storage=new Map([['manatomb.cpu.setup',JSON.stringify({owner:1,body:{human:{name:'Old deck',commanders:[],main:[]},cpu:{commanders:[],main:[]}}})]]);
 const printing='4e5c0438-e691-4c5f-9d04-1861f9588a09';
 const p=page(new Map(),false,storage,{search:'?deck=9',decks:[{id:9,name:'Turn 2 Tifa Wins'}],deck:{name:'Turn 2 Tifa Wins',commanders:[{name:'Tifa Lockhart',quantity:1}],main:[{name:'Forest',quantity:99,printing}]}});
 await p.settle(); assert.equal(p.get('humanSaved').value,'9'); assert.equal(p.get('humanSaved').children.find(o=>Number(o.value)===9).textContent,'Turn 2 Tifa Wins');
 assert.equal(p.get('humanSaved').children.some(o=>o.value==='editor'),false);assert.equal(p.get('humanMain').value,'99 Forest');assert.equal(p.posts.length,0);
 await p.get('start').onclick(); const body=p.posts.at(-1);assert.equal(body.human.name,'Turn 2 Tifa Wins');assert.equal(body.human.main[0].printing,printing);
});
