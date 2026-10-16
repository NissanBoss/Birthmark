# Changelog

## v1

The first release.

Says whether the photos you are about to publish carry your camera's mark,
the pattern its sensor leaves in every photo, and what would take it out.

- **The fingerprint** of a camera from its own photos, by the published
  method: a Daubechies wavelet denoiser with the adaptive Wiener filter of
  Mihcak and others, the noise weighted by light, the row and column
  averages taken out of each of the colour filter's four grids, and the
  spectrum flattened. More than one camera can be given.
- **The search** in each photo to publish: every way round, at the scale
  the photo's size gives, over every crop that scale leaves room for, and
  scored by the PCE against a line raised for the number of places
  searched. The pattern is found in photos WhatsApp and Facebook have
  turned, shrunk and saved again.
- **What breaks it:** the photos that carry the mark are shrunk to six
  sizes and saved as JPEG in memory, searched again, and the size where the
  mark went is reported.
- **Calibrated on nineteen real phones** from VISION, in their own photos
  and as WhatsApp and Facebook gave them back, with the pairs of phones of
  one model kept apart. The calibration runs every week on GitHub's
  machines.
- **It only reads:** nothing is written, nothing is sent, no other program
  is run, and there is no model in it, with tests that read the source to
  keep it that way. One photo is compared at a time under a memory ceiling.
  HEIC, raw, WebP and video are listed as not read.
