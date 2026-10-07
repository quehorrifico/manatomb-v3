'use strict';
window.ManaTombCPU = {
  launch(draft, owner) {
    const commander = String(draft.commanderName || '').trim();
    const deck = {name: String(draft.name || 'Commander deck'), commanders: commander ? [{name: commander, quantity: 1}] : [], main: Object.entries(draft.cards || {}).filter(([name]) => name !== commander).map(([name, quantity]) => ({name, quantity: Number(quantity)}))};
    // This is a copy. The builder's original draft, notes, sideboard and print
    // selections remain in their existing owner-scoped storage untouched.
    try {
      const returnURL = new URL(location.href); returnURL.searchParams.delete('reset');
      sessionStorage.setItem('manatomb.cpu.start', JSON.stringify({owner, deck}));
      sessionStorage.setItem('manatomb.cpu.return', JSON.stringify({owner,path:returnURL.pathname+returnURL.search}));
    }
    catch (_) { alert('This browser cannot retain the deck snapshot. Open CPU play and paste the deck list instead.'); return; }
    location.assign('/cpu');
  }
};
