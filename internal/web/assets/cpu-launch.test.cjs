const test=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
test('CPU launch copies main/commander only, scopes snapshot to owner, preserves original draft',()=>{
 const storage=new Map();let destination;const context={window:{},sessionStorage:{setItem:(k,v)=>storage.set(k,v)},location:{href:'http://local.test/decks/new/workbench?format=Commander&reset=1',assign:v=>destination=v},URL,alert:()=>assert.fail('unexpected storage error')};vm.runInNewContext(fs.readFileSync(__dirname+'/cpu-launch.js','utf8'),context);
 const draft={name:'Unsaved deck',commanderName:'Isamaru, Hound of Konda',commanderPrintID:'6ab46b84-5411-4c8a-9292-052fa17ca8dc',description:'Keep my note',cards:{'Isamaru, Hound of Konda':1,Plains:99},sideboardCards:{Mountain:10},maybeCards:{Swamp:2},cardMeta:{Plains:{printID:'4e5c0438-e691-4c5f-9d04-1861f9588a09'}}};const before=JSON.stringify(draft);
 context.window.ManaTombCPU.launch(draft,42);assert.equal(JSON.stringify(draft),before);assert.equal(destination,'/cpu');const saved=JSON.parse(storage.get('manatomb.cpu.start'));assert.equal(saved.owner,42);assert.deepEqual(saved.deck.main,[{name:'Plains',quantity:99,printing:'4e5c0438-e691-4c5f-9d04-1861f9588a09'}]);assert.deepEqual(saved.deck.commanders,[{name:'Isamaru, Hound of Konda',quantity:1,printing:'6ab46b84-5411-4c8a-9292-052fa17ca8dc'}]);assert.equal(saved.deck.sideboard,undefined);assert.deepEqual(JSON.parse(storage.get('manatomb.cpu.return')),{owner:42,path:'/decks/new/workbench?format=Commander'});
});

function launcher() {
 const storage=new Map([['manatomb.cpu.start','stale draft']]);const destinations=[];
 const context={window:{},sessionStorage:{setItem:(k,v)=>storage.set(k,v),removeItem:k=>storage.delete(k)},location:{href:'http://local.test/decks/9',assign:v=>destinations.push(v)},URL};
 vm.runInNewContext(fs.readFileSync(__dirname+'/cpu-launch.js','utf8'),context);return {context,storage,destinations};
}
test('saved CPU launch waits for save acknowledgment and loads the existing account deck',async()=>{
 const f=launcher();let finish; const saving=new Promise(resolve=>finish=resolve);let calls=0;
 const pending=f.context.window.ManaTombCPU.launch({id:9,name:'Turn 2 Tifa Wins'},42,()=>{calls++;return saving;});
 assert.equal(calls,1);assert.equal(f.destinations.length,0);
 finish({id:9,name:'Turn 2 Tifa Wins'});await pending;
 assert.deepEqual(f.destinations,['/cpu?deck=9']);assert.equal(f.storage.has('manatomb.cpu.start'),false);
 assert.deepEqual(JSON.parse(f.storage.get('manatomb.cpu.return')),{owner:42,path:'/decks/9'});
});
test('failed account save leaves the player in the editor',async()=>{
 const f=launcher();await assert.rejects(f.context.window.ManaTombCPU.launch({id:9},42,async()=>{throw Error('offline');}),/offline/);
 assert.equal(f.destinations.length,0);
 await assert.rejects(f.context.window.ManaTombCPU.launch({id:9},42),/Save this deck/);
});
function editorSaveHook(overview, tail=Promise.resolve(), mutationError=null) {
 const source=fs.readFileSync(__dirname+'/../templates/deck_show_guest_draft_script.html.tmpl','utf8');
 const hook=source.slice(source.indexOf('window.mtDeckSaveForCPU = async function () {'),source.indexOf("var saveDeckButton = document.getElementById('deck-save');"));
 const context={window:{},deckMenuNameInput:{value:'Turn 2 Tifa Wins'},normalizeName:s=>s.trim(),saveSavedDeckName:()=>overview.flush(),savedOverview:overview,savedMutationTail:tail,savedMutationError:mutationError,savedDraft:{id:9,name:'Turn 2 Tifa Wins'}};
 vm.runInNewContext(hook,context);return context.window.mtDeckSaveForCPU;
}
test('editor save hook waits for queued card writes and rejects unacknowledged metadata',async()=>{
 const {create}=require('./saved_overview.js');let fail=true;
 const overview=create({key:'test',initial:{name:'Turn 2 Tifa Wins'},storage:{getItem:()=>null,setItem(){},removeItem(){}},setTimeout:()=>1,clearTimeout(){},status(){},render(){},send:async fields=>{if(fail)throw Error('offline');return fields;}});
 overview.update({description:'latest notes'});
 const f=launcher(); await assert.rejects(f.context.window.ManaTombCPU.launch({id:9},42,editorSaveHook(overview)),/Could not save/);assert.equal(f.destinations.length,0);
 fail=false;let finish;const writes=new Promise(resolve=>finish=resolve);
 const pending=f.context.window.ManaTombCPU.launch({id:9},42,editorSaveHook(overview,writes));await new Promise(setImmediate);assert.equal(f.destinations.length,0);
 finish();await pending;assert.deepEqual(f.destinations,['/cpu?deck=9']);assert.equal(overview.hasPending(),false);
});
