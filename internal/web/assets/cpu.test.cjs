const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

function page() {
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
  let failure = '', timer, downloaded, expired=false; const posts=[]; 
  const sandbox = {
    document: {body: new Element(), getElementById: get, createElement: tag => new Element(tag), createElementNS: (_, tag) => new Element(tag), createTextNode: text => new Element('text', text), querySelectorAll: selector => selector === '[data-stop-phase]' ? descendants(get('players')).filter(e=>e.dataset.stopPhase) : selector.includes('#prompt button') ? get('prompt').children.filter(e=>e.tag==='button') : []},
    fetch: async (url, options) => {
      if(options.method==='POST')posts.push(JSON.parse(options.body));
      let body = url.endsWith('/config') ? {owner: 1} : url.endsWith('/decks') ? [] : view;
      const reject = expired || options.method === 'POST' && failure;
      if (reject) body = {error: failure};
      return {ok: !reject, status: expired ? 401 : reject ? 409 : 200, headers: {get: () => 'application/json'}, json: async () => body};
    },
    URL: {createObjectURL: blob => {downloaded = blob.parts.join(''); return 'blob:test';}, revokeObjectURL() {}}, Blob: class {constructor(parts){this.parts=parts;}},
    crypto: {randomUUID: () => 'request-one'}, setTimeout: (callback, delay) => { if (callback.name === 'refresh') { timer = callback; sandbox.pollDelay = delay; } }, clearTimeout() {},
    sessionStorage: {getItem: () => null}, location: {search: ''}, URLSearchParams,
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(__dirname + '/cpu-table.js', 'utf8'), sandbox);
  vm.runInContext(fs.readFileSync(__dirname + '/cpu.js', 'utf8'), sandbox);
  return {get, posts, pollDelay:()=>sandbox.pollDelay, table:sandbox.ManatombCPUTable, expire:()=>{expired=true;timer=null;}, nextPoll:()=>timer, updateView:patch=>Object.assign(view,patch), focused:()=>sandbox.document.activeElement, update: patch => Object.assign(view.state,patch), exported: () => downloaded, poison: () => {view.state.players=[{name:'Private hand name',zones:[]}];view.state.prompt.text='Hidden rule data';view.state.secret='private-token';view.error='private deck validation text';}, settle: () => new Promise(resolve => setImmediate(resolve)), refresh: () => timer(), savedRefresh:()=>timer, reject: message => {failure = message;}, end: status => {view = {...view, status};}};
}

test('a rejected reply remains visible after refreshing the unchanged prompt', async () => {
  const p = page(); await p.settle();
  p.reject('Assignment does not satisfy Forge combat damage order/lethal constraints');
  await p.get('prompt').children.find(e => e.tag === 'button' && e.textContent === 'OK').onclick();
  assert.match(p.get('error').textContent, /Forge combat damage/);
  assert.equal(p.get('prompt').dataset.prompt, 'prompt-one');
  assert.equal(p.get('prompt').children.find(e => e.tag === 'button').disabled, false);
});

test('terminal snapshots clear prompt identity and disable historical board controls', async () => {
  for (const status of ['finished', 'failed', 'expired', 'cancelled']) {
    const p = page(); await p.settle(); p.end(status); await p.refresh();
    assert.equal(p.get('prompt').hidden, true);
    assert.equal(p.get('prompt').dataset.prompt, undefined);
    assert.equal(p.get('prompt').children.length, 0);
    assert.equal(p.get('players').children[0].children[0].children[0].disabled, true);
    assert.equal(p.get('setup').hidden, false);
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


test('public activity is bounded, deduplicated and excludes private text', async () => {
  const p=page();await p.settle();p.poison();
  for(let turn=1;turn<=70;turn++){p.update({turn,phase:'MAIN1'});await p.refresh();}
  assert.equal(p.get('activity').children.length,50);
  await p.refresh();assert.equal(p.get('activity').children.length,50);
  const text=p.get('activity').children.map(e=>e.textContent).join('\n');
  assert.match(text,/turn 70/);assert.doesNotMatch(text,/turn 20(?:\D|$)/);
  for(const value of ['Private hand name','Hidden rule data','private-token','private deck validation text'])assert.equal(text.includes(value),false);
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
  assert.equal(p.get('prompt').hidden,true);
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
  assert.equal(chips.length,12);assert.equal(chips.filter(n=>n.tag==='button').length,0);
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
