// current: the trust block as it is built today (trust.js, trust.css), kept so the owner can compare. Its markup comes
// from lib.js buildCurrentBlock, which copies trust.js createTrustBlock, and its look comes from the real trust.css.
window.S3 = window.S3 || {};
(function (S3) {
  'use strict';
  S3.variants = S3.variants || {};
  S3.variants.current = {
    key: 'current',
    tab: 'Current d4 (as built)',
    redesign: false,
    header: 'none',
    button: 'built',
    summary: 'The block as the owner saw it in Safari: the whole two row button underlines every word on hover, and the reporter words sit in its second row beside a small user icon. The only sign that it opens is that hover underline, so on a phone there is none.',
    weaknesses: [
      'Every word in the button underlines while the pointer is over it, and there is no cue at all without a pointer.',
      'The reporter words sit under the counts, a long way from the labels at the top of the report.',
      'The panel sentences are the long ones from trustcopy.go: 281 characters for the four lines of an Unconfirmed report.'
    ]
  };
}(window.S3));
