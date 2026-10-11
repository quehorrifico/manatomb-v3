'use strict';
window.ManaTombCPU = {
  async launch(draft, owner, saveAccountDeck) {
    if (Number(draft.id) > 0) {
      if (typeof saveAccountDeck !== 'function') throw new Error('Save this deck before starting a playtest.');
      const saved = await saveAccountDeck();
      if (!Number.isSafeInteger(Number(saved?.id)) || Number(saved.id) <= 0) throw new Error('The deck could not be saved. Please try again.');
      const returnURL = new URL(location.href); returnURL.searchParams.delete('reset');
      try {
        sessionStorage.removeItem('manatomb.cpu.start');
        sessionStorage.setItem('manatomb.cpu.return', JSON.stringify({owner, path:returnURL.pathname + returnURL.search}));
      } catch (_) { /* Saved decks can still be loaded directly from the account. */ }
      location.assign('/cpu?deck=' + Number(saved.id));
      return;
    }
    const commander = String(draft.commanderName || '').trim();
    const deck = {name: String(draft.name || 'Commander deck'), commanders: commander ? [{name: commander, quantity: 1}] : [], main: Object.entries(draft.cards || {}).filter(([name]) => name !== commander).map(([name, quantity]) => ({name, quantity: Number(quantity)}))};
    for (const card of [...deck.commanders, ...deck.main]) {
      const meta = draft.cardMeta?.[card.name] || {};
      const printing = card.name === commander ? draft.commanderPrintID || meta.preferredPrintID || meta.printID || meta.scryfallID : meta.preferredPrintID || meta.printID || meta.scryfallID;
      if (printing) card.printing = printing;
    }
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
