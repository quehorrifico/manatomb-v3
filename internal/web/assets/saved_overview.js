// Account metadata autosave. Only a server acknowledgment means saved.
(function (root) {
  'use strict';
  function create(options) {
    var pending = null, revision = 0, active = null, timer = null, deadline = null, durable = false;
    function fields(value) {
      var out = { description: String(value.description || ''), tags: String(value.tags || ''), format: String(value.format || 'Sandbox') };
      if (Object.prototype.hasOwnProperty.call(value, 'name')) out.name = String(value.name || '');
      return out;
    }
    var baseline = fields(options.initial || {});
    function status(state, message) { options.status(state, message); }
    function clearTimers() {
      if (timer !== null) options.clearTimeout(timer);
      if (deadline !== null) options.clearTimeout(deadline);
      timer = deadline = null;
    }
    function journal() {
      durable = false;
      try {
        var text = JSON.stringify({ version: 1, fields: pending });
        options.storage.setItem(options.key, text);
        durable = options.storage.getItem(options.key) === text;
      } catch (_) { /* Never claim recoverability without verified storage. */ }
    }
    function waiting(message) {
      status(durable ? 'unsaved' : 'error', durable ? message : 'Not saved. Recovery storage is unavailable; keep this page open and copy your changes before leaving.');
    }
    function overlay(state) { return pending ? Object.assign({}, state, pending) : state; }
    function schedule() {
      if (timer !== null) options.clearTimeout(timer);
      timer = options.setTimeout(flush, 180);
      // Continuous typing cannot postpone the account save indefinitely.
      if (deadline === null) deadline = options.setTimeout(flush, 2000);
    }
    function flush() {
      clearTimers();
      if (active) return active.then(function (ok) { return ok && pending ? flush() : ok; });
      if (!pending) return Promise.resolve(true);
      var sentRevision = revision, sent = fields(pending);
      status('saving', 'Saving to your account…');
      active = Promise.resolve().then(function () { return options.send(sent); }).then(function (result) {
        baseline = fields(Object.assign({}, baseline, result || sent));
        if (revision === sentRevision) {
          pending = null;
          try { options.storage.removeItem(options.key); } catch (_) { /* Server acknowledgment still stands. */ }
          status('saved', 'Saved to your account.');
        }
        options.render(overlay(result));
        return true;
      }).catch(function (error) {
        clearTimers();
        waiting('Not saved to your account. Your changes are recoverable on this device. Reconnecting…');
        // Validation/auth failures need user action; network and server failures retry.
        if (!error || error.retryable !== false) timer = options.setTimeout(flush, 5000);
        return false;
      }).then(function (ok) {
        active = null;
        if (ok && pending) { schedule(); return flush(); }
        return ok;
      });
      return active;
    }
    function update(value) {
      pending = fields(Object.assign({}, baseline, pending || {}, value)); revision++;
      journal();
      waiting('Unsaved changes. A recovery copy is stored on this device.');
      schedule();
    }
    try {
      var restored = JSON.parse(options.storage.getItem(options.key) || 'null');
      if (restored && restored.version === 1 && restored.fields && typeof restored.fields === 'object') {
        pending = fields(Object.assign({}, baseline, restored.fields)); revision++;
        journal();
        waiting('Recovered unsaved changes. Saving to your account…');
        schedule();
      }
    } catch (_) { /* New edits detect storage failure. */ }
    return { update: update, flush: flush, overlay: overlay,
      adopt: function (state) { baseline = fields(Object.assign({}, baseline, state)); },
      hasPending: function () { return pending !== null; } };
  }
  root.ManaTombSavedOverview = { create: create };
  if (typeof module !== 'undefined' && module.exports) module.exports = root.ManaTombSavedOverview;
})(typeof window !== 'undefined' ? window : globalThis);
