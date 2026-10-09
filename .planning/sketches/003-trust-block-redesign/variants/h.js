// H: beside the label. The reporter words are plain text directly after the state label on the same line (popup: in
// the label group; rows and Activity: on the line under the meta line). No chip, no icon, no underline.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';
  S3.variants = S3.variants || {};
  S3.variants.h = {
    key: 'h',
    tab: 'H: Beside the label',
    redesign: true,
    header: 'beside',
    button: 'rows',
    summary: 'The reporter words sit right after the Unconfirmed label on the same line, as plain regular text. They wrap under the label only when there is no room. The button keeps the two counts on row one and only the still current words on row two, with a chevron that turns when the panel opens. Added at the owner request after seeing E, F and G.',
    weaknesses: [
      'A report that is Live has no state label, so its reporter words sit alone on that line; the words and the label move together but the line is not always the same width.',
      'A long tag (Unreliable reporter) next to a long label can wrap to a second line on the narrowest surfaces.'
    ]
  };
}(window.S3));
