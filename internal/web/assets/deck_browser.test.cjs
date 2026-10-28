const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const context={window:{}};
vm.runInNewContext(fs.readFileSync(__dirname+'/deck_browser.js','utf8'),context);
const core=context.window.ManatombDeckBrowser;
test('curated roles replace text guesses and never turn tagged cards into combo cards',()=>{
 const card=core.normalizeCard({name:'Lightning Bolt',typeLine:'Instant',oracleText:'Deals damage to any target.',roles:['removal']});
 assert.deepEqual(Array.from(core.categoryKeys(card)),['removal']);
 assert.equal(core.primaryCategory(card),'Removal');
 assert.equal(core.primaryCategory(core.normalizeCard({...card,primaryRole:'utility'})),'Utility');
});
test('multiple roles and a deck override survive editor and public adapters',()=>{
 const meta={name:'Land tutor',roles:['ramp','tutor'],primaryRole:'tutor',roleSource:'scryfall'};
 const editor=core.adaptEditorRow({cardName:meta.name,qty:2,cardMeta:meta});
 const publicCard=core.adaptPublicCard({CardName:meta.name,Quantity:2,Roles:meta.roles,RoleOverride:'tutor'});
 for(const card of [editor,publicCard]){assert.deepEqual(Array.from(core.categoryKeys(card)),['ramp','tutor']);assert.equal(core.primaryCategory(card),'Tutors');}
});
