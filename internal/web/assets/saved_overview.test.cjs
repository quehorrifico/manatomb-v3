const test = require('node:test');
const assert = require('node:assert/strict');
const { create } = require('./saved_overview.js');
function fixture(storage = new Map(), send = async fields => fields) {
  const states = [], renders = [], timers = new Map(); let seq = 0;
  const adapter = storage instanceof Map ? {
    getItem: k => storage.get(k), setItem: (k, v) => storage.set(k, v), removeItem: k => storage.delete(k)
  } : storage;
  const model = create({ key: 'owner:deck', storage: adapter, send,
    status: (state, message) => states.push({ state, message }), render: value => renders.push(value),
    setTimeout: fn => { timers.set(++seq, fn); return seq; }, clearTimeout: id => timers.delete(id) });
  return { model, storage, states, renders, timers };
}
function deferred() { let resolve, reject; const promise = new Promise((a,b) => { resolve=a; reject=b; }); return { promise, resolve, reject }; }
test('failed save survives immediate navigation/reload and retries only with acknowledgment', async () => {
  const f = fixture(new Map(), async () => { throw Error('offline'); });
  f.model.update({ description: 'never lose this edit', format: 'Commander', tags: 'Aggro' });
  assert.equal(f.states.at(-1).state, 'unsaved');
  assert.ok(f.storage.get('owner:deck').includes('never lose this edit'));
  await f.model.flush();
  assert.equal(f.model.hasPending(), true);
  assert.match(f.states.at(-1).message, /Not saved to your account/);
  const reloaded = fixture(f.storage);
  assert.equal(reloaded.model.overlay({ description: 'old server note' }).description, 'never lose this edit');
  assert.equal(reloaded.timers.size, 2, 'recovery autosaves with a bounded deadline');
  await reloaded.model.flush();
  assert.equal(reloaded.states.at(-1).state, 'saved');
  assert.equal(f.storage.size, 0);
});
test('slow acknowledgment cannot overwrite a newer edit and submits only one request at a time', async () => {
  const first = deferred(); const sends = [];
  const f = fixture(new Map(), fields => { sends.push(fields); return sends.length === 1 ? first.promise : Promise.resolve(fields); });
  f.model.update({description:'old'}); const saving = f.model.flush();
  assert.equal(f.states.at(-1).state, 'saving');
  f.model.update({description:'new'}); const secondSave = f.model.flush(); await Promise.resolve();
  assert.equal(sends.length, 1);
  assert.equal(f.model.overlay({description:'unrelated stale mutation'}).description, 'new');
  first.resolve({description:'old'}); await saving; await secondSave;
  assert.ok(f.renders.every(value => value.description === 'new'));
  await f.model.flush(); assert.equal(sends.length,2);
  assert.equal(sends[1].description,'new'); assert.equal(f.states.at(-1).state,'saved');
});
test('storage denial is visible and retains in-memory changes without claiming recoverability', async () => {
  const f = fixture({getItem(){throw Error('denied');},setItem(){throw Error('quota');}} ,async()=>{throw Error('offline');});
  f.model.update({description:'copy before leaving'}); await f.model.flush();
  assert.equal(f.states.at(-1).state,'error');
  assert.match(f.states.at(-1).message,/copy your changes before leaving/);
  assert.equal(f.model.overlay({}).description,'copy before leaving');
});
test('rapid edits coalesce transport while every edit is journaled synchronously', () => {
  const f=fixture();
  for(let i=0;i<20;i++)f.model.update({description:String(i)});
  assert.equal(f.timers.size,2); assert.match(f.storage.get('owner:deck'),/"description":"19"/);
});
test('offline saves automatically retry while keeping the recovery journal', async () => {
  let attempts=0;
  const f=fixture(new Map(), async fields => { if (++attempts===1) throw Error('offline'); return fields; });
  f.model.update({name:'Recovery Deck',description:'notes'});
  assert.equal(await f.model.flush(),false);
  assert.equal(f.storage.size,1);
  assert.equal(f.timers.size,1);
  await [...f.timers.values()][0]();
  assert.equal(attempts,2); assert.equal(f.storage.size,0);
  assert.equal(f.states.at(-1).state,'saved');
});
test('validation and expired sessions retain recovery without repeated invalid requests', async () => {
  const error=Object.assign(Error('expired'),{retryable:false});
  const f=fixture(new Map(),async()=>{throw error;});
  f.model.update({description:'keep this'}); await f.model.flush();
  assert.equal(f.timers.size,0); assert.equal(f.storage.size,1);
});
test('partial name and note edits preserve the last acknowledged metadata', async () => {
  const f=fixture();
  f.model.adopt({name:'Original',description:'original notes',tags:'Aggro',format:'Commander'});
  f.model.update({name:'Renamed'}); await f.model.flush();
  assert.deepEqual(f.renders.at(-1),{name:'Renamed',description:'original notes',tags:'Aggro',format:'Commander'});
  f.model.update({description:'new notes'}); await f.model.flush();
  f.model.update({name:'Renamed again'}); await f.model.flush();
  assert.equal(f.renders.at(-1).description,'new notes');
  assert.equal(f.renders.at(-1).name,'Renamed again');
});
test('continuous typing still saves on the original deadline', async () => {
  const f=fixture();
  f.model.update({description:'first'});
  const deadline=[...f.timers.values()][1];
  for(let i=0;i<100;i++) f.model.update({description:'note '+i});
  assert.ok([...f.timers.values()].includes(deadline));
  await deadline();
  assert.equal(f.renders.at(-1).description,'note 99');
  assert.equal(f.model.hasPending(),false);
});
