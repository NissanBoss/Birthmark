# Birthmark

Whether the photos you are about to publish carry your camera's mark: the
pattern its sensor leaves in every photo it takes, which no metadata tool
touches, and which links a photo to the phone that took it.

```
EXAMPLE
```

That is a real run, on real photos. The camera is an iPhone 4s from VISION,
the University of Florence's set of photos from thirty five phones; its
fingerprint comes from thirty of its photos of the sky and of walls. Of the
four photos to publish, three were taken by that iPhone: one as the camera
wrote it, one after WhatsApp had it, one after Facebook had it. The fourth
was taken by a Samsung.

## The problem

You strip the EXIF before you post a photo you do not want tied to you,
or the app does it for you. The GPS is gone, the phone's model is gone, the
serial number is gone. The photo still says which phone took it.

Every pixel of a camera sensor turns light into charge a little more or a
little less eagerly than the pixels around it. The differences are
fractions of a per cent, they come from the factory, and they never
change. Every photo the sensor takes is multiplied by that pattern: a
photo of the sky is the sky times the sensor. Take the picture out of a
photo with a denoiser and the pattern is in what is left, weak in one
photo and unmistakable once twenty photos are averaged into a fingerprint.

This is not new. Lukáš, Fridrich and Goljan described it in 2006, and it
is how forensic labs tie a photo to a camera. Goljan, Fridrich and Filler
tested it on a million photos from Flickr in 2009: at the line they drew,
about two false alarms in a hundred thousand. It survives JPEG, and it
survives the apps: shrinking a photo shrinks the pattern with it, and
cropping keeps the part of the pattern that was there.

So anybody with a handful of photos you have published under your name,
from wherever you post them at full size, can take any photo posted under
another name and ask whether your phone took it. A source who sends a
journalist a photo has sent the sensor along with it. An account you keep
apart from your own is one shared camera away from not being apart.

Metadata tools cannot help, because none of this is metadata. It is the
picture.

## What already exists

- [The Binghamton camera fingerprint code](https://dde.binghamton.edu/download/camera_fingerprint/)
  is the reference implementation, in Matlab and Python, from the people
  who wrote the method. It answers the forensic question, was this photo
  taken by this camera, for an analyst who has the camera.
- [prnu-python](https://github.com/polimi-ispl/prnu-python) from the
  Politecnico di Milano is a Python library of the same, for research.
- [sim-pez/prnu](https://github.com/sim-pez/prnu) compares denoisers for
  extracting the fingerprint.
- [Camera-Fingerprint-PRNU](https://github.com/ernestbies/Camera-Fingerprint-PRNU)
  is a module for the Autopsy forensic suite, last changed in 2020.
- [ExifTool](https://exiftool.org) and
  [mat2](https://0xacab.org/jvoisin/mat2) take the metadata out of a photo,
  and [Baggage](https://github.com/NissanBoss/Baggage) shows it before you
  send the file. None of them can see the sensor, because the sensor is not
  in the metadata.

Birthmark asks the other question: not whether a camera took a photo, but
whether you can be found by your camera. It looks for the mark the way
somebody looking for you would, in every crop and at every size the
photo's own size suggests, and when it finds it, it tries smaller and
smaller copies to tell you what would have taken it out. One program, no
Python, no Matlab, nothing to install.

## How it works

Give it a folder of photos straight from your camera, the files it wrote,
and the photos you mean to publish:

```bash
birthmark --camera ~/DCIM/Camera ~/to-post
```

**The fingerprint.** The camera's photos of the size it writes are each
taken through a wavelet denoiser, and what the denoiser leaves is averaged,
weighted by how bright each pixel was, because the pattern is a gain and
shows only where there is light. What every camera of a kind shares is
taken out: the averages of every row and column, which carry the readout
and the colour filter, and the peaks in the spectrum, which carry JPEG's
blocks. What is left is the sensor's own. This is the method of the
reference implementation, step by step: Daubechies wavelets with eight
taps, four levels, the adaptive Wiener filter of Mihcak, Kozintsev and
Ramchandran, and the same filter again in the Fourier domain.

**The search.** A photo to publish is denoised the same way and correlated
with the fingerprint at every shift, and the score is the PCE, the peak to
correlation energy: how far the best match stands above everything else.
Before that, the fingerprint is given whatever was done to the photo:
turned every way round, shrunk to the scale the photo's size gives, and
slid over every crop that scale leaves room for. WhatsApp turns a photo
upright and shrinks it; Instagram crops a tall one to four by five first;
the search covers both. The line is a PCE of 60, raised by twice
the logarithm of the number of places searched, because the more places a
peak may sit, the higher one of them can stand by chance.

**What breaks it.** For every photo that carries the mark, copies are
shrunk to 2048, 1600, 1280, 1080, 800 and 640 on the long side, saved as
JPEG at quality 80, and searched again, in memory. The report says at what
size the mark went.

The wavelet transform, the Fourier transform, the denoiser and the
correlation are all written here, in Go, with nothing taken from anywhere
else.

## The calibration

CALIBRATION

## Running it

Download the package for your system from the
[Releases page](https://github.com/NissanBoss/Birthmark/releases), unpack
it and run the binary. There is nothing to install.

```bash
birthmark --camera ~/DCIM/Camera ~/to-post
```

```bash
birthmark --camera ~/phone --camera ~/old-phone photo.jpg
```

The camera's folder wants ten photos or more, straight from the camera,
not edited and not sent through anything: photos of bright, plain things
like the sky or a wall make the cleanest fingerprints, but any photos will
do with enough of them. Photos of another size in the folder are left out.
`--reveal` shows the home folder in paths instead of `~`.

Exit code is 1 when a photo carries the mark, 3 when nothing could be
compared, and 0 otherwise.

One photo is compared at a time, under a memory ceiling of two gigabytes:
a photo of twelve megapixels needs most of a gigabyte while it is
compared. A folder of thirty such photos takes a minute or two.

Windows marks anything that came from the internet, so the first run may
bring up a blue "Windows protected your PC" box. Right click the zip,
Properties, tick Unblock at the bottom, and then extract it.

## What it cannot do

**HEIC, raw, WebP and video are not read.** They are listed as not read,
not passed over. An iPhone saves HEIC unless told otherwise: Settings,
Camera, Formats, Most Compatible makes it save JPEG, and photos it shares
or exports are JPEG anyway.

**A phone with several cameras has several sensors.** The wide, the ultra
wide, the telephoto and the front camera each have their own pattern, and
a fingerprint of one says nothing about the others. Give the camera's
folder photos from the lens you mean to check, which for most photos is
the main one.

**On some phones the match is with the model, not the phone.** Recent
phones process every photo heavily, and some models leave patterns of
their own processing that every phone of that model shares. Iuliani and
others found this in most Samsung and Huawei models they tested, in 2021.
Birthmark has no other phone of your model to compare with, so it cannot
tell which it found; a match with your model is a leak either way, only a
smaller one. What the calibration found for phones of one model is above.

**Not finding the mark is not proof it is not there.** The search tries
the scales and crops a photo's own size suggests. A photo cropped and then
shrunk by a factor its size does not give away can be searched by trying
every scale, which is what a forensic lab does and this does not. And
somebody with more of your photos than you gave here has a cleaner
fingerprint and finds a fainter mark. The sizes in the report are where
the mark went for this fingerprint, not a promise.

**Digital zoom, portrait mode, night mode and panoramas** change the
geometry between the sensor and the photo in ways the search does not
follow, so the mark in them is often not found even when it is there.

**It does not take the mark out.** It only reads. It tells you what would
have taken it out, and the photo is yours to change.

## It only reads

Nothing is written, nothing is sent anywhere, no other program is run,
and there is no model in it. Tests read the source and fail the build if a
call that writes a file, an import of anything under `net`, an import of
`os/exec` or anything that looks like a model turns up. The copies it
shrinks to see what breaks the mark live in memory and nowhere else.

A gap is not a pass. A photo it cannot decode, and a format it does not
read, is printed first, before anything that was found.

It does not scold anybody. A screenshot has no sensor and comes out clear,
which is the right answer, and a photo that carries no mark is reported as
carrying no mark, with what that does and does not prove.

The method is the published one, every step of it in the source, and the
numbers above come from real phones, not from photos made up for the
tests. The calibration runs again every week on GitHub's machines, so a
change that makes it worse on real phones shows up the week it is made.

## Building

Needs [Go](https://go.dev) and nothing else. No dependencies at all.

```bash
go build -o birthmark .
```

```bash
go test ./...
```

The tests make up sensors, photograph scenes through them, save the photos
as JPEG and shrink, turn and crop them the way the apps do. The real phones
are downloaded and compared by `.github/workflows/vision.yml`; it takes an
hour or two of every processor of a machine and several gigabytes of
memory, so run it there and not on a computer anybody is using:

```bash
sh testdata/vision/fetch.sh ~/vision 30 20 D01 D02 D10
```

```bash
BIRTHMARK_VISION=~/vision BIRTHMARK_WORKERS=1 go test -run TestVision -v -timeout 6h .
```

## Licence

MIT. Do what you like with it.

VISION, which the calibration downloads and this repository does not
contain, is by Shullani, Fontani, Iuliani, Al Shaya and Piva, under CC
BY-SA 4.0: "VISION: a video and image dataset for source identification",
EURASIP Journal on Information Security, 2017.
