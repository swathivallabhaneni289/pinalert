// F: header byline. The reporter words are a quiet second line of the header, directly under the labels and left aligned,
// then the title. The button is the same two rows as E.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';
  S3.variants = S3.variants || {};
  S3.variants.f = {
    key: 'f',
    tab: 'F: Header byline',
    redesign: true,
    header: 'byline',
    button: 'rows',
    summary: 'The reporter words are a plain second line under the labels, left aligned with the title, so they sit in the same place on every report and every surface. The button is the same two rows as E, with the chevron.',
    weaknesses: [
      'It costs a line. Measured with the panel closed: between 4px shorter and 27px taller than today in the popup at 288px, and between 5px shorter and 23px taller on a row at 236px.',
      'On a 288px popup with an Unconfirmed label the top of the card is three lines (labels, the wrapped label, the words) before the title.',
      'Two rows of trust text stay in the button on a live report.',
      'At a large text size the chevron moves under the text, right aligned, when less than 6.85em is left beside it (the 188px Activity row from about 145%, the 228px Activity row and the 236px feed row at 200%). That costs 24px of height, and it keeps every count whole.'
    ]
  };
}(window.S3));
