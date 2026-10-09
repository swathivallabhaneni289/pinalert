// G: compact line. The header is F's byline, and the button becomes one wrapping line that holds the counts and the
// still current words together, with the chevron at the end.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';
  S3.variants = S3.variants || {};
  S3.variants.g = {
    key: 'g',
    tab: 'G: Compact line',
    redesign: true,
    header: 'byline',
    button: 'line',
    summary: 'F\'s byline header, with the button flowing the counts and the still current words as one wrapping line and a chevron at the end. It wraps to two lines when the width runs out, which on a phone is every time.',
    weaknesses: [
      'One line never happens on a phone. Measured: 0 of 5 live states at any popup width, on rows of 236px and 276px and on Activity rows of 188px and 228px; 2 of 5 on the 350px row of the two pane layout; 5 of 5 only on the Activity page at desktop width (508px).',
      'Where it does fit, rows in one list get different heights depending on how long the still current word is.',
      'Three kinds of text (16px counts, 14px words, a chevron) share one line.',
      'At a large text size the chevron moves under the text, right aligned, when less than 6.85em is left beside it (the 188px Activity row from about 145%, the 228px Activity row and the 236px feed row at 200%). That costs 24px of height, and it keeps every count whole.'
    ]
  };
}(window.S3));
