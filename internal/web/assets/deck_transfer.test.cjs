const test = require('node:test');
const assert = require('node:assert/strict');
const { formatExport, exportRows } = require('./deck_transfer.js');
const deck = {
  name: 'Export test', commanderName: 'Commander',
  cards: { Commander: 1, 'Bolt': 4, 'Comma, Card': 2, 'Fourth': 1 },
  sideboardCards: { Bolt: 2 }, maybeCards: { Maybe: 3 },
  cardMeta: { Commander: { printID: 'commander-id' }, Bolt: { printID: 'main-id', setCode: 'LEA', collectorNumber: '1' } },
  boardCardMeta: { side: { Bolt: { printID: 'side-id', setCode: 'M10', collectorNumber: '2' } } }
};
const all = { quantity: true, set: true, collector: true, uuid: true, sections: true };
test('exports every board and the commander once, preserving each board printing', () => {
  const text = formatExport(deck, 'text', all);
  assert.equal((text.match(/1 Commander/g) || []).length, 1);
  assert.match(text, /Mainboard:\n4 Bolt \(LEA\) 1 \{scryfall:main-id\}/);
  assert.match(text, /Sideboard:\n2 Bolt \(M10\) 2 \{scryfall:side-id\}/);
  assert.match(text, /Maybeboard:\n3 Maybe/);
  assert.equal(exportRows(deck).reduce((n, row) => n + row.qty, 0), 13);
});
test('all 32 option combinations affect text and CSV independently', () => {
  for (let mask = 0; mask < 32; mask++) {
    const opts = Object.fromEntries(Object.keys(all).map((key, i) => [key, !!(mask & (1 << i))]));
    const text = formatExport(deck, 'text', opts);
    const csv = formatExport(deck, 'csv', opts);
    assert.equal(text.includes('4 Bolt'), opts.quantity);
    assert.equal(text.includes('(LEA)'), opts.set);
    assert.equal(text.includes('{scryfall:'), opts.uuid);
    assert.equal(text.includes('Mainboard:'), opts.sections);
    assert.equal(csv.split('\n')[0].includes('Quantity'), opts.quantity);
    assert.equal(csv.split('\n')[0].includes('Set'), opts.set);
    assert.equal(csv.split('\n')[0].includes('Collector Number'), opts.collector);
    assert.equal(csv.split('\n')[0].includes('Scryfall UUID'), opts.uuid);
    assert.equal(csv.split('\n')[0].includes('Section'), opts.sections);
  }
});
test('preview uses exactly the first three card rows of the full export', () => {
  for (const type of ['text', 'csv', 'clipboard']) {
    const preview = formatExport(deck, type, all, 3);
    assert.ok(formatExport(deck, type, all).startsWith(preview));
    assert.ok(!preview.includes('Fourth'));
    assert.ok(!preview.includes('Sideboard'));
  }
});
test('CSV escapes card names and empty decks have no misleading export', () => {
  assert.match(formatExport(deck, 'csv', all), /"Comma, Card"/);
  assert.equal(formatExport({}, 'text', all), '');
  assert.equal(formatExport({}, 'csv', all), '');
  assert.equal(formatExport(deck, 'clipboard', all), formatExport(deck, 'text', all));
});
test('public boards use the same export formatter and cost counts selected printings', () => {
  const { fromBoards, deckCost } = require('./deck_transfer.js');
  const boards = { main: [{name:'Alpha',quantity:2,price:'1.25',setCode:'TST',printID:'selected'}], side:[{name:'Beta',quantity:1,price:''}], maybe:[{name:'Maybe',quantity:10,price:'100'}] };
  const state=fromBoards(boards,{name:'Public deck'});
  assert.match(formatExport(state,'text',all), /2 Alpha \(TST\) \{scryfall:selected\}/);
  assert.deepEqual(deckCost(state), {display:'$2.50',unpriced:1});
});
test('cheapest printing application keeps commander and board versions consistent without changing roles',()=>{
 const {applyCheapestPrintings}=require('./deck_transfer.js');
 const original={commanderName:'Leader',commanderPrintID:'old',cards:{Leader:2,Other:1},cardMeta:{Leader:{cardID:'leader',printID:'old',priceUSD:'10',roles:['ramp'],primaryRole:'utility'},Other:{cardID:'other',printID:'tie',priceUSD:'1'}},boardCardMeta:{main:{Leader:{cardID:'leader',printID:'old',priceUSD:'10'}}}};
 const next=applyCheapestPrintings(original,{leader:{cardID:'leader',printID:'cheap',priceUSD:'2'},other:{cardID:'other',printID:'different',priceUSD:'1'}});
 assert.equal(original.commanderPrintID,'old');
 assert.equal(next.commanderPrintID,'cheap');
 assert.equal(next.cardMeta.Leader.printID,'cheap');
 assert.equal(next.boardCardMeta.main.Leader.printID,'cheap');
 assert.equal(next.cardMeta.Leader.primaryRole,'utility');
 assert.equal(next.cardMeta.Other.printID,'tie');
 const protectedCommander=applyCheapestPrintings({...original,cardMeta:{...original.cardMeta,Leader:{cardID:'leader',printID:'already-cheaper',priceUSD:'0.50'}}},{leader:{cardID:'leader',printID:'cheap',priceUSD:'2'}});
 assert.equal(protectedCommander.cardMeta.Leader.printID,'already-cheaper');
 assert.equal(protectedCommander.boardCardMeta.main.Leader.printID,'cheap');
 assert.equal(applyCheapestPrintings(original,{leader:{cardID:'wrong-oracle',printID:'wrong',priceUSD:'0.01'}}).commanderPrintID,'old');
});
