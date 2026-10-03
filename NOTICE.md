# Third-party notices

rabbitrun is released under the [MIT License](./LICENSE). The following parts are covered by their own licenses.

## Fonts

| File | Font | License |
| ---- | ---- | ------- |
| `assets/fonts/LilitaOne-Regular.ttf` | Lilita One by Juan Montoreano | [SIL Open Font License 1.1](./assets/fonts/OFL-LilitaOne.txt) |
| `assets/fonts/mplus-1p-regular.ttf` | M+ 1p Regular by M+ FONTS PROJECT | [M+ FONTS License](./assets/fonts/LICENSE-MPLUS.txt) (free to use, copy and distribute, with or without modification) |

## Music

The background music is original drum'n'bass arrangements synthesized at run time (`music.go`). The melodies and chords come from these scores of Antonio Vivaldi's *The Four Seasons* (Il cimento dell'armonia e dell'inventione, Op. 8), typeset in LilyPond by Anonymous for the [Mutopia Project](https://www.mutopiaproject.org/) (copyright 2010) from Performers' Facsimiles, and licensed under [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/):

| Song | Source | Mutopia reference | License |
| ---- | ------ | ----------------- | ------- |
| *La Primavera (Spring)* Op. 8 No. 1, 1st movement (Allegro) | [Mutopia Project piece 301](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=301) | Mutopia-2010/02/08-301 | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| *L'Estate (Summer)* Op. 8 No. 2, 1st movement (Allegro non molto) | [Mutopia Project piece 336](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=336) | Mutopia-2010/02/08-336 | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| *L'Autunno (Autumn)* Op. 8 No. 3, 1st movement (Allegro) | [Mutopia Project piece 350](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=350) | Mutopia-2010/02/08-350 | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| *L'Inverno (Winter)* Op. 8 No. 4, 1st movement (Allegro non molto) | [Mutopia Project piece 351](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=351) | Mutopia-2010/02/08-351 | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |

The note data for the *Four Seasons* in `songs_data.go` (`songSpring`, `songSummer`, `songAutumn`, `songWinter`) are adaptations of the first-movement MIDI files of those editions and are shared under the same CC BY-SA 3.0 license. They were changed as follows: the melody is the highest note of the solo violin part, one MIDI quarter note becomes two beats of the game's tempo, all five string parts are reduced to one major or minor triad per two beats (the root of which drives the bass line), each song is padded to a whole number of two-bar drum patterns so that it loops, and Spring's opening eighth-note pickup is moved to the end of the loop. The arrangement in `music.go` (drums, bass, piano, pad and effects) is new.

## Go modules

The license texts of the Go modules linked into rabbitrun ship with every release: under `THIRD_PARTY_LICENSES/` in the archives and under `/usr/share/doc/rabbitrun/THIRD_PARTY_LICENSES` in the Linux packages. CI checks those licenses with go-licenses (`.github/workflows/licenses.yml`).
