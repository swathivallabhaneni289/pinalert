// E: pill row words. The top row stays as it is today (badge, severity label, state label) and the reporter words join
// it as plain text at the far end. When they do not fit they drop under the labels, left aligned.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';
  S3.variants = S3.variants || {};
  S3.variants.e = {
    key: 'e',
    tab: 'E: Pill row words',
    redesign: true,
    header: 'far',
    button: 'rows',
    summary: 'The reporter words sit at the far end of the label row in plain regular text, before the close button in the popup, and drop under the labels, left aligned, when they do not fit. The button keeps the two counts on row one and only the still current words on row two, with a chevron that turns when the panel opens.',
    weaknesses: [
      'Where the words sit depends on room. Measured: at the far end in 8 of 8 row states at 236px, but in only 1 to 2 of 8 popup states and 4 of 8 Activity states at 188px, so one report looks different on different pages.',
      'An Unconfirmed label, or a long tag with its popup number, pushes the words under the labels, so the layout has two looks.',
      'On a wide desktop row the words sit at the far right edge, hundreds of pixels from the label (about 360px on the 640px Activity page). In its favour, rows are the most compact: up to 28px shorter than today at 236px.',
      'Where the words do reach the far end of the popup they stop 16px short of the close button box at a 360px phone (about 30px short of the X mark), so the words and the X can read as one unit, "New reporter x", which is close to the tag look the owner rejected.',
      'Not the recommendation. If the far end placement is liked on the feed list, the option is E on feed rows only and F\'s byline on the popup and the Activity page: one rule per surface (see the README).',
      'At a large text size the chevron moves under the text, right aligned, when less than 6.85em is left beside it (the 188px Activity row from about 145%, the 228px Activity row and the 236px feed row at 200%). That costs 24px of height, and it keeps every count whole.'
    ]
  };
}(window.S3));
