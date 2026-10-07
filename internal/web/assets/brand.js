(function () {
  'use strict';
  var icon = document.querySelector('link[rel="icon"]');
  if (!icon) return;
  var source = '';
  function applyTheme() {
    var styles = getComputedStyle(document.documentElement);
    var accent = styles.getPropertyValue('--mt-accent').trim();
    var background = styles.getPropertyValue('--mt-bg').trim();
    var meta = document.querySelector('meta[name="theme-color"]');
    if (meta && background) meta.content = background;
    if (source && accent) icon.href = 'data:image/svg+xml,' + encodeURIComponent(source.replace('fill="currentColor"', 'fill="' + accent + '"'));
  }
  fetch(icon.getAttribute('href'), { credentials: 'same-origin' }).then(function (response) {
    if (!response.ok) throw new Error('Logo unavailable');
    return response.text();
  }).then(function (svg) { source = svg; applyTheme(); }).catch(function () { /* The SVG fallback remains available. */ });
  new MutationObserver(applyTheme).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
})();
