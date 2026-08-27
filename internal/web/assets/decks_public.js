(function () {
  "use strict";

  var sortField = document.querySelector("[data-public-decks-sort]");
  if (sortField && sortField.form) {
    sortField.addEventListener("change", function () {
      if (typeof sortField.form.requestSubmit === "function") {
        sortField.form.requestSubmit();
      } else {
        sortField.form.submit();
      }
    });
  }

  var colorGroup = document.querySelector("[data-public-decks-color-options]");
  if (!colorGroup) return;

  var inputs = Array.prototype.slice.call(
    colorGroup.querySelectorAll('input[name="color"]')
  );
  var colorlessInput = inputs.find(function (input) {
    return input.value === "C";
  });

  inputs.forEach(function (input) {
    input.addEventListener("change", function () {
      if (!input.checked) return;

      if (input === colorlessInput) {
        inputs.forEach(function (other) {
          if (other !== colorlessInput) other.checked = false;
        });
        return;
      }

      if (colorlessInput) colorlessInput.checked = false;
    });
  });
})();
