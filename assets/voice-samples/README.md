# Voice samples

The audio diagrams play with, one file per pitch a voice lists in the `voices`
table: `{voice_id}/{midi}.mp3`. Every deploy publishes this folder to the
environment's media bucket under `audio/voices/`, and `make db:reset` uploads it
to the local object store. A test fails when the files and the `voices` pitches
disagree.

`scripts/voice-samples.sh` renders missing files here (it needs ffmpeg and
Bash 4+). Commit them with the migration that adds the voice or its pitches.
Samples are cached as immutable at their URL, so a voice's sound never changes
in place: a re-rendered voice gets a new `voice_id`.

## Credit

Rendered from [tonejs-instruments](https://github.com/nbrosowsky/tonejs-instruments)
by Nicholaus Brosowsky, licensed under
[CC BY 3.0](https://creativecommons.org/licenses/by/3.0/). Each pitch is
re-pitched from the nearest recording, trimmed to 3 s with a 0.6 s fade, and
encoded as mono MP3 at 96 kbps.

- `acoustic-guitar/`: guitar-acoustic
- `electric-bass/`: bass-electric
- `piano/`: piano
